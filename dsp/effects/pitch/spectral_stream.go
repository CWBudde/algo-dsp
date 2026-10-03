package pitch

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/internal/streamframe"
)

// StreamingSpectralPitchShifter provides causal, bounded phase-vocoder pitch
// shifting with sample scheduling independent of caller block partitioning.
// It uses interpolated spectral-bin remapping at all supported pitch ratios.
// The associated processor must not be reconfigured during processing.
type StreamingSpectralPitchShifter struct {
	processor *SpectralPitchShifter
	stream    *streamframe.Engine
}

// NewStreamingSpectralPitchShifter reserves all render storage and rewinds the
// supplied processor. Configuration and FFT plans belong to that processor.
func NewStreamingSpectralPitchShifter(processor *SpectralPitchShifter) (*StreamingSpectralPitchShifter, error) {
	if processor == nil {
		return nil, fmt.Errorf("spectral pitch stream: nil processor")
	}

	if err := processor.validate(); err != nil {
		return nil, fmt.Errorf("spectral pitch stream: %w", err)
	}

	p := &StreamingSpectralPitchShifter{processor: processor}

	engine, err := streamframe.New(processor.windowCoeffs, processor.analysisHop, p.transform)
	if err != nil {
		return nil, fmt.Errorf("spectral pitch stream: %w", err)
	}

	p.stream = engine
	p.Reset()

	return p, nil
}

// Latency returns the analysis lookahead, or zero at the identity pitch ratio.
func (p *StreamingSpectralPitchShifter) Latency() int {
	if math.Abs(p.processor.pitchRatio-1) <= pitchShifterIdentityEps {
		return 0
	}

	return p.stream.Latency()
}

// Reset rewinds phase and overlap-add state without reallocating.
func (p *StreamingSpectralPitchShifter) Reset() { p.processor.Reset(); p.stream.Reset() }

// ProcessInPlace processes any block size without allocation. Feed zeros to
// drain the latency and spectral tail after the final source sample.
func (p *StreamingSpectralPitchShifter) ProcessInPlace(block []float64) error {
	if p.Latency() == 0 {
		return nil
	}

	return p.stream.ProcessInPlace(block, 1)
}

func (p *StreamingSpectralPitchShifter) transform(input, output []float64) error {
	s := p.processor
	half := s.frameSize / 2

	hop := float64(s.analysisHop)
	for i, x := range input {
		s.analysisSpectrum[i] = complex(x*s.windowCoeffs[i], 0)
	}

	if err := s.plan.Forward(s.analysisSpectrum, s.analysisSpectrum); err != nil {
		return fmt.Errorf("spectral pitch stream: forward: %w", err)
	}

	for k := 0; k <= half; k++ {
		re, im := real(s.analysisSpectrum[k]), imag(s.analysisSpectrum[k])
		s.magnitudes[k] = math.Hypot(re, im)
		phase := math.Atan2(im, re)
		delta := wrapPhase(phase - s.prevPhase[k] - s.omega[k]*hop)
		s.instFreqs[k] = s.omega[k] + delta/hop
		s.prevPhase[k] = phase
	}

	for k := 0; k <= half; k++ {
		source := float64(k) / s.pitchRatio
		if source >= float64(half) {
			s.shiftedMag[k] = 0
			s.shiftedFreq[k] = s.omega[k]
		} else {
			lo := int(source)
			hi := min(lo+1, half)
			fraction := source - float64(lo)
			s.shiftedMag[k] = s.magnitudes[lo]*(1-fraction) + s.magnitudes[hi]*fraction
			s.shiftedFreq[k] = (s.instFreqs[lo]*(1-fraction) + s.instFreqs[hi]*fraction) * s.pitchRatio
		}

		s.sumPhase[k] += s.shiftedFreq[k] * hop
		s.synthesisSpectrum[k] = complex(s.shiftedMag[k]*math.Cos(s.sumPhase[k]), s.shiftedMag[k]*math.Sin(s.sumPhase[k]))
	}

	s.synthesisSpectrum[0] = complex(real(s.synthesisSpectrum[0]), 0)

	s.synthesisSpectrum[half] = complex(real(s.synthesisSpectrum[half]), 0)
	for k := 1; k < half; k++ {
		v := s.synthesisSpectrum[k]
		s.synthesisSpectrum[s.frameSize-k] = complex(real(v), -imag(v))
	}

	if err := s.plan.Inverse(s.timeFrame, s.synthesisSpectrum); err != nil {
		return fmt.Errorf("spectral pitch stream: inverse: %w", err)
	}

	for i := range output {
		output[i] = real(s.timeFrame[i]) * s.windowCoeffs[i]
	}

	return nil
}
