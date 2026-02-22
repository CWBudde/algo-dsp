package window

import (
	"math"

	"github.com/cwbudde/algo-vecmath"
)

// Type identifies a window function.
type Type int

const (
	TypeRectangular Type = iota
	TypeHann
	TypeHamming
	TypeBlackman
	TypeBlackmanHarris4Term
	TypeFlatTop
	TypeKaiser
	TypeTukey
	TypeTriangle
	TypeCosine
	TypeWelch
	TypeLanczos
	TypeGauss
	TypeExactBlackman
	TypeBlackmanHarris3Term
	TypeBlackmanNuttall
	TypeNuttallCTD
	TypeNuttallCFD
	TypeLawrey5Term
	TypeLawrey6Term
	TypeBurgessOptimized59dB
	TypeBurgessOptimized71dB
	TypeAlbrecht2Term
	TypeAlbrecht3Term
	TypeAlbrecht4Term
	TypeAlbrecht5Term
	TypeAlbrecht6Term
	TypeAlbrecht7Term
	TypeAlbrecht8Term
	TypeAlbrecht9Term
	TypeAlbrecht10Term
	TypeAlbrecht11Term
	TypeFreeCosine
)

// Slope controls which edge(s) of the window are tapered.
type Slope int

const (
	SlopeSymmetric Slope = iota
	SlopeLeft
	SlopeRight
)

// Metadata holds spectral properties of a window type.
type Metadata struct {
	Name                string
	ENBW                float64
	HighestSidelobe     float64
	CoherentGain        float64
	CoherentGainSquared float64
}

// Option configures window generation.
type Option func(*config)

type config struct {
	alpha        float64
	periodic     bool
	slope        Slope
	dcRemoval    bool
	invert       bool
	bartlett     bool
	customCoeffs []float64
}

func defaultConfig() config {
	return config{
		alpha: 1,
		slope: SlopeSymmetric,
	}
}

// WithAlpha configures alpha/beta parameters for parametric windows.
func WithAlpha(v float64) Option {
	return func(c *config) {
		if v >= 0 {
			c.alpha = v
		}
	}
}

// WithPeriodic configures periodic form (FFT framing) instead of symmetric form.
func WithPeriodic() Option {
	return func(c *config) {
		c.periodic = true
	}
}

// WithSlope configures edge tapering mode.
func WithSlope(s Slope) Option {
	return func(c *config) {
		c.slope = s
	}
}

// WithDCRemoval subtracts mean after window generation.
func WithDCRemoval() Option {
	return func(c *config) {
		c.dcRemoval = true
	}
}

// WithInvert inverts coefficients (1 - w[n]).
func WithInvert() Option {
	return func(c *config) {
		c.invert = true
	}
}

// WithBartlett enables the half-sample-shift Bartlett variant for Triangle.
func WithBartlett() Option {
	return func(c *config) {
		c.bartlett = true
	}
}

// WithCustomCoeffs sets cosine-term coefficients for FreeCosine.
func WithCustomCoeffs(coeffs []float64) Option {
	copyCoeffs := append([]float64(nil), coeffs...)

	return func(c *config) {
		c.customCoeffs = copyCoeffs
	}
}

// Generate returns window coefficients of the given length.
func Generate(t Type, length int, opts ...Option) []float64 {
	if length <= 0 {
		return nil
	}

	cfg := defaultConfig()

	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	out := make([]float64, length)
	for i := range out {
		x := samplePosition(i, length, cfg.periodic)
		out[i] = evalWindow(t, x, cfg)
	}

	postProcess(out, cfg)

	return out
}

// Apply multiplies buf in-place by the selected window.
func Apply(t Type, buf []float64, opts ...Option) {
	if len(buf) == 0 {
		return
	}

	coeffs := Generate(t, len(buf), opts...)
	if len(coeffs) != len(buf) {
		return
	}

	vecmath.MulBlockInPlace(buf, coeffs)
}

// Info returns static metadata for a window type.
func Info(t Type) Metadata {
	if m, ok := metadataByType[t]; ok {
		return m
	}

	return Metadata{}
}

// Hann returns Hann window coefficients.
func Hann(size int, opts ...Option) ([]float64, error) {
	return Generate(TypeHann, size, opts...), validateLength(size)
}

// Hamming returns Hamming window coefficients.
func Hamming(size int, opts ...Option) ([]float64, error) {
	return Generate(TypeHamming, size, opts...), validateLength(size)
}

// Blackman returns Blackman window coefficients.
func Blackman(size int, opts ...Option) ([]float64, error) {
	return Generate(TypeBlackman, size, opts...), validateLength(size)
}

// FlatTop returns 5-term flat-top window coefficients.
func FlatTop(size int, opts ...Option) ([]float64, error) {
	return Generate(TypeFlatTop, size, opts...), validateLength(size)
}

// Kaiser returns Kaiser window coefficients.
func Kaiser(size int, beta float64, opts ...Option) ([]float64, error) {
	if size <= 0 || beta < 0 {
		return nil, validateKaiser(size, beta)
	}

	return Generate(TypeKaiser, size, append(opts, WithAlpha(beta))...), nil
}

// Tukey returns Tukey window coefficients.
func Tukey(size int, alpha float64, opts ...Option) ([]float64, error) {
	if size <= 0 || alpha < 0 || alpha > 1 {
		return nil, validateTukey(size, alpha)
	}

	return Generate(TypeTukey, size, append(opts, WithAlpha(alpha))...), nil
}

// Gaussian returns Gaussian window coefficients.
func Gaussian(size int, alpha float64, opts ...Option) ([]float64, error) {
	if size <= 0 || alpha <= 0 {
		return nil, validateGauss(size, alpha)
	}

	return Generate(TypeGauss, size, append(opts, WithAlpha(alpha))...), nil
}

// Lanczos returns Lanczos window coefficients.
func Lanczos(size int, opts ...Option) ([]float64, error) {
	return Generate(TypeLanczos, size, opts...), validateLength(size)
}

// EquivalentNoiseBandwidth returns the ENBW in bins for a window.
func EquivalentNoiseBandwidth(coeffs []float64) (float64, error) {
	if len(coeffs) == 0 {
		return 0, errEmptyCoeffs
	}

	sum := 0.0
	sumSquares := 0.0

	for _, c := range coeffs {
		sum += c
		sumSquares += c * c
	}

	if sum == 0 {
		return 0, errZeroCoherentGain
	}

	return float64(len(coeffs)) * sumSquares / (sum * sum), nil
}

// ApplyCoefficients multiplies samples with coefficients and returns a new slice.
func ApplyCoefficients(samples, coeffs []float64) ([]float64, error) {
	if len(samples) != len(coeffs) {
		return nil, errMismatchedLength
	}

	out := make([]float64, len(samples))
	vecmath.MulBlock(out, samples, coeffs)

	return out, nil
}

// ApplyCoefficientsInPlace multiplies samples with coefficients in place.
func ApplyCoefficientsInPlace(samples, coeffs []float64) error {
	if len(samples) != len(coeffs) {
		return errMismatchedLength
	}

	vecmath.MulBlockInPlace(samples, coeffs)

	return nil
}

func evalWindow(t Type, x float64, cfg config) float64 {
	switch cfg.slope {
	case SlopeLeft:
		if x >= 0.5 {
			return 1
		}

		x *= 2
	case SlopeRight:
		if x <= 0.5 {
			return 1
		}

		x = 2*x - 1
	}

	if x < 0 {
		x = 0
	}

	if x > 1 {
		x = 1
	}

	switch t {
	case TypeRectangular:
		return 1
	case TypeHann:
		return cosineFromCoeffs(x, hannCoeffs)
	case TypeHamming:
		return cosineFromCoeffs(x, hammingCoeffs)
	case TypeBlackman:
		return cosineFromCoeffs(x, blackmanCoeffs)
	case TypeBlackmanHarris4Term:
		return cosineFromCoeffs(x, blackmanHarris4Coeffs)
	case TypeFlatTop:
		return cosineFromCoeffs(x, flatTopCoeffs)
	case TypeKaiser:
		return kaiserAt(x, cfg.alpha)
	case TypeTukey:
		return tukeyAt(x, cfg.alpha)
	case TypeTriangle:
		return triangleAt(x, cfg.bartlett)
	case TypeCosine:
		return math.Sin(math.Pi * x)
	case TypeWelch:
		d := x - 0.5
		return 1 - 4*d*d
	case TypeLanczos:
		return sinc((2*x - 1) * cfg.alpha)
	case TypeGauss:
		v := (2*x - 1) * cfg.alpha
		return math.Exp(-math.Ln2 * v * v)
	case TypeExactBlackman:
		return cosineFromCoeffs(x, exactBlackmanCoeffs)
	case TypeBlackmanHarris3Term:
		return cosineFromCoeffs(x, blackmanHarris3Coeffs)
	case TypeBlackmanNuttall:
		return cosineFromCoeffs(x, blackmanNuttallCoeffs)
	case TypeNuttallCTD:
		return cosineFromCoeffs(x, nuttallCTDCoeffs)
	case TypeNuttallCFD:
		return cosineFromCoeffs(x, nuttallCFDCoeffs)
	case TypeLawrey5Term:
		return cosineFromCoeffs(x, lawrey5Coeffs)
	case TypeLawrey6Term:
		return cosineFromCoeffs(x, lawrey6Coeffs)
	case TypeBurgessOptimized59dB:
		return cosineFromCoeffs(x, burgess59Coeffs)
	case TypeBurgessOptimized71dB:
		return cosineFromCoeffs(x, burgess71Coeffs)
	case TypeAlbrecht2Term:
		return cosineFromCoeffs(x, albrecht2Coeffs)
	case TypeAlbrecht3Term:
		return cosineFromCoeffs(x, albrecht3Coeffs)
	case TypeAlbrecht4Term:
		return cosineFromCoeffs(x, albrecht4Coeffs)
	case TypeAlbrecht5Term:
		return cosineFromCoeffs(x, albrecht5Coeffs)
	case TypeAlbrecht6Term:
		return cosineFromCoeffs(x, albrecht6Coeffs)
	case TypeAlbrecht7Term:
		return cosineFromCoeffs(x, albrecht7Coeffs)
	case TypeAlbrecht8Term:
		return cosineFromCoeffs(x, albrecht8Coeffs)
	case TypeAlbrecht9Term:
		return cosineFromCoeffs(x, albrecht9Coeffs)
	case TypeAlbrecht10Term:
		return cosineFromCoeffs(x, albrecht10Coeffs)
	case TypeAlbrecht11Term:
		return cosineFromCoeffs(x, albrecht11Coeffs)
	case TypeFreeCosine:
		if len(cfg.customCoeffs) == 0 {
			return 1
		}

		return cosineFromCoeffs(x, cfg.customCoeffs)
	default:
		return 1
	}
}

func postProcess(coeffs []float64, cfg config) {
	if cfg.invert {
		for i := range coeffs {
			coeffs[i] = 1 - coeffs[i]
		}
	}

	if cfg.dcRemoval {
		sum := 0.0
		for _, v := range coeffs {
			sum += v
		}

		mean := sum / float64(len(coeffs))
		for i := range coeffs {
			coeffs[i] -= mean
		}
	}
}

func cosineFromCoeffs(x float64, coeffs []float64) float64 {
	phase := 2 * math.Pi * x

	sum := 0.0
	for k, c := range coeffs {
		sum += c * math.Cos(float64(k)*phase)
	}

	return sum
}

func samplePosition(n, size int, periodic bool) float64 {
	if size <= 1 {
		return 0
	}

	den := float64(size - 1)
	if periodic {
		den = float64(size)
	}

	return float64(n) / den
}

func kaiserAt(x, beta float64) float64 {
	if beta <= 0 {
		return 1
	}

	r := 2*x - 1
	term := math.Sqrt(math.Max(0, 1-r*r))

	return besselI0(beta*term) / besselI0(beta)
}

func tukeyAt(x, alpha float64) float64 {
	if alpha <= 0 {
		return 1
	}

	if alpha >= 1 {
		return cosineFromCoeffs(x, hannCoeffs)
	}

	a := alpha / 2
	switch {
	case x < a:
		return 0.5 * (1 + math.Cos(math.Pi*(2*x/alpha-1)))
	case x <= 1-a:
		return 1
	default:
		return 0.5 * (1 + math.Cos(math.Pi*(2*x/alpha-2/alpha+1)))
	}
}

func triangleAt(x float64, bartlett bool) float64 {
	if bartlett {
		return 1 - math.Abs(2*x-1)
	}

	if x <= 0.5 {
		return 2 * x
	}

	return 2 * (1 - x)
}

func sinc(x float64) float64 {
	if x == 0 {
		return 1
	}

	px := math.Pi * x

	return math.Sin(px) / px
}

// besselI0 returns a numerical approximation of the modified Bessel function I0.
func besselI0(x float64) float64 {
	ax := math.Abs(x)
	if ax < 3.75 {
		y := x / 3.75
		y *= y

		return 1.0 + y*(3.5156229+y*(3.0899424+y*(1.2067492+y*(0.2659732+y*(0.0360768+y*0.0045813)))))
	}

	y := 3.75 / ax

	return (math.Exp(ax) / math.Sqrt(ax)) *
		(0.39894228 + y*(0.01328592+y*(0.00225319+y*(-0.00157565+y*(0.00916281+y*(-0.02057706+y*(0.02635537+y*(-0.01647633+y*0.00392377))))))))
}
