package graphiceq

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
	"github.com/cwbudde/algo-dsp/dsp/window"
)

const (
	// defaultQ is the shelf quality factor. It is deliberately above the
	// Butterworth value 1/sqrt(2): the target interpolates linearly in log
	// frequency, so it is steeper between two octave centres than a
	// maximally flat shelf, and the slight resonance at Q = 1 tracks it more
	// closely than a shelf with no overshoot does.
	defaultQ = 1.0

	// lengthCycles is how many periods of the lowest FIR-shaped band the
	// default FIR length spans. Two periods resolve an octave band without
	// making the taper dominate the response.
	lengthCycles = 2.0

	minimumLength   = 3
	magnitudeFloor  = 1e-12
	measurementSpan = 0.5
)

// DefaultLength reports the FIR length [Design] uses when Config.Length is
// zero.
//
// The length spans two periods of the lowest band the FIR still has to shape,
// which is Bands[IIRBands]. Octave centres double, so moving one more band into
// the IIR part halves the result — this is where the latency saving comes from.
// It returns zero when the configuration leaves no band for the FIR.
func DefaultLength(cfg Config) int {
	if cfg.SampleRate <= 0 || cfg.IIRBands < 0 ||
		cfg.IIRBands >= len(cfg.Bands) {
		return 0
	}

	lowest := cfg.Bands[cfg.IIRBands].Frequency
	if lowest <= 0 {
		return 0
	}

	length := int(math.Ceil(lengthCycles * cfg.SampleRate / lowest))
	length = max(length, minimumLength)

	if length%2 == 0 {
		length++
	}

	return length
}

// resolved holds a validated configuration with every default filled in.
type resolved struct {
	cfg     Config
	length  int
	fftSize int
	bins    int
	q       float64
}

// Design builds the hybrid equaliser described by cfg.
//
// The lowest cfg.IIRBands bands become a biquad cascade and the rest become one
// linear-phase FIR designed for whatever the cascade left over. With
// cfg.IIRBands zero the result is the all-FIR reference design, which is the
// baseline the hybrid is meant to be compared against.
func Design(cfg Config) (Result, error) {
	state, err := resolve(cfg)
	if err != nil {
		return Result{}, err
	}

	sections := designSections(state)
	frequencies := state.frequencies()
	targetDB := targetCurve(state.cfg.Bands, frequencies)
	sectionDB := cascadeMagnitudeDB(sections, frequencies, state.cfg.SampleRate)

	taps, firDB, err := designFIR(state, targetDB, sectionDB)
	if err != nil {
		return Result{}, err
	}

	totalDB := make([]float64, state.bins)
	for k := range totalDB {
		totalDB[k] = sectionDB[k] + firDB[k]
	}

	return Result{
		Sections: sections,
		Taps:     taps,
		Latency:  (len(taps) - 1) / 2,
		Metrics: measure(
			state,
			sections,
			taps,
			frequencies,
			targetDB,
			totalDB,
		),
	}, nil
}

func resolve(cfg Config) (resolved, error) {
	if cfg.SampleRate <= 0 || math.IsNaN(cfg.SampleRate) ||
		math.IsInf(cfg.SampleRate, 0) {
		return resolved{}, fmt.Errorf(
			"%w: got %v",
			ErrInvalidSampleRate,
			cfg.SampleRate,
		)
	}

	if len(cfg.Bands) == 0 {
		return resolved{}, ErrNoBands
	}

	if err := validateBands(cfg); err != nil {
		return resolved{}, err
	}

	if cfg.IIRBands < 0 || cfg.IIRBands >= len(cfg.Bands) {
		return resolved{}, fmt.Errorf(
			"%w: %d of %d bands requested, at least one must stay in the FIR",
			ErrInvalidSplit,
			cfg.IIRBands,
			len(cfg.Bands),
		)
	}

	if cfg.Q < 0 {
		return resolved{}, fmt.Errorf("%w: got %v", ErrInvalidQ, cfg.Q)
	}

	if cfg.WindowAlpha < 0 {
		return resolved{}, fmt.Errorf(
			"%w: got %v",
			ErrInvalidWindowAlpha,
			cfg.WindowAlpha,
		)
	}

	length, err := resolveLength(cfg)
	if err != nil {
		return resolved{}, err
	}

	fftSize, err := resolveFFTSize(cfg, length)
	if err != nil {
		return resolved{}, err
	}

	q := cfg.Q
	if q == 0 {
		q = defaultQ
	}

	return resolved{
		cfg:     cfg,
		length:  length,
		fftSize: fftSize,
		bins:    fftSize/2 + 1,
		q:       q,
	}, nil
}

func validateBands(cfg Config) error {
	nyquist := cfg.SampleRate / 2

	previous := 0.0

	for i, band := range cfg.Bands {
		if math.IsNaN(band.GainDB) || math.IsInf(band.GainDB, 0) {
			return fmt.Errorf(
				"%w: band %d gain is %v",
				ErrInvalidBand,
				i,
				band.GainDB,
			)
		}

		if !(band.Frequency > 0) || band.Frequency >= nyquist {
			return fmt.Errorf(
				"%w: band %d centre %v is outside (0, %v)",
				ErrInvalidBand,
				i,
				band.Frequency,
				nyquist,
			)
		}

		if band.Frequency <= previous {
			return fmt.Errorf(
				"%w: band %d centre %v does not exceed the previous %v",
				ErrInvalidBand,
				i,
				band.Frequency,
				previous,
			)
		}

		previous = band.Frequency
	}

	return nil
}

func resolveLength(cfg Config) (int, error) {
	if cfg.Length < 0 {
		return 0, fmt.Errorf("%w: got %d", ErrInvalidLength, cfg.Length)
	}

	if cfg.Length == 0 {
		return DefaultLength(cfg), nil
	}

	if cfg.Length%2 == 0 {
		return 0, fmt.Errorf(
			"%w: linear-phase length %d is even",
			ErrInvalidLength,
			cfg.Length,
		)
	}

	return cfg.Length, nil
}

func resolveFFTSize(cfg Config, length int) (int, error) {
	if cfg.FFTSize != 0 {
		if cfg.FFTSize < length || cfg.FFTSize%2 != 0 {
			return 0, fmt.Errorf(
				"%w: FFT size %d cannot hold a length of %d",
				ErrInvalidLength,
				cfg.FFTSize,
				length,
			)
		}

		return cfg.FFTSize, nil
	}

	target := max(16, 8*length)

	size := 1
	for size < target {
		size <<= 1
	}

	return size, nil
}

func (r resolved) frequencies() []float64 {
	out := make([]float64, r.bins)
	for k := range out {
		out[k] = float64(k) * r.cfg.SampleRate / float64(r.fftSize)
	}

	return out
}

// designSections realises the offloaded bands as a cascade of low shelves, one
// per band, each stepping from one band gain to the next.
//
// Shelves rather than peaking sections, because everything below the lowest
// centre has to be shaped too and that is exactly the region the shortened FIR
// can no longer reach. Placing shelf i at the geometric mean of two neighbouring
// centres and giving it their gain difference makes the cascade agree with the
// target at every centre and at every midpoint: an RBJ shelf reaches half its
// gain at its corner, which is where the log-linear target curve is also
// halfway.
//
// The cascade therefore approximates the target relative to the gain of the
// first band left to the FIR, and the FIR supplies that reference level as a
// broadband offset it can realise at any length.
func designSections(r resolved) []biquad.Coefficients {
	if r.cfg.IIRBands == 0 {
		return nil
	}

	sections := make([]biquad.Coefficients, 0, r.cfg.IIRBands)

	for i := range r.cfg.IIRBands {
		lower, upper := r.cfg.Bands[i], r.cfg.Bands[i+1]

		sections = append(sections, design.LowShelf(
			math.Sqrt(lower.Frequency*upper.Frequency),
			lower.GainDB-upper.GainDB,
			r.q,
			r.cfg.SampleRate,
		))
	}

	return sections
}

func cascadeMagnitudeDB(
	sections []biquad.Coefficients,
	frequencies []float64,
	sampleRate float64,
) []float64 {
	out := make([]float64, len(frequencies))
	if len(sections) == 0 {
		return out
	}

	chain := biquad.NewChain(sections)
	for k, frequency := range frequencies {
		out[k] = chain.MagnitudeDB(frequency, sampleRate)
	}

	return out
}

// targetCurve interpolates the requested band gains linearly in log frequency
// and holds the outermost gains beyond the outermost centres.
func targetCurve(bands []Band, frequencies []float64) []float64 {
	out := make([]float64, len(frequencies))

	for k, frequency := range frequencies {
		out[k] = interpolateGain(bands, frequency)
	}

	return out
}

func interpolateGain(bands []Band, frequency float64) float64 {
	if frequency <= bands[0].Frequency {
		return bands[0].GainDB
	}

	last := bands[len(bands)-1]
	if frequency >= last.Frequency {
		return last.GainDB
	}

	for i := 1; i < len(bands); i++ {
		upper := bands[i]
		if frequency > upper.Frequency {
			continue
		}

		lower := bands[i-1]
		span := math.Log2(upper.Frequency / lower.Frequency)
		position := math.Log2(frequency/lower.Frequency) / span

		return lower.GainDB + position*(upper.GainDB-lower.GainDB)
	}

	return last.GainDB
}

// designFIR builds the linear-phase FIR that corrects whatever the cascade left
// over, and returns its dB magnitude on the design grid.
func designFIR(
	r resolved,
	targetDB, sectionDB []float64,
) ([]float64, []float64, error) {
	workspace, err := newFFTWorkspace(r.fftSize)
	if err != nil {
		return nil, nil, err
	}

	delay := float64(r.length-1) / 2

	spectrum := make([]complex128, r.fftSize)

	for k := range r.bins {
		magnitude := math.Pow(10, (targetDB[k]-sectionDB[k])/20)
		angle := -2 * math.Pi * float64(k) * delay / float64(r.fftSize)
		value := complex(
			magnitude*math.Cos(angle),
			magnitude*math.Sin(angle),
		)

		spectrum[k] = value
		if k > 0 && k < r.fftSize-k {
			spectrum[r.fftSize-k] = complex(real(value), -imag(value))
		}
	}

	impulse, err := workspace.inverseReal(spectrum)
	if err != nil {
		return nil, nil, err
	}

	taps := make([]float64, r.length)
	copy(taps, impulse)
	applyWindow(r, taps)

	response, err := workspace.forwardReal(taps)
	if err != nil {
		return nil, nil, err
	}

	firDB := make([]float64, r.bins)
	for k := range firDB {
		firDB[k] = 20 * math.Log10(
			max(magnitudeFloor, complexAbs(response[k])),
		)
	}

	return taps, firDB, nil
}

func applyWindow(r resolved, taps []float64) {
	if r.cfg.Window == window.TypeRectangular {
		return
	}

	options := []window.Option{}
	if r.cfg.WindowAlpha > 0 {
		options = append(options, window.WithAlpha(r.cfg.WindowAlpha))
	}

	coefficients := window.Generate(r.cfg.Window, len(taps), options...)
	for i := range taps {
		taps[i] *= coefficients[i]
	}
}

func complexAbs(value complex128) float64 {
	return math.Hypot(real(value), imag(value))
}

// measure compares the combined dB response with the target curve over the
// range the target actually describes.
func measure(
	r resolved,
	sections []biquad.Coefficients,
	taps []float64,
	frequencies, targetDB, totalDB []float64,
) Metrics {
	lower := r.cfg.Bands[0].Frequency * measurementSpan

	metrics := Metrics{BandErrorDB: make([]float64, len(r.cfg.Bands))}
	sum, count := 0.0, 0

	for k, frequency := range frequencies {
		if frequency < lower {
			continue
		}

		deviation := totalDB[k] - targetDB[k]
		sum += deviation * deviation
		count++
		metrics.MaxErrorDB = max(metrics.MaxErrorDB, math.Abs(deviation))
	}

	if count > 0 {
		metrics.RMSErrorDB = math.Sqrt(sum / float64(count))
	}

	// The band centres rarely land on a design bin, and reading the nearest one
	// would report the error of a neighbouring frequency instead. They are
	// cheap enough to evaluate exactly.
	chain := biquad.NewChain(sections)

	for i, band := range r.cfg.Bands {
		total := firMagnitudeDB(taps, band.Frequency, r.cfg.SampleRate)
		if len(sections) > 0 {
			total += chain.MagnitudeDB(band.Frequency, r.cfg.SampleRate)
		}

		metrics.BandErrorDB[i] = total - band.GainDB
	}

	return metrics
}

// firMagnitudeDB evaluates the FIR response at one exact frequency.
func firMagnitudeDB(taps []float64, frequency, sampleRate float64) float64 {
	omega := 2 * math.Pi * frequency / sampleRate
	realPart, imagPart := 0.0, 0.0

	for n, tap := range taps {
		angle := omega * float64(n)
		realPart += tap * math.Cos(angle)
		imagPart -= tap * math.Sin(angle)
	}

	return 20 * math.Log10(
		max(magnitudeFloor, math.Hypot(realPart, imagPart)),
	)
}
