package hilbert

import (
	"fmt"
	"math"
)

// Processor64 is a stateful 64-bit polyphase half-pi Hilbert transformer.
type Processor64 struct {
	coeffs []float64
	xMem   [2][]float64
	yMem   [2][]float64
	prev   float64
	phase  int

	transition    float64
	attenuationDB float64
}

// New64 creates a 64-bit Hilbert processor using designed coefficients.
func New64(numberOfCoeffs int, transition float64) (*Processor64, error) {
	coeffs, err := DesignCoefficients(numberOfCoeffs, transition)
	if err != nil {
		return nil, err
	}

	attenuationDB, err := AttenuationFromOrderTBW(numberOfCoeffs, transition)
	if err != nil {
		return nil, err
	}

	p := &Processor64{
		transition:    transition,
		attenuationDB: attenuationDB,
	}

	err = p.SetCoefficients(coeffs)
	if err != nil {
		return nil, err
	}

	p.transition = transition
	p.attenuationDB = attenuationDB

	return p, nil
}

// New64Default creates a 64-bit processor using legacy defaults.
func New64Default() (*Processor64, error) {
	return New64(DefaultCoefficientCount, DefaultTransition)
}

// New64FromCoefficients creates a 64-bit processor from explicit coefficients.
func New64FromCoefficients(coeffs []float64) (*Processor64, error) {
	p := &Processor64{transition: math.NaN(), attenuationDB: math.NaN()}

	err := p.SetCoefficients(coeffs)
	if err != nil {
		return nil, err
	}

	return p, nil
}

// SetDesign replaces coefficients using order + transition design parameters.
func (p *Processor64) SetDesign(numberOfCoeffs int, transition float64) error {
	coeffs, err := DesignCoefficients(numberOfCoeffs, transition)
	if err != nil {
		return err
	}

	attenuationDB, err := AttenuationFromOrderTBW(numberOfCoeffs, transition)
	if err != nil {
		return err
	}

	err = p.SetCoefficients(coeffs)
	if err != nil {
		return err
	}

	p.transition = transition
	p.attenuationDB = attenuationDB

	return nil
}

// SetCoefficients replaces allpass coefficients and clears internal state.
func (p *Processor64) SetCoefficients(coeffs []float64) error {
	err := validateCoefficients64(coeffs)
	if err != nil {
		return err
	}

	p.coeffs = append(p.coeffs[:0], coeffs...)
	for i := range p.xMem {
		if cap(p.xMem[i]) < len(coeffs) {
			p.xMem[i] = make([]float64, len(coeffs))
			p.yMem[i] = make([]float64, len(coeffs))
		} else {
			p.xMem[i] = p.xMem[i][:len(coeffs)]
			p.yMem[i] = p.yMem[i][:len(coeffs)]
		}
	}

	p.transition = math.NaN()
	p.attenuationDB = math.NaN()

	p.ClearBuffers()

	return nil
}

// ProcessSample processes one sample and returns quadrature outputs (A/B).
func (p *Processor64) ProcessSample(input float64) (outputA, outputB float64) {
	switch len(p.coeffs) {
	case 1:
		return p.processSample1(input)
	case 2:
		return p.processSample2(input)
	case 3:
		return p.processSample3(input)
	case 4:
		return p.processSample4(input)
	default:
		return p.processSampleLarge(input)
	}
}

// ProcessEnvelopeSample processes one sample and returns analytic magnitude.
func (p *Processor64) ProcessEnvelopeSample(input float64) float64 {
	a, b := p.ProcessSample(input)
	return math.Hypot(a, b)
}

// ProcessBlock processes input into outputA/outputB. All slices must have the
// same length.
func (p *Processor64) ProcessBlock(input, outputA, outputB []float64) error {
	if len(input) != len(outputA) || len(input) != len(outputB) {
		return fmt.Errorf("hilbert: ProcessBlock slice length mismatch: in=%d outA=%d outB=%d",
			len(input), len(outputA), len(outputB))
	}

	for i := range input {
		outputA[i], outputB[i] = p.ProcessSample(input[i])
	}

	return nil
}

// ProcessEnvelopeBlock processes input into envelope output.
func (p *Processor64) ProcessEnvelopeBlock(input, envelope []float64) error {
	if len(input) != len(envelope) {
		return fmt.Errorf("hilbert: ProcessEnvelopeBlock slice length mismatch: in=%d env=%d",
			len(input), len(envelope))
	}

	for i := range input {
		envelope[i] = p.ProcessEnvelopeSample(input[i])
	}

	return nil
}

// ClearBuffers clears internal filter memories.
func (p *Processor64) ClearBuffers() {
	for ph := range p.xMem {
		for i := range p.xMem[ph] {
			p.xMem[ph][i] = 0
			p.yMem[ph][i] = 0
		}
	}

	p.prev = 0
	p.phase = 0
}

// Reset is an alias for [Processor64.ClearBuffers].
func (p *Processor64) Reset() {
	p.ClearBuffers()
}

// Coefficients returns a copy of configured coefficients.
func (p *Processor64) Coefficients() []float64 {
	out := make([]float64, len(p.coeffs))
	copy(out, p.coeffs)

	return out
}

// NumberOfCoefficients returns the configured coefficient count.
func (p *Processor64) NumberOfCoefficients() int {
	return len(p.coeffs)
}

// Transition returns design transition bandwidth, or NaN for direct coeff mode.
func (p *Processor64) Transition() float64 {
	return p.transition
}

// AttenuationDB returns design attenuation, or NaN for direct coeff mode.
func (p *Processor64) AttenuationDB() float64 {
	return p.attenuationDB
}

func (p *Processor64) processSample1(input float64) (outputA, outputB float64) {
	phase := p.phase
	y := p.yMem[phase]
	x := p.xMem[phase]

	y[0] = (input+y[0])*p.coeffs[0] - x[0]
	x[0] = input
	outputA = y[0]
	outputB = p.prev

	p.prev = input
	p.phase = 1 - p.phase

	return outputA, outputB
}

func (p *Processor64) processSample2(input float64) (outputA, outputB float64) {
	phase := p.phase
	y := p.yMem[phase]
	x := p.xMem[phase]

	y[0] = (input+y[0])*p.coeffs[0] - x[0]
	x[0] = input
	y[1] = (p.prev+y[1])*p.coeffs[1] - x[1]
	x[1] = p.prev
	outputA = y[0]
	outputB = y[1]

	p.prev = input
	p.phase = 1 - p.phase

	return outputA, outputB
}

func (p *Processor64) processSample3(input float64) (outputA, outputB float64) {
	phase := p.phase
	y := p.yMem[phase]
	x := p.xMem[phase]

	y[0] = (input+y[0])*p.coeffs[0] - x[0]
	x[0] = input
	y[1] = (p.prev+y[1])*p.coeffs[1] - x[1]
	x[1] = p.prev
	y[2] = (y[0]+y[2])*p.coeffs[2] - x[2]
	x[2] = y[0]
	outputA = y[1]
	outputB = y[2]

	p.prev = input
	p.phase = 1 - p.phase

	return outputA, outputB
}

//nolint:dupl
func (p *Processor64) processSample4(input float64) (outputA, outputB float64) {
	phase := p.phase
	y := p.yMem[phase]
	x := p.xMem[phase]

	y[0] = (input+y[0])*p.coeffs[0] - x[0]
	x[0] = input
	y[1] = (p.prev+y[1])*p.coeffs[1] - x[1]
	x[1] = p.prev
	y[2] = (y[0]+y[2])*p.coeffs[2] - x[2]
	x[2] = y[0]
	y[3] = (y[1]+y[3])*p.coeffs[3] - x[3]
	x[3] = y[1]
	outputA = y[2]
	outputB = y[3]

	p.prev = input
	p.phase = 1 - p.phase

	return outputA, outputB
}

func (p *Processor64) processSampleLarge(input float64) (outputA, outputB float64) {
	phase := p.phase
	y := p.yMem[phase]
	x := p.xMem[phase]

	y[0] = (input+y[0])*p.coeffs[0] - x[0]
	x[0] = input
	y[1] = (p.prev+y[1])*p.coeffs[1] - x[1]
	x[1] = p.prev

	for i := 2; i < len(p.coeffs); i++ {
		y[i] = (y[i-2]+y[i])*p.coeffs[i] - x[i]
		x[i] = y[i-2]
	}

	last := len(p.coeffs) - 1
	outputA = y[last-1]
	outputB = y[last]

	p.prev = input
	p.phase = 1 - p.phase

	return outputA, outputB
}
