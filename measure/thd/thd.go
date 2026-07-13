package thd

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/window"
	algofft "github.com/cwbudde/algo-fft"
)

const (
	defaultRangeLowerHz = 20.0
	defaultRangeUpperHz = 20000.0
	defaultRubNBuzz     = 10
)

// Config holds THD calculation parameters.
type Config struct {
	SampleRate      float64
	FFTSize         int
	FundamentalFreq float64
	RangeLowerFreq  float64
	RangeUpperFreq  float64
	CaptureBins     int
	MaxHarmonics    int
	RubNBuzzStart   int
	WindowType      window.Type
}

// Result holds THD measurement results.
//
// All ratios are amplitude ratios computed in the power domain per the
// IEEE root-sum-square (RSS) definitions: a metric that aggregates several
// spectral components is sqrt(sum of component powers) / fundamental
// amplitude. Two harmonics of 1%% each therefore yield THD = 1.414%%,
// not 2%%.
type Result struct {
	FundamentalFreq float64
	// FundamentalLevel is the RSS amplitude of the fundamental, including
	// the configured capture window around its bin.
	FundamentalLevel float64
	// THD is sqrt(sum of harmonic powers) / fundamental amplitude (THD_R).
	THD float64
	// THDN is sqrt(total in-range power minus fundamental power) /
	// fundamental amplitude.
	THDN    float64
	THD_dB  float64
	THDN_dB float64
	// OddHD and EvenHD are RSS amplitude ratios over odd/even harmonics.
	OddHD  float64
	EvenHD float64
	// Noise is the RSS amplitude ratio of in-range energy that is neither
	// the fundamental nor a captured harmonic.
	Noise float64
	// RubNBuzz is the RSS amplitude ratio over harmonics k >= RubNBuzzStart.
	RubNBuzz float64
	// Harmonics holds per-harmonic amplitude ratios |H_k|/|H_1|.
	Harmonics []float64
	// SINAD is -THDN_dB: 10*log10(fundamental power / (noise+distortion power)).
	SINAD float64
}

// Calculator performs THD analysis on frequency-domain data.
type Calculator struct {
	cfg Config
}

// NewCalculator creates a new THD calculator.
func NewCalculator(cfg Config) *Calculator {
	cfg = normalizeConfig(cfg)
	return &Calculator{cfg: cfg}
}

// Analyze is a one-shot THD analysis for a complex spectrum.
func Analyze(spectrum []complex128, cfg Config) Result {
	return NewCalculator(cfg).Calculate(spectrum)
}

// AnalyzeSignal performs one-shot THD analysis from a time-domain signal.
// It applies the configured window, performs an FFT, and evaluates THD metrics.
func AnalyzeSignal(signal []float64, cfg Config) Result {
	return NewCalculator(cfg).AnalyzeSignal(signal)
}

// Calculate computes THD metrics from a complex spectrum.
func (c *Calculator) Calculate(spectrum []complex128) Result {
	if len(spectrum) == 0 {
		return Result{}
	}

	binCount := len(spectrum)/2 + 1
	if binCount <= 1 {
		return Result{}
	}

	magSquared := make([]float64, binCount)
	for i := range magSquared {
		x := spectrum[i]
		magSquared[i] = real(x)*real(x) + imag(x)*imag(x)
	}

	cfg := c.cfg
	if cfg.FFTSize <= 0 {
		cfg.FFTSize = len(spectrum)
	}

	if cfg.SampleRate <= 0 {
		cfg.SampleRate = float64(cfg.FFTSize)
	}

	calc := Calculator{cfg: cfg}

	return calc.CalculateFromMagnitude(magSquared)
}

// AnalyzeSignal computes THD metrics from a real-valued time-domain signal.
func (c *Calculator) AnalyzeSignal(signal []float64) Result {
	if len(signal) == 0 {
		return Result{}
	}

	cfg := c.cfg

	fftSize := cfg.FFTSize
	if fftSize <= 0 {
		fftSize = nextPowerOf2(len(signal))
	}

	if fftSize <= 1 {
		return Result{}
	}

	winType := cfg.WindowType
	if winType == 0 {
		winType = window.TypeHann
	}

	// Analyze at most fftSize samples: windowing is applied to the segment
	// that actually enters the FFT.
	segment := signal
	if len(segment) > fftSize {
		segment = segment[:fftSize]
	}

	coeffs := window.Generate(winType, len(segment))

	inData := make([]complex128, fftSize)

	for i := range segment {
		w := 1.0
		if len(coeffs) == len(segment) {
			w = coeffs[i]
		}

		inData[i] = complex(segment[i]*w, 0)
	}

	plan, err := algofft.NewPlan64(fftSize)
	if err != nil {
		return Result{}
	}

	out := make([]complex128, fftSize)
	if err := plan.Forward(out, inData); err != nil {
		return Result{}
	}

	cfg.FFTSize = fftSize
	if cfg.SampleRate <= 0 {
		cfg.SampleRate = float64(fftSize)
	}

	calc := NewCalculator(cfg)

	return calc.Calculate(out)
}

// CalculateFromMagnitude computes THD metrics from a squared-magnitude spectrum.
// magSquared is expected to contain non-negative-frequency bins [0..Nyquist].
func (c *Calculator) CalculateFromMagnitude(magSquared []float64) Result {
	if len(magSquared) <= 1 {
		return Result{}
	}

	cfg := c.cfg
	if cfg.FFTSize <= 0 {
		cfg.FFTSize = 2 * (len(magSquared) - 1)
	}

	if cfg.FFTSize <= 1 {
		return Result{}
	}

	if cfg.SampleRate <= 0 {
		cfg.SampleRate = float64(cfg.FFTSize)
	}

	binCount := len(magSquared)
	maxBin := binCount - 1

	binHz := cfg.SampleRate / float64(cfg.FFTSize)
	if binHz <= 0 {
		return Result{}
	}

	lowerBin := clampInt(int(math.Round(cfg.RangeLowerFreq/binHz)), 1, maxBin)
	upperBin := clampInt(int(math.Round(cfg.RangeUpperFreq/binHz)), lowerBin, maxBin)

	fundamentalBin := c.findFundamentalBin(magSquared, lowerBin, upperBin, binHz)
	if fundamentalBin < 1 || fundamentalBin > maxBin {
		return Result{}
	}

	captureBins := cfg.CaptureBins
	if captureBins <= 0 {
		captureBins = c.autoCaptureBins()
	}

	if captureBins*2 > fundamentalBin {
		captureBins = fundamentalBin / 2
	}

	// All aggregation happens in the power domain (sums of squared
	// magnitudes); amplitude ratios are formed at the end via square roots.
	// Summing linear magnitudes instead would inflate wideband terms such
	// as the noise floor by roughly sqrt(bin count) — tens of dB at
	// realistic FFT sizes.
	fundamentalPower := getBinPower(magSquared, fundamentalBin, captureBins)
	if fundamentalPower <= 0 {
		return Result{
			FundamentalFreq: float64(fundamentalBin) * binHz,
		}
	}

	thdPower := 0.0
	oddPower := 0.0
	evenPower := 0.0
	rubPower := 0.0
	harmonics := make([]float64, 0, 8)

	harmonicCount := 0
	for k := 2; ; k++ {
		if cfg.MaxHarmonics > 0 && harmonicCount >= cfg.MaxHarmonics {
			break
		}

		bin := k * fundamentalBin
		if bin > upperBin || bin > maxBin {
			break
		}

		if bin < lowerBin {
			continue
		}

		power := getBinPower(magSquared, bin, captureBins)

		thdPower += power
		if k%2 == 0 {
			evenPower += power
		} else {
			oddPower += power
		}

		if k >= cfg.RubNBuzzStart {
			rubPower += power
		}

		if power > 0 {
			harmonics = append(harmonics, math.Sqrt(power/fundamentalPower))
		}

		harmonicCount++
	}

	totalPower := 0.0

	for i := lowerBin; i <= upperBin; i++ {
		if magSquared[i] > 0 {
			totalPower += magSquared[i]
		}
	}

	thdnPower := totalPower - fundamentalPower
	if thdnPower < 0 {
		thdnPower = 0
	}

	noisePower := thdnPower - thdPower
	if noisePower < 0 {
		noisePower = 0
	}

	thd := math.Sqrt(thdPower / fundamentalPower)
	thdn := math.Sqrt(thdnPower / fundamentalPower)
	odd := math.Sqrt(oddPower / fundamentalPower)
	even := math.Sqrt(evenPower / fundamentalPower)
	noise := math.Sqrt(noisePower / fundamentalPower)
	rub := math.Sqrt(rubPower / fundamentalPower)

	sinad := math.Inf(1)
	if thdn > 0 {
		sinad = 20 * math.Log10(1/thdn)
	}

	return Result{
		FundamentalFreq:  float64(fundamentalBin) * binHz,
		FundamentalLevel: math.Sqrt(fundamentalPower),
		THD:              thd,
		THDN:             thdn,
		THD_dB:           ratioToDB(thd),
		THDN_dB:          ratioToDB(thdn),
		OddHD:            odd,
		EvenHD:           even,
		Noise:            noise,
		RubNBuzz:         rub,
		Harmonics:        harmonics,
		SINAD:            sinad,
	}
}

func (c *Calculator) findFundamentalBin(magSquared []float64, lowerBin, upperBin int, binHz float64) int {
	if c.cfg.FundamentalFreq > 0 {
		bin := int(math.Round(c.cfg.FundamentalFreq / binHz))
		return clampInt(bin, lowerBin, upperBin)
	}

	bestBin := lowerBin
	bestVal := -1.0

	for i := lowerBin; i <= upperBin; i++ {
		v := magSquared[i]
		if v > bestVal {
			bestVal = v
			bestBin = i
		}
	}

	return bestBin
}

func (c *Calculator) autoCaptureBins() int {
	if v, ok := firstMinimumBinsByType(c.cfg.WindowType); ok {
		return int(math.Round(v))
	}

	// Fallback for less common windows: estimate on a modest size to keep
	// auto-capture setup bounded.
	n := c.cfg.FFTSize
	if n <= 0 {
		return 0
	}

	if n > 4096 {
		n = 4096
	}

	coeffs := window.Generate(c.cfg.WindowType, n)

	analysis := window.Analyze(coeffs)
	if analysis.FirstMinimumBins <= 0 || math.IsNaN(analysis.FirstMinimumBins) {
		return 0
	}

	return int(math.Round(analysis.FirstMinimumBins))
}

func firstMinimumBinsByType(t window.Type) (float64, bool) {
	switch t {
	case window.TypeRectangular:
		return 1, true
	case window.TypeHann, window.TypeHamming, window.TypeTriangle, window.TypeCosine, window.TypeWelch:
		return 2, true
	case window.TypeBlackman, window.TypeExactBlackman, window.TypeKaiser:
		return 3, true
	case window.TypeBlackmanHarris3Term:
		return 3, true
	case window.TypeBlackmanHarris4Term, window.TypeBlackmanNuttall, window.TypeNuttallCTD, window.TypeNuttallCFD:
		return 4, true
	case window.TypeFlatTop:
		return 5, true
	default:
		return 0, false
	}
}

func normalizeConfig(cfg Config) Config {
	if cfg.RangeLowerFreq <= 0 {
		cfg.RangeLowerFreq = defaultRangeLowerHz
	}

	if cfg.RangeUpperFreq <= 0 {
		cfg.RangeUpperFreq = defaultRangeUpperHz
	}

	if cfg.RangeUpperFreq < cfg.RangeLowerFreq {
		cfg.RangeUpperFreq = cfg.RangeLowerFreq
	}

	if cfg.RubNBuzzStart < 1 {
		cfg.RubNBuzzStart = defaultRubNBuzz
	}

	if cfg.WindowType == 0 {
		cfg.WindowType = window.TypeHann
	}

	if cfg.CaptureBins < 0 {
		cfg.CaptureBins = 0
	}

	if cfg.MaxHarmonics < 0 {
		cfg.MaxHarmonics = 0
	}

	return cfg
}

// getBinPower returns the total power (sum of squared magnitudes) in the
// capture window centered on bin.
func getBinPower(magSquared []float64, bin, captureBins int) float64 {
	if bin < 0 || bin >= len(magSquared) {
		return 0
	}

	if captureBins <= 0 {
		return positive(magSquared[bin])
	}

	loBin := bin - captureBins
	if loBin < 0 {
		loBin = 0
	}

	hiBin := bin + captureBins
	if hiBin >= len(magSquared) {
		hiBin = len(magSquared) - 1
	}

	sum := 0.0
	for i := loBin; i <= hiBin; i++ {
		sum += positive(magSquared[i])
	}

	return sum
}

func positive(v float64) float64 {
	if v <= 0 {
		return 0
	}

	return v
}

func ratioToDB(v float64) float64 {
	if v <= 0 {
		return math.Inf(-1)
	}

	return 20 * math.Log10(v)
}

func clampInt(val, lo, hi int) int {
	if val < lo {
		return lo
	}

	if val > hi {
		return hi
	}

	return val
}

func nextPowerOf2(n int) int {
	if n <= 1 {
		return 1
	}

	p := 1
	for p < n {
		p <<= 1
	}

	return p
}
