package signal

import "fmt"

// AddInto32 sums equally sized a and b into dst without clipping. Arithmetic
// follows IEEE-754, including overflow and nonfinite propagation. Exact aliasing
// with either input is supported; partial overlap is not. Mismatched lengths
// leave dst unchanged. Empty inputs are allowed. The successful path allocates
// nothing and requires no float64 scratch buffers.
func AddInto32(dst, a, b []float32) error {
	if len(dst) != len(a) || len(dst) != len(b) {
		return fmt.Errorf("signal.add: equal lengths required")
	}

	for i, sample := range a {
		dst[i] = sample + b[i]
	}

	return nil
}

// AverageInto32 averages equally sized a and b into dst without clipping.
// Each pair uses a float64 sum and scaling followed by one float32 rounding, so
// finite float32 inputs cannot overflow and subnormal inputs are not prematurely
// rounded. IEEE-754 nonfinite inputs propagate. Exact aliasing with either input
// is supported; partial overlap is not. Mismatched lengths leave dst unchanged.
// Empty inputs are allowed. The successful path allocates nothing and requires
// no float64 scratch buffers.
func AverageInto32(dst, a, b []float32) error {
	if len(dst) != len(a) || len(dst) != len(b) {
		return fmt.Errorf("signal.average: equal lengths required")
	}

	for i, sample := range a {
		dst[i] = float32((float64(sample) + float64(b[i])) * 0.5)
	}

	return nil
}
