package spectrum

import (
	"fmt"
	"math"
)

// defaultGoertzelPowerFloor is the smallest power value used by [Goertzel.DB]
// before taking the logarithm, preventing -Inf for silent input.
const defaultGoertzelPowerFloor = 1e-300

// GoertzelOption mutates Goertzel analyzer construction parameters.
type GoertzelOption func(*goertzelConfig) error

type goertzelConfig struct {
	powerFloor float64
}

func defaultGoertzelConfig() goertzelConfig {
	return goertzelConfig{
		powerFloor: defaultGoertzelPowerFloor,
	}
}

// WithGoertzelPowerFloor sets the minimum power used by [Goertzel.DB] before
// the logarithm is taken. It must be > 0 and finite. The default is 1e-300.
func WithGoertzelPowerFloor(floor float64) GoertzelOption {
	return func(cfg *goertzelConfig) error {
		if floor <= 0 || math.IsNaN(floor) || math.IsInf(floor, 0) {
			return fmt.Errorf("goertzel power floor must be > 0 and finite: %f", floor)
		}

		cfg.powerFloor = floor

		return nil
	}
}

// Goertzel is a stateful single-bin Goertzel analyzer. It estimates the energy
// at one target frequency using the second-order recurrence
//
//	s = in + coef*s0 - s1;  s1 = s0;  s0 = s
//
// with coef = 2*cos(2*pi*f/fs). Unlike a full FFT it evaluates a single
// frequency, which is efficient for tone detection (DTMF, pilot tones,
// third-octave analysis). The target frequency need not align with a DFT bin.
//
// Window semantics: [Goertzel.ProcessSample] and [Goertzel.ProcessBlock]
// accumulate continuously. To obtain block-finalized, DFT-comparable metrics,
// call [Goertzel.Reset] before feeding a finite block of N samples, then read
// the result via [Goertzel.Power], [Goertzel.Magnitude], [Goertzel.Complex],
// or [Goertzel.NormalizedMagnitude].
type Goertzel struct {
	frequency  float64
	sampleRate float64
	powerFloor float64

	coef float64 // 2*cos(w)
	cosW float64
	sinW float64

	s0 float64 // most recent state
	s1 float64 // previous state
}

// NewGoertzel creates a single-bin Goertzel analyzer for the given target
// frequency (Hz) and sample rate (Hz). The frequency must satisfy
// 0 < frequency < sampleRate/2.
func NewGoertzel(frequency, sampleRate float64, opts ...GoertzelOption) (*Goertzel, error) {
	if err := validateGoertzelRates(frequency, sampleRate); err != nil {
		return nil, err
	}

	cfg := defaultGoertzelConfig()

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	goertzel := &Goertzel{
		frequency:  frequency,
		sampleRate: sampleRate,
		powerFloor: cfg.powerFloor,
	}
	goertzel.updateCoefficient()

	return goertzel, nil
}

func validateGoertzelRates(frequency, sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("goertzel sample rate must be > 0 and finite: %f", sampleRate)
	}

	if frequency <= 0 || math.IsNaN(frequency) || math.IsInf(frequency, 0) {
		return fmt.Errorf("goertzel frequency must be > 0 and finite: %f", frequency)
	}

	if frequency >= sampleRate/2 {
		return fmt.Errorf("goertzel frequency must be < sampleRate/2 (%g): %f", sampleRate/2, frequency)
	}

	return nil
}

func (g *Goertzel) updateCoefficient() {
	w := 2 * math.Pi * g.frequency / g.sampleRate
	g.cosW = math.Cos(w)
	g.sinW = math.Sin(w)
	g.coef = 2 * g.cosW
}

// SetFrequency updates the target frequency and recomputes the coefficient.
// The analyzer state is left untouched; call [Goertzel.Reset] for a clean
// measurement at the new frequency.
func (g *Goertzel) SetFrequency(frequency float64) error {
	if err := validateGoertzelRates(frequency, g.sampleRate); err != nil {
		return err
	}

	g.frequency = frequency
	g.updateCoefficient()

	return nil
}

// SetSampleRate updates the sample rate and recomputes the coefficient. The
// current frequency must remain below the new Nyquist limit.
func (g *Goertzel) SetSampleRate(sampleRate float64) error {
	if err := validateGoertzelRates(g.frequency, sampleRate); err != nil {
		return err
	}

	g.sampleRate = sampleRate
	g.updateCoefficient()

	return nil
}

// Reset clears the accumulated analyzer state.
func (g *Goertzel) Reset() {
	g.s0 = 0
	g.s1 = 0
}

// ProcessSample feeds one input sample into the analyzer and returns the input
// unchanged, allowing the analyzer to be inserted transparently into a chain.
func (g *Goertzel) ProcessSample(in float64) float64 {
	g.s0, g.s1 = goertzelStep(in, g.coef, g.s0, g.s1), g.s0

	return in
}

// ProcessBlock feeds an entire block of samples into the analyzer. The state is
// hoisted into local variables for the duration of the loop so the recurrence
// runs in registers rather than through repeated struct-field accesses.
func (g *Goertzel) ProcessBlock(buf []float64) {
	s0, s1, coef := g.s0, g.s1, g.coef
	for _, x := range buf {
		s0, s1 = goertzelStep(x, coef, s0, s1), s0
	}

	g.s0, g.s1 = s0, s1
}

// goertzelStep is the second-order recurrence s = in + coef*s0 - s1, factored
// out so every caller -- single bin, bank, and the four-wide bank kernel --
// compiles the identical expression and cannot diverge in how the compiler
// contracts the multiply-add.
func goertzelStep(in, coef, s0, s1 float64) float64 {
	return in + coef*s0 - s1
}

// Power returns the accumulated power |X|^2 at the target frequency using the
// Goertzel state: s0^2 + s1^2 - coef*s0*s1.
func (g *Goertzel) Power() float64 {
	return g.s0*g.s0 + g.s1*g.s1 - g.coef*g.s0*g.s1
}

// Magnitude returns the accumulated magnitude |X| at the target frequency.
// Power is theoretically non-negative; a small negative value caused by
// floating-point roundoff is clamped to zero so the result never becomes NaN.
func (g *Goertzel) Magnitude() float64 {
	power := g.Power()
	if power < 0 {
		power = 0
	}

	return math.Sqrt(power)
}

// DB returns the power in decibels, 10*log10(Power), floored to avoid -Inf for
// silent input (see [WithGoertzelPowerFloor]).
func (g *Goertzel) DB() float64 {
	p := g.Power()
	if p < g.powerFloor {
		p = g.powerFloor
	}

	return 10 * math.Log10(p)
}

// Complex returns the complex spectral value at the target frequency in the
// standard closed form re = s0 - s1*cos(w), im = s1*sin(w). When the state
// represents exactly one finite block (i.e. after [Goertzel.Reset] followed by
// processing N samples) its magnitude equals the magnitude of the corresponding
// DFT bin (the phase carries a constant rotation relative to a textbook DFT).
// With continuous accumulation the returned value is not DFT-comparable.
func (g *Goertzel) Complex() complex128 {
	return complex(g.s0-g.s1*g.cosW, g.s1*g.sinW)
}

// NormalizedMagnitude returns the amplitude estimate of a tone at the target
// frequency for a block of n samples: Magnitude * 2 / n. For a pure cosine of
// amplitude A measured over an integer number of cycles, this returns ~A.
func (g *Goertzel) NormalizedMagnitude(n int) float64 {
	if n <= 0 {
		return 0
	}

	return g.Magnitude() * 2 / float64(n)
}

// Frequency returns the target frequency in Hz.
func (g *Goertzel) Frequency() float64 { return g.frequency }

// SampleRate returns the sample rate in Hz.
func (g *Goertzel) SampleRate() float64 { return g.sampleRate }

// AnalyzeBlock computes the Goertzel power |X|^2 at a single target frequency
// over one finite block in a single call. It creates a transient analyzer,
// processes the block from a cleared state, and returns the power, so the result
// is comparable to the corresponding DFT bin of a block of the same length. For
// repeated analysis of the same frequency, reuse a [Goertzel] instead to avoid
// re-deriving the coefficient on every call.
func AnalyzeBlock(input []float64, frequency, sampleRate float64) (float64, error) {
	g, err := NewGoertzel(frequency, sampleRate)
	if err != nil {
		return 0, err
	}

	g.ProcessBlock(input)

	return g.Power(), nil
}

// GoertzelBank evaluates several target frequencies over a shared input stream,
// which is the efficient pattern for multi-tone detection such as DTMF or
// pilot-tone analysis. Each bin is an independent [Goertzel] sharing the same
// sample rate.
type GoertzelBank struct {
	bins []*Goertzel
}

// NewGoertzelBank creates a bank of single-bin analyzers, one per target
// frequency. At least one frequency is required and every frequency must
// satisfy 0 < f < sampleRate/2.
func NewGoertzelBank(frequencies []float64, sampleRate float64, opts ...GoertzelOption) (*GoertzelBank, error) {
	if len(frequencies) == 0 {
		return nil, fmt.Errorf("goertzel bank requires at least one frequency")
	}

	bins := make([]*Goertzel, len(frequencies))
	for idx, freq := range frequencies {
		analyzer, err := NewGoertzel(freq, sampleRate, opts...)
		if err != nil {
			return nil, fmt.Errorf("goertzel bank frequency %d: %w", idx, err)
		}

		bins[idx] = analyzer
	}

	return &GoertzelBank{bins: bins}, nil
}

// Reset clears the accumulated state of every bin.
func (b *GoertzelBank) Reset() {
	for _, g := range b.bins {
		g.Reset()
	}
}

// ProcessSample feeds one input sample into every bin.
func (b *GoertzelBank) ProcessSample(in float64) {
	for _, g := range b.bins {
		g.s0, g.s1 = goertzelStep(in, g.coef, g.s0, g.s1), g.s0
	}
}

// ProcessBlock feeds an entire block of samples into every bin.
//
// Bins are advanced four at a time in a single pass over buf, rather than one
// bin per pass: the recurrence is serial within a bin but the bins are
// independent, so a group of four reads the sample buffer once instead of four
// times and gives the processor four independent dependency chains to overlap.
// Each bin's state is hoisted into locals for the duration of the pass and
// written back afterwards. Bins left over after the last full group of four are
// run through [Goertzel.ProcessBlock].
//
// Every bin sees the identical recurrence with the identical operands in the
// identical order, so the result is bit-for-bit what per-bin passes produce.
func (b *GoertzelBank) ProcessBlock(buf []float64) {
	if len(buf) == 0 {
		return
	}

	idx := 0
	for ; idx+4 <= len(b.bins); idx += 4 {
		processBlock4(b.bins[idx], b.bins[idx+1], b.bins[idx+2], b.bins[idx+3], buf)
	}

	for ; idx < len(b.bins); idx++ {
		b.bins[idx].ProcessBlock(buf)
	}
}

// processBlock4 advances four independent Goertzel recurrences over buf in one
// pass. The four analyzers must be distinct; NewGoertzelBank is the only
// producer of GoertzelBank.bins and allocates one analyzer per frequency, so
// they always are.
func processBlock4(g0, g1, g2, g3 *Goertzel, buf []float64) {
	c0, a0, b0 := g0.coef, g0.s0, g0.s1
	c1, a1, b1 := g1.coef, g1.s0, g1.s1
	c2, a2, b2 := g2.coef, g2.s0, g2.s1
	c3, a3, b3 := g3.coef, g3.s0, g3.s1

	for _, x := range buf {
		a0, b0 = goertzelStep(x, c0, a0, b0), a0
		a1, b1 = goertzelStep(x, c1, a1, b1), a1
		a2, b2 = goertzelStep(x, c2, a2, b2), a2
		a3, b3 = goertzelStep(x, c3, a3, b3), a3
	}

	g0.s0, g0.s1 = a0, b0
	g1.s0, g1.s1 = a1, b1
	g2.s0, g2.s1 = a2, b2
	g3.s0, g3.s1 = a3, b3
}

// Powers writes the per-bin power into dst and returns it. If dst has
// sufficient capacity it is reused, keeping the call allocation-free; otherwise
// a new slice is allocated. Pass nil to always allocate.
func (b *GoertzelBank) Powers(dst []float64) []float64 {
	dst = resizeFloat64(dst, len(b.bins))
	for i, g := range b.bins {
		dst[i] = g.Power()
	}

	return dst
}

// Magnitudes writes the per-bin magnitude into dst and returns it, reusing dst
// when capacity allows (see [GoertzelBank.Powers]).
func (b *GoertzelBank) Magnitudes(dst []float64) []float64 {
	dst = resizeFloat64(dst, len(b.bins))
	for i, g := range b.bins {
		dst[i] = g.Magnitude()
	}

	return dst
}

// Bin returns the analyzer for the i-th frequency.
func (b *GoertzelBank) Bin(i int) *Goertzel { return b.bins[i] }

// Len returns the number of bins.
func (b *GoertzelBank) Len() int { return len(b.bins) }

// SampleRate returns the sample rate in Hz, read from the first bin so it always
// reflects the bins that are actually processing.
func (b *GoertzelBank) SampleRate() float64 { return b.bins[0].sampleRate }

// resizeFloat64 returns a slice of length n, reusing dst's backing array when
// its capacity is sufficient.
func resizeFloat64(dst []float64, n int) []float64 {
	if cap(dst) >= n {
		return dst[:n]
	}

	return make([]float64, n)
}
