package crossover

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design/pass"
)

// Crossover is a two-way Linkwitz-Riley crossover network that splits
// an input signal into complementary lowpass and highpass outputs.
//
// The lowpass and highpass outputs sum to an allpass-filtered version
// of the input (flat magnitude response). Polarity correction for
// orders ≡ 2 mod 4 (LR2, LR6, …) is handled automatically.
type Crossover struct {
	lp    *biquad.Chain
	hp    *biquad.Chain
	freq  float64
	order int
	sr    float64
}

// New creates a two-way Linkwitz-Riley crossover at the given frequency
// and order. The order must be a positive even integer (2, 4, 6, 8, …).
//
// For orders ≡ 2 mod 4 (LR2, LR6, …), the HP polarity is automatically
// inverted so that LP + HP = allpass for all even orders.
//
// Returns an error for invalid parameters.
func New(freq float64, order int, sampleRate float64) (*Crossover, error) {
	if order <= 0 || order%2 != 0 {
		return nil, fmt.Errorf("crossover: order must be a positive even integer, got %d", order)
	}
	if sampleRate <= 0 {
		return nil, fmt.Errorf("crossover: sample rate must be positive, got %v", sampleRate)
	}
	if freq <= 0 || freq >= sampleRate/2 {
		return nil, fmt.Errorf("crossover: frequency must be in (0, %v), got %v", sampleRate/2, freq)
	}

	lpCoeffs := pass.LinkwitzRileyLP(freq, order, sampleRate)
	var hpCoeffs []biquad.Coefficients
	if pass.LinkwitzRileyNeedsHPInvert(order) {
		hpCoeffs = pass.LinkwitzRileyHPInverted(freq, order, sampleRate)
	} else {
		hpCoeffs = pass.LinkwitzRileyHP(freq, order, sampleRate)
	}
	if lpCoeffs == nil || hpCoeffs == nil {
		return nil, fmt.Errorf("crossover: failed to design LR%d at %.1f Hz", order, freq)
	}

	return &Crossover{
		lp:    biquad.NewChain(lpCoeffs),
		hp:    biquad.NewChain(hpCoeffs),
		freq:  freq,
		order: order,
		sr:    sampleRate,
	}, nil
}

// ProcessSample filters one input sample and returns the lowpass and
// highpass outputs. Their sum is allpass (flat magnitude response).
func (c *Crossover) ProcessSample(x float64) (lo, hi float64) {
	return c.lp.ProcessSample(x), c.hp.ProcessSample(x)
}

// ProcessBlock filters a block of input samples, writing the lowpass
// output to lo and the highpass output to hi. All three slices must
// have the same length.
func (c *Crossover) ProcessBlock(input, lo, hi []float64) {
	n := len(input)
	if n == 0 {
		return
	}
	_ = lo[n-1]
	_ = hi[n-1]
	copy(lo, input)
	copy(hi, input)
	c.lp.ProcessBlock(lo)
	c.hp.ProcessBlock(hi)
}

// LP returns the lowpass chain for direct inspection or analysis.
func (c *Crossover) LP() *biquad.Chain { return c.lp }

// HP returns the highpass chain for direct inspection or analysis.
// For orders ≡ 2 mod 4, this chain includes the polarity inversion.
func (c *Crossover) HP() *biquad.Chain { return c.hp }

// Freq returns the crossover frequency in Hz.
func (c *Crossover) Freq() float64 { return c.freq }

// Order returns the Linkwitz-Riley order (always even).
func (c *Crossover) Order() int { return c.order }

// SampleRate returns the sample rate in Hz.
func (c *Crossover) SampleRate() float64 { return c.sr }

// Reset clears the internal filter states of both LP and HP chains.
func (c *Crossover) Reset() {
	c.lp.Reset()
	c.hp.Reset()
}

// MultiBand is a multi-way crossover network built from cascaded two-way
// Linkwitz-Riley crossovers. It splits an input signal into N+1 frequency
// bands for N crossover frequencies.
//
// The bands are ordered from lowest to highest frequency. The cascade
// topology passes each stage's highpass output as the next stage's input,
// so the sum of all band outputs equals LP₁ + HP₁·AP₂·…·APₙ rather than a
// single allpass. For a two-band (one crossover) network this is exact. For
// three or more bands, the magnitude flatness degrades as crossover
// frequencies become closer; the error is negligible when crossovers are
// spaced at least an octave apart.
type MultiBand struct {
	stages []*Crossover
	bands  int
}

// NewMultiBand creates a multi-way crossover from the given crossover
// frequencies and order. Frequencies must be in strictly ascending order
// and all within (0, sampleRate/2). The order applies to all crossover
// points and must be a positive even integer.
//
// For N frequencies, the crossover produces N+1 output bands.
func NewMultiBand(freqs []float64, order int, sampleRate float64) (*MultiBand, error) {
	if len(freqs) == 0 {
		return nil, fmt.Errorf("crossover: at least one frequency is required")
	}
	for i := 1; i < len(freqs); i++ {
		if freqs[i] <= freqs[i-1] {
			return nil, fmt.Errorf("crossover: frequencies must be strictly ascending, got %.1f after %.1f", freqs[i], freqs[i-1])
		}
	}

	stages := make([]*Crossover, len(freqs))
	for i, f := range freqs {
		xo, err := New(f, order, sampleRate)
		if err != nil {
			return nil, fmt.Errorf("crossover: stage %d: %w", i, err)
		}
		stages[i] = xo
	}

	return &MultiBand{
		stages: stages,
		bands:  len(freqs) + 1,
	}, nil
}

// NumBands returns the number of output bands.
func (m *MultiBand) NumBands() int { return m.bands }

// Stages returns the underlying two-way crossover stages.
func (m *MultiBand) Stages() []*Crossover { return m.stages }

// ProcessSample filters one input sample and returns per-band outputs.
// The returned slice has NumBands() elements, ordered from lowest to
// highest frequency band.
//
// At each stage the highpass output becomes the next stage's input, and the
// lowpass output is the current band output. The final highpass is the
// highest band. See the MultiBand documentation for the reconstruction
// accuracy of this cascade topology.
func (m *MultiBand) ProcessSample(x float64) []float64 {
	out := make([]float64, m.bands)
	remainder := x
	for i, stage := range m.stages {
		lo, hi := stage.ProcessSample(remainder)
		out[i] = lo
		remainder = hi
	}
	out[m.bands-1] = remainder
	return out
}

// ProcessBlock filters a block of input samples and returns per-band
// output blocks. The returned slice has NumBands() elements, each of
// the same length as input.
func (m *MultiBand) ProcessBlock(input []float64) [][]float64 {
	n := len(input)
	out := make([][]float64, m.bands)
	for i := range out {
		out[i] = make([]float64, n)
	}

	// Work buffer for the running remainder (starts as input).
	remainder := make([]float64, n)
	copy(remainder, input)

	hi := make([]float64, n)
	for i, stage := range m.stages {
		stage.ProcessBlock(remainder, out[i], hi)
		copy(remainder, hi)
	}
	copy(out[m.bands-1], remainder)
	return out
}

// Reset clears all internal filter states.
func (m *MultiBand) Reset() {
	for _, s := range m.stages {
		s.Reset()
	}
}
