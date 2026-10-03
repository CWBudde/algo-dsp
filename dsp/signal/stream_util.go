package signal

import (
	"fmt"
	"math"
)

// MeanAccumulator computes a compensated mean across arbitrarily partitioned
// float32 blocks. The zero value is ready to use. It retains no input samples.
type MeanAccumulator struct {
	sum, correction float64
	count           int64
}

// AddFloat32 adds a block in sample order. Nonfinite samples and count overflow
// are rejected atomically. Empty blocks are allowed. The successful path does
// not allocate.
func (a *MeanAccumulator) AddFloat32(data []float32) error {
	if int64(len(data)) > math.MaxInt64-a.count {
		return fmt.Errorf("signal.mean: sample count overflow")
	}

	next := *a

	for _, sample := range data {
		if math.Float32bits(sample)&0x7fffffff >= 0x7f800000 {
			return fmt.Errorf("signal.mean: samples must be finite")
		}

		x := float64(sample)

		t := next.sum + x
		if math.Abs(next.sum) >= math.Abs(x) {
			next.correction += (next.sum - t) + x
		} else {
			next.correction += (x - t) + next.sum
		}

		next.sum = t
	}

	next.count += int64(len(data))
	*a = next

	return nil
}

// Count returns the number of samples accumulated.
func (a *MeanAccumulator) Count() int64 { return a.count }

// Mean returns the mean of all accumulated samples, or an error for empty input.
func (a *MeanAccumulator) Mean() (float64, error) {
	if a.count == 0 {
		return 0, fmt.Errorf("signal.mean: input must not be empty")
	}

	return (a.sum + a.correction) / float64(a.count), nil
}

// SubtractMeanInto32 subtracts a previously measured full-range mean from src.
// dst must have the same length. Exact aliasing is supported; partial overlap
// is not. Arithmetic follows IEEE-754 without clipping. Nonfinite mean and
// mismatched lengths leave dst unchanged. The successful path does not allocate.
func SubtractMeanInto32(dst, src []float32, mean float64) error {
	if len(dst) != len(src) || math.IsNaN(mean) || math.IsInf(mean, 0) {
		return fmt.Errorf("signal.subtract-mean: equal lengths and finite mean required")
	}

	for i, sample := range src {
		dst[i] = float32(float64(sample) - mean)
	}

	return nil
}

// ScaleInto32 multiplies src by a finite gain, including negative gains, into
// equally sized dst. Exact aliasing is supported; partial overlap is not. It
// performs one float64 multiplication and one float32 rounding per sample, with
// no clipping or input scan. IEEE-754 nonfinite input/output propagates. Invalid
// arguments leave dst unchanged. The successful path does not allocate.
func ScaleInto32(dst, src []float32, gain float64) error {
	if len(dst) != len(src) || math.IsNaN(gain) || math.IsInf(gain, 0) {
		return fmt.Errorf("signal.scale: equal lengths and finite gain required")
	}

	for i, sample := range src {
		dst[i] = float32(float64(sample) * gain)
	}

	return nil
}
