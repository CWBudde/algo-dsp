package effects

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/window"
	algofft "github.com/cwbudde/algo-fft"
)

const (
	defaultSpectralPitchRatio  = 1.0
	defaultSpectralSampleRate  = 44100.0
	defaultSpectralFrameSize   = 1024
	defaultSpectralAnalysisHop = 256
	minSpectralFrameSize       = 64
	spectralNormFloor          = 1e-12
)

// SpectralPitchShifter performs frequency-domain pitch shifting.
//
// It uses a phase-vocoder STFT core with direct spectral bin shifting
// (Laroche & Dolson 1999). Analysis and synthesis use the same hop size;
// pitch shifting is achieved by remapping spectral bins by the pitch ratio
// with linear interpolation, so no time-domain resampling is needed.
//
// This processor is mono, one-shot buffer oriented, and not thread-safe.
type SpectralPitchShifter struct {
	sampleRate  float64
	pitchRatio  float64
	frameSize   int
	analysisHop int

	windowType window.Type

	plan *algofft.Plan[complex128]

	windowCoeffs []float64
	omega        []float64
	prevPhase    []float64
	sumPhase     []float64

	analysisSpectrum  []complex128
	synthesisSpectrum []complex128
	timeFrame         []complex128

	// Work buffers (allocated once in rebuildState).
	magnitudes  []float64
	instFreqs   []float64
	shiftedMag  []float64
	shiftedFreq []float64
	peakBins    []int
}

// NewSpectralPitchShifter creates a frequency-domain pitch shifter with defaults.
func NewSpectralPitchShifter(sampleRate float64) (*SpectralPitchShifter, error) {
	if !isFinitePositive(sampleRate) {
		return nil, fmt.Errorf("spectral pitch shifter sample rate must be positive and finite: %f", sampleRate)
	}
	s := &SpectralPitchShifter{
		sampleRate:  sampleRate,
		pitchRatio:  defaultSpectralPitchRatio,
		frameSize:   defaultSpectralFrameSize,
		analysisHop: defaultSpectralAnalysisHop,
		windowType:  window.TypeHann,
	}
	if err := s.rebuildState(); err != nil {
		return nil, err
	}
	return s, nil
}

// NewSpectralPitchShifterDefault creates a spectral shifter at 44.1 kHz.
//
// Deprecated: prefer NewSpectralPitchShifter with an explicit sample rate.
func NewSpectralPitchShifterDefault() (*SpectralPitchShifter, error) {
	return NewSpectralPitchShifter(defaultSpectralSampleRate)
}

// SampleRate returns the current sample rate in Hz.
func (s *SpectralPitchShifter) SampleRate() float64 { return s.sampleRate }

// PitchRatio returns the requested pitch-shift ratio.
func (s *SpectralPitchShifter) PitchRatio() float64 { return s.pitchRatio }

// PitchSemitones returns the requested pitch shift in semitones.
func (s *SpectralPitchShifter) PitchSemitones() float64 { return 12.0 * math.Log2(s.pitchRatio) }

// EffectivePitchRatio returns the internally realized pitch ratio.
// With the bin-shifting approach, this equals the requested ratio exactly.
func (s *SpectralPitchShifter) EffectivePitchRatio() float64 {
	return s.pitchRatio
}

// FrameSize returns the FFT frame size.
func (s *SpectralPitchShifter) FrameSize() int { return s.frameSize }

// AnalysisHop returns the analysis hop size in samples.
func (s *SpectralPitchShifter) AnalysisHop() int { return s.analysisHop }

// SynthesisHop returns the synthesis hop size in samples.
// With the bin-shifting approach, this always equals the analysis hop.
func (s *SpectralPitchShifter) SynthesisHop() int { return s.analysisHop }

// WindowType returns the STFT window type.
func (s *SpectralPitchShifter) WindowType() window.Type { return s.windowType }

// SetSampleRate updates sample rate metadata.
func (s *SpectralPitchShifter) SetSampleRate(sampleRate float64) error {
	if !isFinitePositive(sampleRate) {
		return fmt.Errorf("spectral pitch shifter sample rate must be positive and finite: %f", sampleRate)
	}
	s.sampleRate = sampleRate
	return nil
}

// SetPitchRatio updates pitch-shift ratio. ratio must be positive and finite.
func (s *SpectralPitchShifter) SetPitchRatio(ratio float64) error {
	if !isFinitePositive(ratio) || ratio < minPitchShifterRatio || ratio > maxPitchShifterRatio {
		return fmt.Errorf("spectral pitch ratio must be in [%f, %f]: %f",
			minPitchShifterRatio, maxPitchShifterRatio, ratio)
	}
	s.pitchRatio = ratio
	return nil
}

// SetPitchSemitones updates pitch shift in semitones.
func (s *SpectralPitchShifter) SetPitchSemitones(semitones float64) error {
	if math.IsNaN(semitones) || math.IsInf(semitones, 0) {
		return fmt.Errorf("spectral pitch shifter semitones must be finite: %f", semitones)
	}
	ratio := math.Pow(2, semitones/12.0)
	if err := s.SetPitchRatio(ratio); err != nil {
		return fmt.Errorf("spectral pitch shifter semitones out of range: %w", err)
	}
	return nil
}

// SetFrameSize updates FFT frame size. size must be a power of two and >= 64.
func (s *SpectralPitchShifter) SetFrameSize(size int) error {
	if size < minSpectralFrameSize || !isPowerOf2Pitch(size) {
		return fmt.Errorf("spectral frame size must be power-of-two and >= %d: %d", minSpectralFrameSize, size)
	}
	s.frameSize = size
	if s.analysisHop >= s.frameSize {
		s.analysisHop = s.frameSize / 4
		if s.analysisHop <= 0 {
			s.analysisHop = 1
		}
	}
	return s.rebuildState()
}

// SetAnalysisHop updates analysis hop size in samples.
func (s *SpectralPitchShifter) SetAnalysisHop(hop int) error {
	if hop <= 0 || hop >= s.frameSize {
		return fmt.Errorf("spectral analysis hop must be in [1, %d): %d", s.frameSize, hop)
	}
	s.analysisHop = hop
	return nil
}

// SetWindowType updates the STFT window shape.
func (s *SpectralPitchShifter) SetWindowType(t window.Type) error {
	s.windowType = t
	return s.rebuildState()
}

// Reset clears phase tracking state.
func (s *SpectralPitchShifter) Reset() {
	for i := range s.prevPhase {
		s.prevPhase[i] = 0
		s.sumPhase[i] = 0
	}
}

// Process applies frequency-domain pitch shifting to input.
//
// The returned slice always has the same length as input.
// If processing fails due to invalid internal state, this returns a copy of input.
func (s *SpectralPitchShifter) Process(input []float64) []float64 {
	if len(input) == 0 {
		return nil
	}
	if math.Abs(s.pitchRatio-1) <= pitchShifterIdentityEps {
		out := make([]float64, len(input))
		copy(out, input)
		return out
	}

	out, err := s.ProcessWithError(input)
	if err != nil {
		fallback := make([]float64, len(input))
		copy(fallback, input)
		return fallback
	}
	return out
}

// ProcessWithError applies frequency-domain pitch shifting and reports errors.
//
// The algorithm uses direct spectral bin shifting: after computing the analysis
// magnitudes and instantaneous frequencies, it remaps them to shifted bin
// positions using linear interpolation. Identity phase locking (Laroche &
// Dolson 1999) is applied to bins near spectral peaks in the shifted spectrum;
// bins with negligible magnitude use standard per-bin phase advance to avoid
// phase noise. Since analysis and synthesis use the same hop size, no
// time-domain resampling is needed.
func (s *SpectralPitchShifter) ProcessWithError(input []float64) ([]float64, error) {
	if len(input) == 0 {
		return nil, nil
	}
	if err := s.validate(); err != nil {
		return nil, err
	}

	s.Reset()

	hop := s.analysisHop
	frameCount := 1 + (len(input)-1)/hop
	outLen := (frameCount-1)*hop + s.frameSize
	output := make([]float64, outLen)
	norm := make([]float64, outLen)

	half := s.frameSize / 2
	hopF := float64(hop)
	ratio := s.pitchRatio

	for frame := 0; frame < frameCount; frame++ {
		pos := frame * hop

		// Window the analysis frame.
		for i := range s.frameSize {
			x := 0.0
			idx := pos + i
			if idx < len(input) {
				x = input[idx]
			}
			s.analysisSpectrum[i] = complex(x*s.windowCoeffs[i], 0)
		}

		if err := s.plan.Forward(s.analysisSpectrum, s.analysisSpectrum); err != nil {
			return nil, fmt.Errorf("spectral pitch shifter: forward FFT failed: %w", err)
		}

		// Pass 1: compute magnitudes and instantaneous frequencies.
		for k := 0; k <= half; k++ {
			re := real(s.analysisSpectrum[k])
			im := imag(s.analysisSpectrum[k])
			s.magnitudes[k] = math.Hypot(re, im)
			phase := math.Atan2(im, re)

			delta := phase - s.prevPhase[k] - s.omega[k]*hopF
			delta = wrapPhase(delta)

			s.instFreqs[k] = s.omega[k] + delta/hopF
			s.prevPhase[k] = phase
		}

		// Pass 2: spectral bin shifting with linear interpolation.
		// For each synthesis bin k, read from analysis bin k/ratio.
		for k := 0; k <= half; k++ {
			srcK := float64(k) / ratio
			if srcK >= float64(half) {
				s.shiftedMag[k] = 0
				s.shiftedFreq[k] = s.omega[k]
				continue
			}
			lo := int(srcK)
			frac := srcK - float64(lo)
			hi := min(lo+1, half)
			s.shiftedMag[k] = s.magnitudes[lo]*(1-frac) + s.magnitudes[hi]*frac
			// Scale the instantaneous frequency by the ratio: the component
			// that was at frequency instFreqs[lo] is now placed ratio× higher.
			interpFreq := s.instFreqs[lo]*(1-frac) + s.instFreqs[hi]*frac
			s.shiftedFreq[k] = interpFreq * ratio
		}

		// Pass 3: phase accumulation with selective identity phase locking.
		// Find peaks in the shifted magnitudes and determine a threshold
		// below which bins use standard per-bin phase advance.
		s.peakBins = s.peakBins[:0]
		for k := 1; k < half; k++ {
			if s.shiftedMag[k] >= s.shiftedMag[k-1] && s.shiftedMag[k] > s.shiftedMag[k+1] {
				s.peakBins = append(s.peakBins, k)
			}
		}

		if len(s.peakBins) == 0 {
			// No peaks — standard per-bin phase advance.
			for k := 0; k <= half; k++ {
				s.sumPhase[k] += s.shiftedFreq[k] * hopF
				s.synthesisSpectrum[k] = complex(
					s.shiftedMag[k]*math.Cos(s.sumPhase[k]),
					s.shiftedMag[k]*math.Sin(s.sumPhase[k]),
				)
			}
		} else {
			// Advance peak phases using their shifted instantaneous frequencies.
			for _, pk := range s.peakBins {
				s.sumPhase[pk] += s.shiftedFreq[pk] * hopF
			}

			// Lock non-peak bins to their nearest peak. Use interpolated
			// analysis phases for bins with significant energy; use per-bin
			// phase advance for bins with negligible energy to avoid
			// accumulating noise from meaningless phase differences.
			peakIdx := 0
			for k := 0; k <= half; k++ {
				for peakIdx+1 < len(s.peakBins) {
					curr := s.peakBins[peakIdx]
					next := s.peakBins[peakIdx+1]
					if absInt(next-k) < absInt(curr-k) {
						peakIdx++
					} else {
						break
					}
				}

				pk := s.peakBins[peakIdx]
				if k != pk {
					// Only phase-lock bins within the main lobe of
					// the peak (within a few bins). Beyond that,
					// the analysis phases at the source positions
					// become unreliable.
					dist := absInt(k - pk)
					if dist <= 4 && s.shiftedMag[k] > 0 {
						srcK := float64(k) / ratio
						srcPk := float64(pk) / ratio
						phaseK := interpolatePhase(s.prevPhase, srcK, half)
						phasePk := interpolatePhase(s.prevPhase, srcPk, half)
						s.sumPhase[k] = s.sumPhase[pk] + (phaseK - phasePk)
					} else {
						s.sumPhase[k] += s.shiftedFreq[k] * hopF
					}
				}

				s.synthesisSpectrum[k] = complex(
					s.shiftedMag[k]*math.Cos(s.sumPhase[k]),
					s.shiftedMag[k]*math.Sin(s.sumPhase[k]),
				)
			}
		}

		// Mirror for real-valued IFFT.
		s.synthesisSpectrum[0] = complex(real(s.synthesisSpectrum[0]), 0)
		s.synthesisSpectrum[half] = complex(real(s.synthesisSpectrum[half]), 0)
		for k := 1; k < half; k++ {
			v := s.synthesisSpectrum[k]
			s.synthesisSpectrum[s.frameSize-k] = complex(real(v), -imag(v))
		}

		if err := s.plan.Inverse(s.timeFrame, s.synthesisSpectrum); err != nil {
			return nil, fmt.Errorf("spectral pitch shifter: inverse FFT failed: %w", err)
		}

		// Overlap-add with window normalization.
		for i := range s.frameSize {
			idx := pos + i
			w := s.windowCoeffs[i]
			output[idx] += real(s.timeFrame[i]) * w
			norm[idx] += w * w
		}
	}

	for i := range output {
		if norm[i] > spectralNormFloor {
			output[i] /= norm[i]
		}
	}

	return fitLength(output, len(input)), nil
}

// ProcessInPlace applies frequency-domain pitch shifting to buf in place.
func (s *SpectralPitchShifter) ProcessInPlace(buf []float64) {
	out := s.Process(buf)
	copy(buf, out)
}

// ProcessInPlaceWithError applies pitch shifting in place and returns errors.
func (s *SpectralPitchShifter) ProcessInPlaceWithError(buf []float64) error {
	out, err := s.ProcessWithError(buf)
	if err != nil {
		return err
	}
	copy(buf, out)
	return nil
}

func (s *SpectralPitchShifter) validate() error {
	if !isFinitePositive(s.sampleRate) {
		return fmt.Errorf("spectral pitch shifter sample rate must be positive and finite: %f", s.sampleRate)
	}
	if !isFinitePositive(s.pitchRatio) || s.pitchRatio < minPitchShifterRatio || s.pitchRatio > maxPitchShifterRatio {
		return fmt.Errorf("spectral pitch ratio must be in [%f, %f]: %f",
			minPitchShifterRatio, maxPitchShifterRatio, s.pitchRatio)
	}
	if s.frameSize < minSpectralFrameSize || !isPowerOf2Pitch(s.frameSize) {
		return fmt.Errorf("spectral frame size must be power-of-two and >= %d: %d", minSpectralFrameSize, s.frameSize)
	}
	if s.analysisHop <= 0 || s.analysisHop >= s.frameSize {
		return fmt.Errorf("spectral analysis hop must be in [1, %d): %d", s.frameSize, s.analysisHop)
	}
	return nil
}

func (s *SpectralPitchShifter) rebuildState() error {
	if err := s.validate(); err != nil {
		return err
	}

	plan, err := algofft.NewPlan64(s.frameSize)
	if err != nil {
		return fmt.Errorf("spectral pitch shifter: failed to create FFT plan: %w", err)
	}
	s.plan = plan

	coeffs := window.Generate(s.windowType, s.frameSize, window.WithPeriodic())
	if len(coeffs) != s.frameSize {
		return fmt.Errorf("spectral pitch shifter: window generation failed for size %d", s.frameSize)
	}
	s.windowCoeffs = coeffs

	bins := s.frameSize/2 + 1
	s.omega = make([]float64, bins)
	for k := range bins {
		s.omega[k] = 2 * math.Pi * float64(k) / float64(s.frameSize)
	}

	s.prevPhase = make([]float64, bins)
	s.sumPhase = make([]float64, bins)
	s.analysisSpectrum = make([]complex128, s.frameSize)
	s.synthesisSpectrum = make([]complex128, s.frameSize)
	s.timeFrame = make([]complex128, s.frameSize)

	s.magnitudes = make([]float64, bins)
	s.instFreqs = make([]float64, bins)
	s.shiftedMag = make([]float64, bins)
	s.shiftedFreq = make([]float64, bins)
	s.peakBins = make([]int, 0, bins)

	return nil
}

// interpolatePhase linearly interpolates the phase array at fractional index srcK.
// Phase wrapping is handled by interpolating in the complex domain.
func interpolatePhase(phases []float64, srcK float64, half int) float64 {
	if srcK <= 0 {
		return phases[0]
	}
	if srcK >= float64(half) {
		return phases[half]
	}
	lo := int(srcK)
	frac := srcK - float64(lo)
	hi := min(lo+1, half)
	// Interpolate via unit complex numbers to handle wrapping correctly.
	re := math.Cos(phases[lo])*(1-frac) + math.Cos(phases[hi])*frac
	im := math.Sin(phases[lo])*(1-frac) + math.Sin(phases[hi])*frac
	return math.Atan2(im, re)
}

func fitLength(in []float64, n int) []float64 {
	out := make([]float64, n)
	copy(out, in)
	return out
}

func wrapPhase(x float64) float64 {
	x = math.Mod(x+math.Pi, 2*math.Pi)
	if x < 0 {
		x += 2 * math.Pi
	}
	return x - math.Pi
}

func isPowerOf2Pitch(v int) bool {
	return v > 0 && (v&(v-1)) == 0
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
