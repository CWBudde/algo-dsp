package restoration

import (
	"context"
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/stft"
)

// Reader copies exactly the requested valid sample interval. Buffers are borrowed
// for the duration of the call; the processor handles zero padding itself.
type (
	Reader func(dst []float64, start int64) int
	// SpectralConfig selects one bounded spectral operation. Modes are "attenuate",
	// "remove", "heal" or "noise". GainDB is [-120,0]. Heal needs a <=256-sample
	// damaged interval with intact context; the repaired spectrum is blended only
	// into selected bins. Noise requires a captured profile in the matching format.
	SpectralConfig struct {
		Mode                            string
		FFTSize                         int
		SampleRate, GainDB, ReductionDB float64
		Mask                            Mask
		Profile                         *NoiseProfile
		Method                          string
	}
	// SpectralProcessor reads one FFT frame per Step, modifies bins and emits
	// contiguous samples through a normalized inverse STFT. It retains O(FFTSize)
	// scratch and at most 768 samples of interpolation context, never a full file.
	SpectralProcessor struct {
		config           SpectralConfig
		length, frame    int64
		reader           Reader
		analysis         *stft.STFT
		synthesis        *stft.InverseStream
		raw, replacement []float64
		bins, healedBins []complex128
		reducer          *NoiseReducer
		healed           []float64
		healedStart      int64
		closed           bool
		failure          error
	}
)

// NewSpectralProcessor validates all controls before constructing transforms.
func NewSpectralProcessor(length int64, reader Reader, c SpectralConfig) (*SpectralProcessor, error) {
	if length < 1 || length > 1<<52 || reader == nil || !finite(c.SampleRate) || c.SampleRate < 8000 || c.SampleRate > 384000 || c.FFTSize < 256 || c.FFTSize > 8192 || c.FFTSize&(c.FFTSize-1) != 0 {
		return nil, fmt.Errorf("restoration.spectral: invalid source or format")
	}

	if c.Mode != "noise" {
		if err := c.Mask.Validate(length, c.SampleRate); err != nil {
			return nil, err
		}
	}

	if (c.Mode != "attenuate" && c.Mode != "remove" && c.Mode != "heal" && c.Mode != "noise") || !finite(c.GainDB) || c.GainDB > 0 || c.GainDB < -120 {
		return nil, fmt.Errorf("restoration.spectral: invalid mode or gain")
	}

	if c.Mode == "heal" && (c.Mask.End-c.Mask.Start > 256 || c.Mask.Start < 2 || c.Mask.End > length-2) {
		return nil, fmt.Errorf("restoration.spectral: heal requires a <=256-sample gap with two-sided context")
	}

	p := &SpectralProcessor{config: c, length: length, reader: reader, raw: make([]float64, c.FFTSize), replacement: make([]float64, c.FFTSize), bins: make([]complex128, c.FFTSize/2+1), healedBins: make([]complex128, c.FFTSize/2+1)}
	p.config.Mask.Points = append([]Point(nil), c.Mask.Points...)

	var err error

	p.analysis, err = stft.New(c.FFTSize, c.FFTSize/4, stft.WithCenter(stft.PadNone))
	if err != nil {
		return nil, err
	}

	inverse, err := stft.New(c.FFTSize, c.FFTSize/4)
	if err != nil {
		return nil, err
	}

	p.synthesis, err = inverse.InverseStream(length)
	if err != nil {
		return nil, err
	}

	if c.Mode == "noise" {
		if c.Profile == nil || c.Profile.FFTSize != c.FFTSize || c.Profile.SampleRate != c.SampleRate {
			return nil, fmt.Errorf("restoration.spectral: mismatched profile")
		}

		p.reducer, err = NewNoiseReducer(c.Profile, c.ReductionDB, c.Method)
		if err != nil {
			return nil, err
		}
	}

	return p, nil
}

// Step processes at most one FFT frame or one final tail. Callback output is
// borrowed. A cancellation or callback error is terminal; the caller discards
// partial output. No callback is invoked after completion.
func (p *SpectralProcessor) Step(ctx context.Context, emit func(int64, []float64) error) (bool, error) {
	if p == nil || ctx == nil || emit == nil {
		return false, fmt.Errorf("restoration.spectral.step: invalid call")
	}

	if p.failure != nil {
		return false, p.failure
	}

	if p.closed {
		return true, nil
	}

	if err := ctx.Err(); err != nil {
		p.failure = err
		return false, err
	}

	hop := int64(p.config.FFTSize / 4)

	count := (p.length + hop - 1) / hop
	if p.frame >= count {
		done, err := p.synthesis.FinishStep(p.config.FFTSize, emit)
		p.closed = done
		p.failure = err

		return done, err
	}

	start := p.frame*hop - int64(p.config.FFTSize/2)
	if err := p.read(p.raw, start); err != nil {
		p.failure = err
		return false, err
	}

	if err := p.analysis.FrameInto(p.bins, p.raw, 0); err != nil {
		p.failure = err
		return false, err
	}

	if p.config.Mode == "noise" {
		if err := p.reducer.ProcessSpectrum(p.bins); err != nil {
			p.failure = err
			return false, err
		}
	} else {
		if p.config.Mode == "heal" {
			if p.healed == nil {
				p.healedStart = max(0, p.config.Mask.Start-256)
				end := min(p.length, p.config.Mask.End+256)

				p.healed = make([]float64, end-p.healedStart)
				if err := p.read(p.healed, p.healedStart); err != nil {
					p.failure = err
					return false, err
				}

				if err := RepairGap(p.healed, int(p.config.Mask.Start-p.healedStart), int(p.config.Mask.End-p.healedStart)); err != nil {
					p.failure = err
					return false, err
				}
			}

			copy(p.replacement, p.raw)

			for i := range p.replacement {
				pos := start + int64(i)
				if pos >= p.config.Mask.Start && pos < p.config.Mask.End {
					p.replacement[i] = p.healed[pos-p.healedStart]
				}
			}

			if err := p.analysis.FrameInto(p.healedBins, p.replacement, 0); err != nil {
				p.failure = err
				return false, err
			}
		}

		gain := math.Pow(10, p.config.GainDB/20)
		if p.config.Mode == "remove" {
			gain = 0
		}
		// Include every analysis window that overlaps the selected interval; this
		// avoids leaving an impulse in neighbouring windows. Bin masks are evaluated
		// at the nearest selected time when a window centre is outside the interval.
		centre := math.Max(float64(p.config.Mask.Start), math.Min(float64(p.config.Mask.End)-0.5, float64(p.frame*hop)))
		if start < int64(p.config.Mask.End) && start+int64(p.config.FFTSize) > p.config.Mask.Start {
			for k := range p.bins {
				hz := float64(k) * p.config.SampleRate / float64(p.config.FFTSize)
				if p.config.Mask.Contains(centre, hz) {
					if p.config.Mode == "heal" {
						p.bins[k] = p.healedBins[k]
					} else {
						p.bins[k] *= complex(gain, 0)
					}
				}
			}
		}
	}

	if err := ctx.Err(); err != nil {
		p.failure = err
		return false, err
	}

	err := p.synthesis.ProcessFrame(p.bins, emit)

	p.failure = err
	if err != nil {
		return false, err
	}

	p.frame++

	return false, nil
}

func (p *SpectralProcessor) read(dst []float64, start int64) error {
	clear(dst)

	lo := max(int64(0), -start)

	hi := min(int64(len(dst)), p.length-start)
	if hi > lo && p.reader(dst[lo:hi], start+lo) != int(hi-lo) {
		return fmt.Errorf("restoration.spectral: short source read")
	}

	for _, x := range dst {
		if !finite(x) {
			return fmt.Errorf("restoration.spectral: nonfinite source")
		}
	}

	return nil
}
