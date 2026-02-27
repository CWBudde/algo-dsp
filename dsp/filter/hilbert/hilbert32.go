package hilbert

import (
	"fmt"
	"math"
)

// Processor32 is a stateful 32-bit polyphase half-pi Hilbert transformer.
type Processor32 struct {
	coeffs []float32
	xMem   [2][]float32
	yMem   [2][]float32
	prev   float32
	phase  int

	transition    float64
	attenuationDB float64
}

// New32 creates a 32-bit Hilbert processor using designed coefficients.
func New32(numberOfCoeffs int, transition float64) (*Processor32, error) {
	coeffs64, err := DesignCoefficients(numberOfCoeffs, transition)
	if err != nil {
		return nil, err
	}

	coeffs := make([]float32, len(coeffs64))
	for i, c := range coeffs64 {
		coeffs[i] = float32(c)
	}

	attenuationDB, err := AttenuationFromOrderTBW(numberOfCoeffs, transition)
	if err != nil {
		return nil, err
	}

	p := &Processor32{
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

// New32Default creates a 32-bit processor using legacy defaults.
func New32Default() (*Processor32, error) {
	return New32(DefaultCoefficientCount, DefaultTransition)
}

// New32FromCoefficients creates a 32-bit processor from explicit coefficients.
func New32FromCoefficients(coeffs []float32) (*Processor32, error) {
	p := &Processor32{transition: math.NaN(), attenuationDB: math.NaN()}

	err := p.SetCoefficients(coeffs)
	if err != nil {
		return nil, err
	}

	return p, nil
}

// SetDesign replaces coefficients using order + transition design parameters.
func (p *Processor32) SetDesign(numberOfCoeffs int, transition float64) error {
	coeffs64, err := DesignCoefficients(numberOfCoeffs, transition)
	if err != nil {
		return err
	}

	coeffs := make([]float32, len(coeffs64))
	for i, c := range coeffs64 {
		coeffs[i] = float32(c)
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
func (p *Processor32) SetCoefficients(coeffs []float32) error {
	err := validateCoefficients32(coeffs)
	if err != nil {
		return err
	}

	p.coeffs = append(p.coeffs[:0], coeffs...)
	for i := range p.xMem {
		if cap(p.xMem[i]) < len(coeffs) {
			p.xMem[i] = make([]float32, len(coeffs))
			p.yMem[i] = make([]float32, len(coeffs))
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
func (p *Processor32) ProcessSample(input float32) (outputA, outputB float32) {
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
func (p *Processor32) ProcessEnvelopeSample(input float32) float32 {
	a, b := p.ProcessSample(input)
	return float32(math.Hypot(float64(a), float64(b)))
}

// ProcessBlock processes input into outputA/outputB. All slices must have the
// same length.
func (p *Processor32) ProcessBlock(input, outputA, outputB []float32) error {
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
func (p *Processor32) ProcessEnvelopeBlock(input, envelope []float32) error {
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
func (p *Processor32) ClearBuffers() {
	for ph := range p.xMem {
		for i := range p.xMem[ph] {
			p.xMem[ph][i] = 0
			p.yMem[ph][i] = 0
		}
	}

	p.prev = 0
	p.phase = 0
}

// Reset is an alias for [Processor32.ClearBuffers].
func (p *Processor32) Reset() {
	p.ClearBuffers()
}

// Coefficients returns a copy of configured coefficients.
func (p *Processor32) Coefficients() []float32 {
	out := make([]float32, len(p.coeffs))
	copy(out, p.coeffs)

	return out
}

// NumberOfCoefficients returns the configured coefficient count.
func (p *Processor32) NumberOfCoefficients() int {
	return len(p.coeffs)
}

// Transition returns design transition bandwidth, or NaN for direct coeff mode.
func (p *Processor32) Transition() float64 {
	return p.transition
}

// AttenuationDB returns design attenuation, or NaN for direct coeff mode.
func (p *Processor32) AttenuationDB() float64 {
	return p.attenuationDB
}

func (p *Processor32) processSample1(input float32) (outputA, outputB float32) {
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

func (p *Processor32) processSample2(input float32) (outputA, outputB float32) {
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

func (p *Processor32) processSample3(input float32) (outputA, outputB float32) {
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
func (p *Processor32) processSample4(input float32) (outputA, outputB float32) {
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

func (p *Processor32) processSampleLarge(input float32) (outputA, outputB float32) {
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
