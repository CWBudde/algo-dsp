package effects

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/resample"
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
// It uses a phase-vocoder STFT core to time-stretch in the frequency domain,
// then resamples back to the original duration. This keeps output length equal
// to input length while shifting pitch by the configured ratio.
//
// This processor is mono, one-shot buffer oriented, and not thread-safe.
type SpectralPitchShifter struct {
	sampleRate   float64
	pitchRatio   float64
	frameSize    int
	analysisHop  int
	synthesisHop int

	windowType      window.Type
	resampleQuality resample.Quality

	plan *algofft.Plan[complex128]

	windowCoeffs []float64
	omega        []float64
	prevPhase    []float64
	sumPhase     []float64

	analysisSpectrum  []complex128
	synthesisSpectrum []complex128
	timeFrame         []complex128
}

// NewSpectralPitchShifter creates a frequency-domain pitch shifter with defaults.
func NewSpectralPitchShifter(sampleRate float64) (*SpectralPitchShifter, error) {
	if !isFinitePositive(sampleRate) {
		return nil, fmt.Errorf("spectral pitch shifter sample rate must be positive and finite: %f", sampleRate)
	}
	s := &SpectralPitchShifter{
		sampleRate:      sampleRate,
		pitchRatio:      defaultSpectralPitchRatio,
		frameSize:       defaultSpectralFrameSize,
		analysisHop:     defaultSpectralAnalysisHop,
		windowType:      window.TypeHann,
		resampleQuality: resample.QualityBalanced,
	}
	s.updateSynthesisHop()
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
func (s *SpectralPitchShifter) EffectivePitchRatio() float64 {
	return float64(s.synthesisHop) / float64(s.analysisHop)
}

// FrameSize returns the FFT frame size.
func (s *SpectralPitchShifter) FrameSize() int { return s.frameSize }

// AnalysisHop returns the analysis hop size in samples.
func (s *SpectralPitchShifter) AnalysisHop() int { return s.analysisHop }

// SynthesisHop returns the synthesis hop size in samples.
func (s *SpectralPitchShifter) SynthesisHop() int { return s.synthesisHop }

// WindowType returns the STFT window type.
func (s *SpectralPitchShifter) WindowType() window.Type { return s.windowType }

// ResampleQuality returns the quality mode used during duration correction.
func (s *SpectralPitchShifter) ResampleQuality() resample.Quality {
	return s.resampleQuality
}

// SetSampleRate updates sample rate metadata.
//
// The spectral algorithm is ratio-based, so this value does not affect
// processing directly, but it keeps the API aligned with [PitchShifter].
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
	s.updateSynthesisHop()
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
	s.updateSynthesisHop()
	return s.rebuildState()
}

// SetAnalysisHop updates analysis hop size in samples.
func (s *SpectralPitchShifter) SetAnalysisHop(hop int) error {
	if hop <= 0 || hop >= s.frameSize {
		return fmt.Errorf("spectral analysis hop must be in [1, %d): %d", s.frameSize, hop)
	}
	s.analysisHop = hop
	s.updateSynthesisHop()
	return nil
}

// SetWindowType updates the STFT window shape.
func (s *SpectralPitchShifter) SetWindowType(t window.Type) error {
	s.windowType = t
	return s.rebuildState()
}

// SetResampleQuality updates duration-correction resampling quality mode.
func (s *SpectralPitchShifter) SetResampleQuality(q resample.Quality) {
	s.resampleQuality = q
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
// Use this in strict pipelines where internal failures should be surfaced.
func (s *SpectralPitchShifter) ProcessWithError(input []float64) ([]float64, error) {
	if len(input) == 0 {
		return nil, nil
	}
	if err := s.validate(); err != nil {
		return nil, err
	}

	s.Reset()

	frameCount := 1 + (len(input)-1)/s.analysisHop
	stretchedLen := (frameCount-1)*s.synthesisHop + s.frameSize
	stretched := make([]float64, stretchedLen)
	norm := make([]float64, stretchedLen)

	half := s.frameSize / 2
	analysisHopF := float64(s.analysisHop)
	synthesisHopF := float64(s.synthesisHop)

	for frame := 0; frame < frameCount; frame++ {
		inPos := frame * s.analysisHop
		outPos := frame * s.synthesisHop

		for i := 0; i < s.frameSize; i++ {
			x := 0.0
			idx := inPos + i
			if idx < len(input) {
				x = input[idx]
			}
			s.analysisSpectrum[i] = complex(x*s.windowCoeffs[i], 0)
		}

		if err := s.plan.Forward(s.analysisSpectrum, s.analysisSpectrum); err != nil {
			return nil, fmt.Errorf("spectral pitch shifter: forward FFT failed: %w", err)
		}

		for k := 0; k <= half; k++ {
			re := real(s.analysisSpectrum[k])
			im := imag(s.analysisSpectrum[k])
			mag := math.Hypot(re, im)
			phase := math.Atan2(im, re)

			delta := phase - s.prevPhase[k] - s.omega[k]*analysisHopF
			delta = wrapPhase(delta)

			instFreq := s.omega[k] + delta/analysisHopF
			s.sumPhase[k] += instFreq * synthesisHopF
			s.prevPhase[k] = phase

			s.synthesisSpectrum[k] = complex(
				mag*math.Cos(s.sumPhase[k]),
				mag*math.Sin(s.sumPhase[k]),
			)
		}

		s.synthesisSpectrum[0] = complex(real(s.synthesisSpectrum[0]), 0)
		s.synthesisSpectrum[half] = complex(real(s.synthesisSpectrum[half]), 0)
		for k := 1; k < half; k++ {
			v := s.synthesisSpectrum[k]
			s.synthesisSpectrum[s.frameSize-k] = complex(real(v), -imag(v))
		}

		if err := s.plan.Inverse(s.timeFrame, s.synthesisSpectrum); err != nil {
			return nil, fmt.Errorf("spectral pitch shifter: inverse FFT failed: %w", err)
		}

		for i := 0; i < s.frameSize; i++ {
			idx := outPos + i
			w := s.windowCoeffs[i]
			stretched[idx] += real(s.timeFrame[i]) * w
			norm[idx] += w * w
		}
	}

	for i := range stretched {
		if norm[i] > spectralNormFloor {
			stretched[i] /= norm[i]
		}
	}

	if s.synthesisHop == s.analysisHop {
		return fitLength(stretched, len(input)), nil
	}

	shifted, err := resample.Resample(
		stretched,
		s.analysisHop,
		s.synthesisHop,
		resample.WithQuality(s.resampleQuality),
	)
	if err != nil {
		return nil, fmt.Errorf("spectral pitch shifter: resampling failed: %w", err)
	}
	return fitLength(shifted, len(input)), nil
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
	if s.synthesisHop <= 0 {
		return fmt.Errorf("spectral synthesis hop must be > 0: %d", s.synthesisHop)
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
	for k := 0; k < bins; k++ {
		s.omega[k] = 2 * math.Pi * float64(k) / float64(s.frameSize)
	}

	s.prevPhase = make([]float64, bins)
	s.sumPhase = make([]float64, bins)
	s.analysisSpectrum = make([]complex128, s.frameSize)
	s.synthesisSpectrum = make([]complex128, s.frameSize)
	s.timeFrame = make([]complex128, s.frameSize)

	return nil
}

func (s *SpectralPitchShifter) updateSynthesisHop() {
	h := int(math.Round(float64(s.analysisHop) * s.pitchRatio))
	if h < 1 {
		h = 1
	}
	s.synthesisHop = h
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
