package align

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/core"
)

// Sentinel errors returned by this package. Errors are wrapped with context,
// so test for them with errors.Is.
var (
	// ErrInvalidArgument reports invalid input or option values.
	ErrInvalidArgument = errors.New("align: invalid argument")
	// ErrNilOption reports a nil [Option].
	ErrNilOption = errors.New("align: nil option")
	// ErrLengthMismatch reports a part whose length differs from the
	// reference by more than the tolerance set with [WithMaxLengthMismatch].
	ErrLengthMismatch = errors.New("align: length mismatch")
)

// Defaults of [Check].
const (
	// DefaultMaxLag is the default lag search range in seconds (±2 ms).
	DefaultMaxLag = 0.002
	// DefaultStride is the default sample decimation of the lag search.
	DefaultStride = 4
	// DBFloor is the amplitude floor of ResidualRMSDB (-120 dB).
	DBFloor = 1e-6
)

// Result is the outcome of [Check].
type Result struct {
	// Correlation is the normalized correlation of reference and part sum
	// at lag 0 (on the decimated grid).
	Correlation float64
	// BestLag is the lag in seconds with the highest correlation; positive
	// means the part sum lags behind the reference. Only lags whose
	// decimated overlap has non-zero energy in both signals are
	// candidates; without any, BestLag is 0.
	BestLag float64
	// BestCorrelation is the correlation at BestLag (0 without a
	// candidate).
	BestCorrelation float64
	// ResidualRMS is the RMS of reference - sum over all samples.
	ResidualRMS float64
	// ResidualRMSDB is ResidualRMS in dBFS, floored at -120 dB.
	ResidualRMSDB float64
	// MaxLengthMismatch is the largest difference in seconds between the
	// length of a part channel and the reference.
	MaxLengthMismatch float64
}

// Option configures [Check].
type Option func(*config) error

type config struct {
	maxLag        float64
	stride        int
	maxMismatch   float64
	checkMismatch bool
}

// WithMaxLag sets the lag search range to ±seconds (default 2 ms), rounded
// to whole samples. [Check] limits the range to lags shorter than the
// reference, the only ones that overlap it.
func WithMaxLag(seconds float64) Option {
	return func(c *config) error {
		if !(seconds >= 0) || math.IsInf(seconds, 0) {
			return fmt.Errorf("%w: max lag %v", ErrInvalidArgument, seconds)
		}

		c.maxLag = seconds

		return nil
	}
}

// WithStride evaluates the lag correlation on every n-th sample (default 4;
// 1 uses every sample).
func WithStride(n int) Option {
	return func(c *config) error {
		if n < 1 {
			return fmt.Errorf("%w: stride %d", ErrInvalidArgument, n)
		}

		c.stride = n

		return nil
	}
}

// WithMaxLengthMismatch makes Check fail with [ErrLengthMismatch] when a
// part channel's length differs from the reference by more than seconds.
// By default lengths are only reported.
func WithMaxLengthMismatch(seconds float64) Option {
	return func(c *config) error {
		if !(seconds >= 0) || math.IsInf(seconds, 0) {
			return fmt.Errorf("%w: max length mismatch %v", ErrInvalidArgument, seconds)
		}

		c.maxMismatch, c.checkMismatch = seconds, true

		return nil
	}
}

// Check compares the reference (one slice per channel, equal non-zero
// lengths) with the sum of parts (each one slice per channel; channel
// lengths may differ from the reference, extra samples are ignored and
// missing ones count as zero), all sampled at sampleRate.
func Check(ref [][]float64, parts [][][]float64, sampleRate float64, opts ...Option) (Result, error) {
	cfg := config{maxLag: DefaultMaxLag, stride: DefaultStride}

	for i, opt := range opts {
		if opt == nil {
			return Result{}, fmt.Errorf("%w: option %d", ErrNilOption, i)
		}

		err := opt(&cfg)
		if err != nil {
			return Result{}, err
		}
	}

	n, err := validate(ref, parts, sampleRate)
	if err != nil {
		return Result{}, err
	}

	var result Result

	sum := make([]float64, n)
	reference := make([]float64, n)

	for p, part := range parts {
		for _, ch := range part {
			mismatch := math.Abs(float64(len(ch)-n)) / sampleRate
			result.MaxLengthMismatch = math.Max(result.MaxLengthMismatch, mismatch)

			if cfg.checkMismatch && mismatch > cfg.maxMismatch {
				return Result{}, fmt.Errorf("%w: part %d differs by %.3f ms", ErrLengthMismatch, p, mismatch*1000)
			}

			for i := 0; i < min(n, len(ch)); i++ {
				sum[i] += ch[i] / float64(len(part))
			}
		}
	}

	for _, ch := range ref {
		for i, v := range ch {
			reference[i] += v / float64(len(ref))
		}
	}

	// Lags of n samples or more leave no overlap. Clamp in float so that a
	// huge WithMaxLag neither overflows the int conversion nor scans lags
	// that cannot overlap.
	maxLag := int(math.Min(math.Round(cfg.maxLag*sampleRate), float64(n-1)))
	found := false

	for lag := -maxLag; lag <= maxLag; lag++ {
		dot, aa, bb := 0.0, 0.0, 0.0

		for i := max(0, -lag); i < min(n, n-lag); i += cfg.stride {
			a, b := reference[i], sum[i+lag]
			dot += a * b
			aa += a * a
			bb += b * b
		}

		// Without signal energy in the overlap the correlation is
		// undefined: lag 0 reports 0, and the lag is no BestLag candidate.
		if !(aa*bb > 0) {
			continue
		}

		corr := dot / math.Sqrt(aa*bb)

		if lag == 0 {
			result.Correlation = corr
		}

		if !found || corr > result.BestCorrelation {
			found = true
			result.BestLag = float64(lag) / sampleRate
			result.BestCorrelation = corr
		}
	}

	if !found {
		result.BestLag, result.BestCorrelation = 0, result.Correlation
	}

	residual := 0.0

	for i, v := range reference {
		d := v - sum[i]
		residual += d * d
	}

	result.ResidualRMS = math.Sqrt(residual / float64(n))
	result.ResidualRMSDB = core.LinearToDBFloor(result.ResidualRMS, DBFloor)

	return result, nil
}

func validate(ref [][]float64, parts [][][]float64, sampleRate float64) (int, error) {
	if !(sampleRate > 0) || math.IsInf(sampleRate, 0) {
		return 0, fmt.Errorf("%w: sample rate %v", ErrInvalidArgument, sampleRate)
	}

	if len(ref) == 0 || len(ref[0]) == 0 {
		return 0, fmt.Errorf("%w: empty reference", ErrInvalidArgument)
	}

	n := len(ref[0])
	for i, ch := range ref {
		if len(ch) != n {
			return 0, fmt.Errorf("%w: reference channel %d has %d samples, want %d", ErrInvalidArgument, i, len(ch), n)
		}
	}

	if len(parts) == 0 {
		return 0, fmt.Errorf("%w: no parts", ErrInvalidArgument)
	}

	for p, part := range parts {
		if len(part) == 0 {
			return 0, fmt.Errorf("%w: part %d has no channels", ErrInvalidArgument, p)
		}
	}

	return n, nil
}
