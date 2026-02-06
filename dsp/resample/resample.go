package resample

import (
	"errors"
	"fmt"
	"math"
)

var (
	// ErrInvalidRatio indicates an invalid up/down ratio.
	ErrInvalidRatio = errors.New("resample: invalid ratio")
	// ErrInvalidRate indicates an invalid input/output sample rate.
	ErrInvalidRate = errors.New("resample: invalid sample rate")
)

// Quality controls default anti-aliasing filter settings.
type Quality int

const (
	// QualityFast prioritizes lower CPU usage.
	QualityFast Quality = iota
	// QualityBalanced is the default quality/performance trade-off.
	QualityBalanced
	// QualityBest prioritizes stopband attenuation and passband flatness.
	QualityBest
)

// Profile exposes default filter parameters for each quality mode.
type Profile struct {
	TapsPerPhase      int
	CutoffScale       float64
	KaiserBeta        float64
	NominalStopbandDB float64
}

// QualityProfile returns the default profile used by quality mode q.
func QualityProfile(q Quality) Profile {
	switch q {
	case QualityFast:
		return Profile{TapsPerPhase: 16, CutoffScale: 0.88, KaiserBeta: 5.0, NominalStopbandDB: 55}
	case QualityBest:
		return Profile{TapsPerPhase: 64, CutoffScale: 0.96, KaiserBeta: 9.0, NominalStopbandDB: 90}
	default:
		return Profile{TapsPerPhase: 32, CutoffScale: 0.92, KaiserBeta: 7.5, NominalStopbandDB: 75}
	}
}

type config struct {
	quality      Quality
	tapsPerPhase int
	cutoffScale  float64
	kaiserBeta   float64
	maxDen       int
}

// Option configures the resampler.
type Option func(*config)

// WithQuality selects a predefined anti-aliasing quality mode.
func WithQuality(q Quality) Option {
	return func(cfg *config) {
		cfg.quality = q
	}
}

// WithTapsPerPhase overrides taps per polyphase branch.
func WithTapsPerPhase(n int) Option {
	return func(cfg *config) {
		if n > 0 {
			cfg.tapsPerPhase = n
		}
	}
}

// WithCutoffScale overrides normalized cutoff scaling in range (0, 1].
// 1.0 equals the theoretical anti-aliasing cutoff.
func WithCutoffScale(v float64) Option {
	return func(cfg *config) {
		if v > 0 && v <= 1 {
			cfg.cutoffScale = v
		}
	}
}

// WithKaiserBeta overrides the Kaiser window beta parameter.
func WithKaiserBeta(beta float64) Option {
	return func(cfg *config) {
		if beta >= 0 {
			cfg.kaiserBeta = beta
		}
	}
}

// WithMaxDenominator caps denominator size for rate-ratio approximation.
func WithMaxDenominator(n int) Option {
	return func(cfg *config) {
		if n > 0 {
			cfg.maxDen = n
		}
	}
}

func defaultConfig() config {
	return config{
		quality: QualityBalanced,
		maxDen:  4096,
	}
}

func (c config) finalized() config {
	p := QualityProfile(c.quality)
	if c.tapsPerPhase <= 0 {
		c.tapsPerPhase = p.TapsPerPhase
	}
	if c.cutoffScale <= 0 || c.cutoffScale > 1 {
		c.cutoffScale = p.CutoffScale
	}
	if c.kaiserBeta < 0 {
		c.kaiserBeta = p.KaiserBeta
	}
	if c.kaiserBeta == 0 {
		c.kaiserBeta = p.KaiserBeta
	}
	if c.maxDen <= 0 {
		c.maxDen = 4096
	}
	return c
}

// Resampler performs rational sample-rate conversion using a polyphase FIR.
type Resampler struct {
	up   int
	down int

	quality Quality
	profile Profile

	taps       []float64
	phases     [][]float64
	maxPhaseLn int

	phase      int
	inputIndex int
	totalIn    int
	history    []float64
}

// NewRational creates a resampler for ratio up/down.
func NewRational(up, down int, opts ...Option) (*Resampler, error) {
	if up <= 0 || down <= 0 {
		return nil, ErrInvalidRatio
	}
	g := gcd(up, down)
	up /= g
	down /= g

	cfg := defaultConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	cfg = cfg.finalized()

	taps, phases, maxPhaseLn, err := designPolyphaseFIR(up, down, cfg)
	if err != nil {
		return nil, err
	}

	return &Resampler{
		up:         up,
		down:       down,
		quality:    cfg.quality,
		profile:    QualityProfile(cfg.quality),
		taps:       taps,
		phases:     phases,
		maxPhaseLn: maxPhaseLn,
	history:    make([]float64, 0, maxInt(0, maxPhaseLn-1)),
	}, nil
}

// NewForRates creates a resampler by approximating outRate/inRate as a ratio.
func NewForRates(inRate, outRate float64, opts ...Option) (*Resampler, error) {
	if inRate <= 0 || outRate <= 0 || math.IsNaN(inRate) || math.IsNaN(outRate) {
		return nil, ErrInvalidRate
	}
	cfg := defaultConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	cfg = cfg.finalized()

	up, down := approximateRatio(outRate/inRate, cfg.maxDen)
	return NewRational(up, down, opts...)
}

// Upsample2x is a convenience wrapper for 2:1 conversion.
func Upsample2x(input []float64, opts ...Option) ([]float64, error) {
	r, err := NewRational(2, 1, opts...)
	if err != nil {
		return nil, err
	}
	return r.Process(input), nil
}

// Downsample2x is a convenience wrapper for 1:2 conversion.
func Downsample2x(input []float64, opts ...Option) ([]float64, error) {
	r, err := NewRational(1, 2, opts...)
	if err != nil {
		return nil, err
	}
	return r.Process(input), nil
}

// Resample converts input using ratio up/down as a one-shot helper.
func Resample(input []float64, up, down int, opts ...Option) ([]float64, error) {
	r, err := NewRational(up, down, opts...)
	if err != nil {
		return nil, err
	}
	return r.Process(input), nil
}

// Reset clears internal filter state.
func (r *Resampler) Reset() {
	r.phase = 0
	r.inputIndex = 0
	r.totalIn = 0
	r.history = r.history[:0]
}

// Process converts an input block and preserves internal state for streaming.
func (r *Resampler) Process(input []float64) []float64 {
	if len(input) == 0 {
		return nil
	}

	nOut := r.PredictOutputLen(len(input))
	out := make([]float64, 0, nOut)

	work := make([]float64, len(r.history)+len(input))
	copy(work, r.history)
	copy(work[len(r.history):], input)

	baseIndex := r.totalIn - len(r.history)
	lastAvail := r.totalIn + len(input) - 1

	for r.inputIndex <= lastAvail {
		taps := r.phases[r.phase]
		var y float64
		for k, c := range taps {
			idx := r.inputIndex - k
			if idx < baseIndex || idx > lastAvail {
				continue
			}
			y += c * work[idx-baseIndex]
		}
		out = append(out, y)

		r.phase += r.down
		r.inputIndex += r.phase / r.up
		r.phase %= r.up
	}

	r.totalIn += len(input)
	keep := max(0, r.maxPhaseLn-1)
	if keep > len(work) {
		keep = len(work)
	}
	r.history = append(r.history[:0], work[len(work)-keep:]...)

	return out
}

// PredictOutputLen estimates output samples generated for the next Process call.
func (r *Resampler) PredictOutputLen(inputLen int) int {
	if inputLen <= 0 {
		return 0
	}
	lastAvail := r.totalIn + inputLen - 1
	i := r.inputIndex
	phase := r.phase
	count := 0
	for i <= lastAvail {
		count++
		phase += r.down
		i += phase / r.up
		phase %= r.up
	}
	return count
}

// Ratio returns reduced up/down conversion factors.
func (r *Resampler) Ratio() (up, down int) {
	return r.up, r.down
}

// Quality returns the configured quality mode.
func (r *Resampler) Quality() Quality {
	return r.quality
}

// TapsPerPhase returns taps in each polyphase branch for phase 0.
func (r *Resampler) TapsPerPhase() int {
	if len(r.phases) == 0 {
		return 0
	}
	return len(r.phases[0])
}

// Prototype returns a copy of the underlying prototype FIR taps.
func (r *Resampler) Prototype() []float64 {
	out := make([]float64, len(r.taps))
	copy(out, r.taps)
	return out
}

func designPolyphaseFIR(up, down int, cfg config) ([]float64, [][]float64, int, error) {
	if up <= 0 || down <= 0 {
		return nil, nil, 0, ErrInvalidRatio
	}
	if cfg.tapsPerPhase <= 0 {
		return nil, nil, 0, fmt.Errorf("resample: taps per phase must be > 0")
	}
	if cfg.cutoffScale <= 0 || cfg.cutoffScale > 1 {
		return nil, nil, 0, fmt.Errorf("resample: cutoff scale must be in (0,1]")
	}

	nTaps := cfg.tapsPerPhase * up
	fc := (0.5 / float64(maxInt(up, down))) * cfg.cutoffScale
	if fc <= 0 || fc >= 0.5 {
		return nil, nil, 0, fmt.Errorf("resample: invalid cutoff %.6f", fc)
	}

	taps := make([]float64, nTaps)
	center := 0.5 * float64(nTaps-1)
	for n := range nTaps {
		t := float64(n) - center
		h := 2 * fc * sinc(2*fc*t) * kaiserWindow(n, nTaps, cfg.kaiserBeta)
		taps[n] = h
	}

	var sum float64
	for _, v := range taps {
		sum += v
	}
	if sum == 0 {
		return nil, nil, 0, fmt.Errorf("resample: designed zero-sum filter")
	}
	scale := float64(up) / sum
	for i := range taps {
		taps[i] *= scale
	}

	phases := make([][]float64, up)
	maxPhaseLn := 0
	for p := range up {
		phase := make([]float64, 0, (nTaps-p+up-1)/up)
		for i := p; i < nTaps; i += up {
			phase = append(phase, taps[i])
		}
		if len(phase) > maxPhaseLn {
			maxPhaseLn = len(phase)
		}
		phases[p] = phase
	}

	return taps, phases, maxPhaseLn, nil
}

func approximateRatio(v float64, maxDen int) (num, den int) {
	if maxDen <= 0 {
		maxDen = 4096
	}
	if v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 1, 1
	}

	a0 := math.Floor(v)
	p0, q0 := 1.0, 0.0
	p1, q1 := a0, 1.0
	x := v

	for {
		frac := x - math.Floor(x)
		if frac == 0 {
			break
		}
		x = 1 / frac
		a := math.Floor(x)
		p2 := a*p1 + p0
		q2 := a*q1 + q0
		if q2 > float64(maxDen) {
			break
		}
		p0, q0 = p1, q1
		p1, q1 = p2, q2
	}

	num = int(math.Round(p1))
	den = int(math.Round(q1))
	if den <= 0 {
		return 1, 1
	}
	g := gcd(num, den)
	return num / g, den / g
}

func gcd(a, b int) int {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	for b != 0 {
		a, b = b, a%b
	}
	if a == 0 {
		return 1
	}
	return a
}

func sinc(x float64) float64 {
	if math.Abs(x) < 1e-12 {
		return 1
	}
	pix := math.Pi * x
	return math.Sin(pix) / pix
}

func kaiserWindow(i, n int, beta float64) float64 {
	if n <= 1 || beta == 0 {
		return 1
	}
	t := 2*float64(i)/float64(n-1) - 1
	a := math.Sqrt(math.Max(0, 1-t*t))
	return i0(beta*a) / i0(beta)
}

func i0(x float64) float64 {
	// Power series approximation.
	sum := 1.0
	term := 1.0
	x2 := (x * x) / 4
	for k := 1; k < 64; k++ {
		term *= x2 / float64(k*k)
		sum += term
		if term < 1e-16*sum {
			break
		}
	}
	return sum
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
