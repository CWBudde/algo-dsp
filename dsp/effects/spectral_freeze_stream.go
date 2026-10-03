package effects

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/internal/streamframe"
)

// StreamingSpectralFreeze schedules causal freeze frames independently of block
// partitioning, with bounded overlap-add history. The processor owns its FFT
// plans and configuration and must not be reconfigured during processing.
type StreamingSpectralFreeze struct {
	processor *SpectralFreeze
	stream    *streamframe.Engine
}

// NewStreamingSpectralFreeze reserves render storage and rewinds the processor.
func NewStreamingSpectralFreeze(processor *SpectralFreeze) (*StreamingSpectralFreeze, error) {
	if processor == nil {
		return nil, fmt.Errorf("spectral freeze stream: nil processor")
	}

	if err := processor.validate(); err != nil {
		return nil, fmt.Errorf("spectral freeze stream: %w", err)
	}

	p := &StreamingSpectralFreeze{processor: processor}

	engine, err := streamframe.New(processor.windowCoeffs, processor.hopSize, p.transform)
	if err != nil {
		return nil, fmt.Errorf("spectral freeze stream: %w", err)
	}

	p.stream = engine
	p.Reset()

	return p, nil
}

// Latency returns the lookahead for a complete first analysis frame.
func (p *StreamingSpectralFreeze) Latency() int { return p.stream.Latency() }

// Reset clears held spectra, phases and overlap-add history without allocation.
func (p *StreamingSpectralFreeze) Reset() { p.processor.Reset(); p.stream.Reset() }

// ProcessInPlace processes arbitrary block sizes without allocation. Dry and
// wet paths share the same latency. Feed zeros to drain the final spectral tail.
func (p *StreamingSpectralFreeze) ProcessInPlace(block []float64) error {
	return p.stream.ProcessInPlace(block, p.processor.mix)
}

func (p *StreamingSpectralFreeze) transform(input, output []float64) error {
	s := p.processor
	half := s.frameSize / 2

	hop := float64(s.hopSize)
	for i, x := range input {
		s.analysisSpectrum[i] = complex(x*s.windowCoeffs[i], 0)
	}

	if err := s.plan.Forward(s.analysisSpectrum, s.analysisSpectrum); err != nil {
		return fmt.Errorf("spectral freeze stream: forward: %w", err)
	}

	captured := false

	if s.frozen && !s.hasFrozenFrame {
		for k := 0; k <= half; k++ {
			re, im := real(s.analysisSpectrum[k]), imag(s.analysisSpectrum[k])
			s.heldMagnitude[k] = math.Hypot(re, im)
			s.phaseAcc[k] = math.Atan2(im, re)
		}

		s.hasFrozenFrame = true
		captured = true
	}

	for k := 0; k <= half; k++ {
		if s.frozen && s.hasFrozenFrame {
			if s.phaseMode == SpectralFreezePhaseAdvance && !captured {
				s.phaseAcc[k] += s.omega[k] * hop
			}

			phase, mag := s.phaseAcc[k], s.heldMagnitude[k]
			s.synthesisSpectrum[k] = complex(mag*math.Cos(phase), mag*math.Sin(phase))
		} else {
			s.synthesisSpectrum[k] = s.analysisSpectrum[k]
		}
	}

	s.synthesisSpectrum[0] = complex(real(s.synthesisSpectrum[0]), 0)

	s.synthesisSpectrum[half] = complex(real(s.synthesisSpectrum[half]), 0)
	for k := 1; k < half; k++ {
		v := s.synthesisSpectrum[k]
		s.synthesisSpectrum[s.frameSize-k] = complex(real(v), -imag(v))
	}

	if err := s.plan.Inverse(s.timeFrame, s.synthesisSpectrum); err != nil {
		return fmt.Errorf("spectral freeze stream: inverse: %w", err)
	}

	for i := range output {
		output[i] = real(s.timeFrame[i]) * s.windowCoeffs[i]
	}

	return nil
}
