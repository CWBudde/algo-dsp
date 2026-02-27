//nolint:funcorder
package spectrum

import (
	"fmt"
	"math"
)

// Goertzel implements the Goertzel algorithm for single-bin frequency analysis.
//
// The Goertzel algorithm is an efficient way to evaluate individual terms
// of the Discrete Fourier Transform (DFT) without computing the entire FFT.
// It is particularly useful for tone detection (e.g., DTMF) or pilot tone
// analysis.
//
// Behavior and Semantics:
//
// The analyzer is stateful and accumulates information from each processed
// sample. Power() and Magnitude() evaluate the frequency component based on
// all samples processed since the last Reset().
//
// For robust detection, the block size (N) should be chosen based on the desired
// frequency resolution. The main lobe width of the Goertzel filter is 4*pi/N.
// To distinguish between two frequencies, N should be large enough such that
// the frequencies are separated by more than 2*pi/N.
//
// Spectral leakage occurs if the target frequency does not align with an
// integer number of cycles within the processed block. Windowing the input
// signal before processing can reduce leakage at the cost of widening the
// main lobe.
type Goertzel struct {
	frequency  float64
	sampleRate float64
	coeff      float64
	s0, s1     float64
}

// NewGoertzel creates a new Goertzel analyzer for the target frequency.
//
// frequency must be between 0 and sampleRate/2.
func NewGoertzel(frequency, sampleRate float64) (*Goertzel, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("goertzel: sample rate must be > 0: %v", sampleRate)
	}

	if frequency < 0 || frequency > sampleRate/2 || math.IsNaN(frequency) || math.IsInf(frequency, 0) {
		return nil, fmt.Errorf("goertzel: frequency must be between 0 and sampleRate/2: %v", frequency)
	}

	g := &Goertzel{
		frequency:  frequency,
		sampleRate: sampleRate,
	}
	g.updateCoeff()

	return g, nil
}

func (g *Goertzel) updateCoeff() {
	g.coeff = 2 * math.Cos(2*math.Pi*g.frequency/g.sampleRate)
}

// Reset clears the internal state.
func (g *Goertzel) Reset() {
	g.s0 = 0
	g.s1 = 0
}

// ProcessSample updates the internal state with a single input sample.
func (g *Goertzel) ProcessSample(input float64) {
	s := input + g.coeff*g.s0 - g.s1
	g.s1 = g.s0
	g.s0 = s
}

// ProcessBlock updates the internal state with a block of samples.
func (g *Goertzel) ProcessBlock(input []float64) {
	s0, s1 := g.s0, g.s1

	coeff := g.coeff
	for _, x := range input {
		s := x + coeff*s0 - s1
		s1 = s0
		s0 = s
	}

	g.s0, g.s1 = s0, s1
}

// Power returns the squared magnitude of the frequency component.
//
// This is typically called after processing a block of samples.
// The result is equivalent to |X[k]|^2 from a DFT of the same block length.
func (g *Goertzel) Power() float64 {
	return g.s0*g.s0 + g.s1*g.s1 - g.coeff*g.s0*g.s1
}

// Magnitude returns the magnitude of the frequency component.
func (g *Goertzel) Magnitude() float64 {
	p := g.Power()
	if p <= 0 {
		return 0
	}

	return math.Sqrt(p)
}

// PowerDB returns the power in decibels (dB) with a safe floor at -300 dB.
func (g *Goertzel) PowerDB() float64 {
	p := g.Power()
	if p <= 1e-30 {
		return -300
	}

	return 10 * math.Log10(p)
}

// SetFrequency updates the target frequency.
func (g *Goertzel) SetFrequency(frequency float64) error {
	if frequency < 0 || frequency > g.sampleRate/2 || math.IsNaN(frequency) || math.IsInf(frequency, 0) {
		return fmt.Errorf("goertzel: frequency must be between 0 and sampleRate/2: %v", frequency)
	}

	g.frequency = frequency
	g.updateCoeff()

	return nil
}

// SetSampleRate updates the sample rate.
func (g *Goertzel) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("goertzel: sample rate must be > 0: %v", sampleRate)
	}

	g.sampleRate = sampleRate
	g.updateCoeff()

	return nil
}

// Frequency returns the current target frequency.
func (g *Goertzel) Frequency() float64 { return g.frequency }

// SampleRate returns the current sample rate.
func (g *Goertzel) SampleRate() float64 { return g.sampleRate }

// AnalyzeBlock computes the Goertzel power for a single frequency in one shot.
func AnalyzeBlock(input []float64, frequency, sampleRate float64) (float64, error) {
	g, err := NewGoertzel(frequency, sampleRate)
	if err != nil {
		return 0, err
	}

	g.ProcessBlock(input)

	return g.Power(), nil
}

// MultiGoertzel manages multiple Goertzel analyzers for batch processing.
type MultiGoertzel struct {
	analyzers []*Goertzel
}

// NewMultiGoertzel creates a set of Goertzel analyzers for multiple frequencies.
func NewMultiGoertzel(frequencies []float64, sampleRate float64) (*MultiGoertzel, error) {
	analyzers := make([]*Goertzel, len(frequencies))
	for i, f := range frequencies {
		g, err := NewGoertzel(f, sampleRate)
		if err != nil {
			return nil, err
		}

		analyzers[i] = g
	}

	return &MultiGoertzel{analyzers: analyzers}, nil
}

// ProcessBlock updates all analyzers with the same input block.
func (m *MultiGoertzel) ProcessBlock(input []float64) {
	for _, g := range m.analyzers {
		g.ProcessBlock(input)
	}
}

// Powers returns the powers for all analyzers.
func (m *MultiGoertzel) Powers() []float64 {
	p := make([]float64, len(m.analyzers))
	for i, g := range m.analyzers {
		p[i] = g.Power()
	}

	return p
}

// Reset resets all analyzers.
func (m *MultiGoertzel) Reset() {
	for _, g := range m.analyzers {
		g.Reset()
	}
}
