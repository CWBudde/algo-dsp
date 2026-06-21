//nolint:funlen,gocognit,cyclop
package effects

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/window"
	algofft "github.com/cwbudde/algo-fft"
)

const (
	defaultSpectralFreezeFrameSize = 1024
	defaultSpectralFreezeHopSize   = 256
	minSpectralFreezeFrameSize     = 64
	spectralFreezeNormFloor        = 1e-12
)

// SpectralFreezePhaseMode controls how frozen-bin phases evolve over time.
type SpectralFreezePhaseMode int

const (
	// SpectralFreezePhaseHold keeps each frozen bin at captured phase.
	SpectralFreezePhaseHold SpectralFreezePhaseMode = iota
	// SpectralFreezePhaseAdvance advances each bin by its expected center frequency.
	SpectralFreezePhaseAdvance
)

// SpectralFreeze captures one STFT magnitude frame and sustains it.
//
// The processor uses overlap-add STFT analysis/synthesis with configurable
// frame size, hop size, window type, and wet/dry mix.
//
// This processor is mono, buffer-oriented, and not thread-safe.
type SpectralFreeze struct {
	sampleRate float64
	frameSize  int
	hopSize    int
	mix        float64
	windowType window.Type
	phaseMode  SpectralFreezePhaseMode
	frozen     bool

	plan *algofft.Plan[complex128]

	windowCoeffs []float64
	omega        []float64

	analysisSpectrum  []complex128
	synthesisSpectrum []complex128
	timeFrame         []complex128

	heldMagnitude  []float64
	phaseAcc       []float64
	hasFrozenFrame bool
}

// NewSpectralFreeze creates a spectral freeze processor with practical defaults.
func NewSpectralFreeze(sampleRate float64) (*SpectralFreeze, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("spectral freeze sample rate must be > 0: %f", sampleRate)
	}

	s := &SpectralFreeze{
		sampleRate: sampleRate,
		frameSize:  defaultSpectralFreezeFrameSize,
		hopSize:    defaultSpectralFreezeHopSize,
		mix:        1,
		windowType: window.TypeHann,
		phaseMode:  SpectralFreezePhaseAdvance,
	}

	err := s.rebuildState()
	if err != nil {
		return nil, err
	}

	return s, nil
}

// SampleRate returns sample rate in Hz.
func (s *SpectralFreeze) SampleRate() float64 { return s.sampleRate }

// FrameSize returns FFT frame size.
func (s *SpectralFreeze) FrameSize() int { return s.frameSize }

// HopSize returns STFT hop size in samples.
func (s *SpectralFreeze) HopSize() int { return s.hopSize }

// Mix returns wet/dry mix in [0, 1].
func (s *SpectralFreeze) Mix() float64 { return s.mix }

// WindowType returns the STFT window type.
func (s *SpectralFreeze) WindowType() window.Type { return s.windowType }

// PhaseMode returns the frozen phase evolution mode.
func (s *SpectralFreeze) PhaseMode() SpectralFreezePhaseMode { return s.phaseMode }

// Frozen returns whether spectral freezing is active.
func (s *SpectralFreeze) Frozen() bool { return s.frozen }

// SetSampleRate updates sample-rate metadata.
func (s *SpectralFreeze) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("spectral freeze sample rate must be > 0: %f", sampleRate)
	}

	s.sampleRate = sampleRate

	return nil
}

// SetFrameSize updates FFT frame size. size must be a power of two and >= 64.
func (s *SpectralFreeze) SetFrameSize(size int) error {
	if size < minSpectralFreezeFrameSize || !isPowerOf2SpectralFreeze(size) {
		return fmt.Errorf("spectral freeze frame size must be power-of-two and >= %d: %d",
			minSpectralFreezeFrameSize, size)
	}

	s.frameSize = size
	if s.hopSize >= s.frameSize {
		s.hopSize = max(s.frameSize/4, 1)
	}

	return s.rebuildState()
}

// SetHopSize updates STFT hop size. hop must be in [1, frameSize).
func (s *SpectralFreeze) SetHopSize(hop int) error {
	if hop <= 0 || hop >= s.frameSize {
		return fmt.Errorf("spectral freeze hop size must be in [1, %d): %d", s.frameSize, hop)
	}

	s.hopSize = hop

	return nil
}

// SetMix updates wet/dry mix in [0, 1].
func (s *SpectralFreeze) SetMix(mix float64) error {
	if mix < 0 || mix > 1 || math.IsNaN(mix) || math.IsInf(mix, 0) {
		return fmt.Errorf("spectral freeze mix must be in [0, 1]: %f", mix)
	}

	s.mix = mix

	return nil
}

// SetWindowType updates STFT window type and rebuilds analysis/synthesis state.
func (s *SpectralFreeze) SetWindowType(t window.Type) error {
	s.windowType = t
	return s.rebuildState()
}

// SetPhaseMode updates frozen phase behavior.
func (s *SpectralFreeze) SetPhaseMode(mode SpectralFreezePhaseMode) error {
	switch mode {
	case SpectralFreezePhaseHold, SpectralFreezePhaseAdvance:
		s.phaseMode = mode
		return nil
	default:
		return fmt.Errorf("spectral freeze phase mode invalid: %d", mode)
	}
}

// SetFrozen enables or disables spectral freezing.
func (s *SpectralFreeze) SetFrozen(frozen bool) {
	if frozen != s.frozen {
		s.hasFrozenFrame = false
	}

	s.frozen = frozen
}

// Freeze enables spectral freezing.
func (s *SpectralFreeze) Freeze() { s.SetFrozen(true) }

// Unfreeze disables spectral freezing.
func (s *SpectralFreeze) Unfreeze() { s.SetFrozen(false) }

// Reset clears captured freeze state and phase accumulators.
func (s *SpectralFreeze) Reset() {
	s.hasFrozenFrame = false
	for i := range s.phaseAcc {
		s.phaseAcc[i] = 0
	}
}

// Process applies spectral freeze processing and returns a new output slice.
// If processing fails, this returns a copy of input.
func (s *SpectralFreeze) Process(input []float64) []float64 {
	if len(input) == 0 {
		return nil
	}

	out, err := s.ProcessWithError(input)
	if err != nil {
		fallback := make([]float64, len(input))
		copy(fallback, input)

		return fallback
	}

	return out
}

// ProcessWithError applies spectral freeze processing and returns errors.
func (s *SpectralFreeze) ProcessWithError(input []float64) ([]float64, error) {
	if len(input) == 0 {
		return nil, nil
	}

	err := s.validate()
	if err != nil {
		return nil, err
	}

	hop := s.hopSize
	frameCount := 1 + (len(input)-1)/hop
	outLen := (frameCount-1)*hop + s.frameSize
	wet := make([]float64, outLen)
	norm := make([]float64, outLen)

	half := s.frameSize / 2
	hopF := float64(hop)

	for frame := range frameCount {
		pos := frame * hop

		for i := range s.frameSize {
			x := 0.0

			idx := pos + i
			if idx < len(input) {
				x = input[idx]
			}

			s.analysisSpectrum[i] = complex(x*s.windowCoeffs[i], 0)
		}

		err := s.plan.Forward(s.analysisSpectrum, s.analysisSpectrum)
		if err != nil {
			return nil, fmt.Errorf("spectral freeze: forward FFT failed: %w", err)
		}

		justCaptured := false

		if s.frozen && !s.hasFrozenFrame {
			for k := 0; k <= half; k++ {
				re := real(s.analysisSpectrum[k])
				im := imag(s.analysisSpectrum[k])
				s.heldMagnitude[k] = math.Hypot(re, im)
				s.phaseAcc[k] = math.Atan2(im, re)
			}

			s.hasFrozenFrame = true
			justCaptured = true
		}

		if s.frozen && s.hasFrozenFrame {
			for k := 0; k <= half; k++ {
				if s.phaseMode == SpectralFreezePhaseAdvance && !justCaptured {
					s.phaseAcc[k] += s.omega[k] * hopF
				}

				phase := s.phaseAcc[k]
				mag := s.heldMagnitude[k]
				s.synthesisSpectrum[k] = complex(
					mag*math.Cos(phase),
					mag*math.Sin(phase),
				)
			}
		} else {
			for k := 0; k <= half; k++ {
				re := real(s.analysisSpectrum[k])
				im := imag(s.analysisSpectrum[k])
				mag := math.Hypot(re, im)
				phase := math.Atan2(im, re)
				s.synthesisSpectrum[k] = complex(
					mag*math.Cos(phase),
					mag*math.Sin(phase),
				)
			}
		}

		s.synthesisSpectrum[0] = complex(real(s.synthesisSpectrum[0]), 0)

		s.synthesisSpectrum[half] = complex(real(s.synthesisSpectrum[half]), 0)
		for k := 1; k < half; k++ {
			v := s.synthesisSpectrum[k]
			s.synthesisSpectrum[s.frameSize-k] = complex(real(v), -imag(v))
		}

		err = s.plan.Inverse(s.timeFrame, s.synthesisSpectrum)
		if err != nil {
			return nil, fmt.Errorf("spectral freeze: inverse FFT failed: %w", err)
		}

		for i := range s.frameSize {
			idx := pos + i
			w := s.windowCoeffs[i]
			wet[idx] += real(s.timeFrame[i]) * w
			norm[idx] += w * w
		}
	}

	out := make([]float64, len(input))
	for i := range out {
		sample := wet[i]
		if norm[i] > spectralFreezeNormFloor {
			sample /= norm[i]
		}

		out[i] = input[i]*(1-s.mix) + sample*s.mix
	}

	return out, nil
}

// ProcessInPlace applies spectral freeze to buf in place.
func (s *SpectralFreeze) ProcessInPlace(buf []float64) {
	out := s.Process(buf)
	copy(buf, out)
}

// ProcessInPlaceWithError applies spectral freeze in place and returns errors.
func (s *SpectralFreeze) ProcessInPlaceWithError(buf []float64) error {
	out, err := s.ProcessWithError(buf)
	if err != nil {
		return err
	}

	copy(buf, out)

	return nil
}

func (s *SpectralFreeze) validate() error {
	if s.sampleRate <= 0 || math.IsNaN(s.sampleRate) || math.IsInf(s.sampleRate, 0) {
		return fmt.Errorf("spectral freeze sample rate must be > 0: %f", s.sampleRate)
	}

	if s.frameSize < minSpectralFreezeFrameSize || !isPowerOf2SpectralFreeze(s.frameSize) {
		return fmt.Errorf("spectral freeze frame size must be power-of-two and >= %d: %d",
			minSpectralFreezeFrameSize, s.frameSize)
	}

	if s.hopSize <= 0 || s.hopSize >= s.frameSize {
		return fmt.Errorf("spectral freeze hop size must be in [1, %d): %d", s.frameSize, s.hopSize)
	}

	if s.mix < 0 || s.mix > 1 || math.IsNaN(s.mix) || math.IsInf(s.mix, 0) {
		return fmt.Errorf("spectral freeze mix must be in [0, 1]: %f", s.mix)
	}

	switch s.phaseMode {
	case SpectralFreezePhaseHold, SpectralFreezePhaseAdvance:
	default:
		return fmt.Errorf("spectral freeze phase mode invalid: %d", s.phaseMode)
	}

	if s.plan == nil {
		return errors.New("spectral freeze FFT plan is nil")
	}

	return nil
}

func (s *SpectralFreeze) rebuildState() error {
	if s.frameSize < minSpectralFreezeFrameSize || !isPowerOf2SpectralFreeze(s.frameSize) {
		return fmt.Errorf("spectral freeze frame size must be power-of-two and >= %d: %d",
			minSpectralFreezeFrameSize, s.frameSize)
	}

	if s.hopSize <= 0 || s.hopSize >= s.frameSize {
		return fmt.Errorf("spectral freeze hop size must be in [1, %d): %d", s.frameSize, s.hopSize)
	}

	plan, err := algofft.NewPlan64(s.frameSize)
	if err != nil {
		return fmt.Errorf("spectral freeze: failed to create FFT plan: %w", err)
	}

	s.plan = plan

	coeffs := window.Generate(s.windowType, s.frameSize, window.WithPeriodic())
	if len(coeffs) != s.frameSize {
		return fmt.Errorf("spectral freeze: window generation failed for size %d", s.frameSize)
	}

	s.windowCoeffs = coeffs

	bins := s.frameSize/2 + 1

	s.omega = make([]float64, bins)
	for k := range bins {
		s.omega[k] = 2 * math.Pi * float64(k) / float64(s.frameSize)
	}

	s.analysisSpectrum = make([]complex128, s.frameSize)
	s.synthesisSpectrum = make([]complex128, s.frameSize)
	s.timeFrame = make([]complex128, s.frameSize)
	s.heldMagnitude = make([]float64, bins)
	s.phaseAcc = make([]float64, bins)
	s.hasFrozenFrame = false

	return nil
}

func isPowerOf2SpectralFreeze(n int) bool {
	return n > 0 && (n&(n-1)) == 0
}
