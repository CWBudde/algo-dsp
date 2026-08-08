package biquad

// Chain is an ordered cascade of biquad sections processed in series.
// It is used for higher-order filters (Butterworth, Chebyshev, etc.)
// where each second-order section feeds into the next.
type Chain struct {
	sections []Section
	gain     float64
}

// chainConfig holds options for NewChain.
type chainConfig struct {
	gain float64
}

// ChainOption configures a Chain.
type ChainOption func(*chainConfig)

// WithGain sets an overall gain applied to the input before cascading.
// Default is 1.0 (unity gain).
func WithGain(g float64) ChainOption {
	return func(cfg *chainConfig) { cfg.gain = g }
}

// NewChain creates a cascade from one or more coefficient sets.
// Each Coefficients value becomes one Section in the cascade.
func NewChain(coeffs []Coefficients, opts ...ChainOption) *Chain {
	cfg := chainConfig{gain: 1}
	for _, o := range opts {
		o(&cfg)
	}

	c := &Chain{
		sections: make([]Section, len(coeffs)),
		gain:     cfg.gain,
	}
	for i := range coeffs {
		c.sections[i].Coefficients = coeffs[i]
	}

	return c
}

// ProcessSample cascades input through all sections in order.
// If gain != 1, the input is scaled before the first section.
//
// This mirrors the legacy TMFDSPButterworthLP.ProcessSample
// (MFFilter.pas:1374–1395) where each section's output feeds the next.
func (c *Chain) ProcessSample(x float64) float64 {
	x *= c.gain
	for i := range c.sections {
		x = c.sections[i].ProcessSample(x)
	}

	return x
}

// ProcessBlock filters a block in-place through the full cascade.
func (c *Chain) ProcessBlock(buf []float64) {
	if c.gain != 1 {
		for i, x := range buf {
			buf[i] = x * c.gain
		}
	}

	for i := range c.sections {
		c.sections[i].ProcessBlock(buf)
	}
}

// Reset clears all section states.
func (c *Chain) Reset() {
	for i := range c.sections {
		c.sections[i].Reset()
	}
}

// Order returns the total filter order (2 per full biquad section).
func (c *Chain) Order() int {
	return 2 * len(c.sections)
}

// NumSections returns the number of biquad sections.
func (c *Chain) NumSections() int {
	return len(c.sections)
}

// Gain returns the current input gain applied before cascading.
func (c *Chain) Gain() float64 { return c.gain }

// SetGain updates the input gain applied before cascading.
func (c *Chain) SetGain(g float64) { c.gain = g }

// Section returns a pointer to the i-th section for inspection or modification.
func (c *Chain) Section(i int) *Section {
	return &c.sections[i]
}

// FlushDenormals flushes denormal delay-line state in every section to exact
// zero. See [Section.FlushDenormals] for the rationale.
//
// It is allocation-free, intended to be called once per block from a
// real-time audio callback.
func (c *Chain) FlushDenormals() {
	for i := range c.sections {
		c.sections[i].FlushDenormals()
	}
}

// State returns a snapshot of all section delay-line states.
//
// It allocates a new slice on every call and is therefore unsuitable for use
// inside a real-time audio callback; take snapshots off the audio thread, or
// use [Chain.Section] together with [Section.State] to read a single section
// without allocating.
func (c *Chain) State() [][2]float64 {
	states := make([][2]float64, len(c.sections))
	for i := range c.sections {
		states[i] = c.sections[i].State()
	}

	return states
}

// SetState restores previously saved section states.
//
// The slice length must be at least [Chain.NumSections]; a shorter slice
// panics with an index-out-of-range error. SetState itself does not allocate,
// but the caller must keep the snapshot slice alive, so a save/restore round
// trip through [Chain.State] does.
func (c *Chain) SetState(states [][2]float64) {
	for i := range c.sections {
		c.sections[i].SetState(states[i])
	}
}
