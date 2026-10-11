package conv

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/core"
	"github.com/cwbudde/algo-vecmath"
)

// CorrelateStridedInto computes a strided cross-correlation of x with the
// kernel h and writes it to dst. For every i in [0, len(dst)):
//
//	dst[i] = sum_{k=0}^{len(h)-1} h[k] * x[start + i*stride + k]
//
// Samples outside x are taken as zero, so a negative start pads x with zeros
// on the left and windows that run past the end of x are padded with zeros on
// the right. Windows that lie entirely outside x produce exactly 0.
//
// This is torch.nn.functional.conv1d (a cross-correlation, the kernel is not
// reversed) with stride stride and the zero padding expressed through start:
//
//	conv1d(x, h, stride=s, padding=p) == CorrelateStridedInto(dst, x, h, -p, s)
//
// with len(dst) = (len(x)+2p-len(h))/s + 1. start = 0, stride = 1 and
// len(dst) = len(x)-len(h)+1 gives the valid part of the correlation.
//
// Each output that overlaps x is a single [vecmath.DotProduct] over the taps
// inside x. dst is overwritten entirely and the call does not allocate.
//
// Errors:
//   - stride < 1 returns an error wrapping [ErrInvalidStride].
//   - len(h) == 0 returns [ErrEmptyKernel].
//   - dst sharing memory with x or h returns an error wrapping [ErrAliasing];
//     in-place operation is not supported.
//
// An empty x is valid (every output is 0) and an empty dst is a no-op once
// stride and h have been validated.
func CorrelateStridedInto(dst, x, h []float64, start, stride int) error {
	if stride < 1 {
		return fmt.Errorf("%w: stride %d, must be >= 1", ErrInvalidStride, stride)
	}

	if len(h) == 0 {
		return ErrEmptyKernel
	}

	if len(dst) == 0 {
		return nil
	}

	if core.Overlaps(dst, x) {
		return fmt.Errorf("%w: dst and x share memory", ErrAliasing)
	}

	if core.Overlaps(dst, h) {
		return fmt.Errorf("%w: dst and h share memory", ErrAliasing)
	}

	n := len(x)
	m := len(h)

	if n == 0 {
		clear(dst)

		return nil
	}

	lo, hi := stridedSpan(len(dst), n, m, start, stride)

	clear(dst[:lo])
	clear(dst[hi:])

	for i := lo; i < hi; i++ {
		// -len(h) < off < len(x), so the window overlaps x and kLo < kHi.
		off := start + i*stride
		kLo := max(0, -off)
		kHi := min(m, n-off)

		dst[i] = vecmath.DotProduct(x[off+kLo:off+kHi], h[kLo:kHi])
	}

	return nil
}

// stridedSpan returns the range [lo, hi) of output indices whose window
// [start+i*stride, start+i*stride+m) overlaps x[0:n] (n >= 1, m >= 1,
// stride >= 1), clamped to [0, outLen) with lo <= hi. Outputs before lo lie
// entirely left of x, outputs from hi on entirely right of it. The arithmetic
// is done in uint so that extreme start values cannot overflow.
func stridedSpan(outLen, n, m, start, stride int) (lo, hi int) {
	s := uint(stride)

	// Leading outputs with start+i*stride <= -m.
	if start <= -m {
		d := uint(-m) - uint(start) // -m-start >= 0, computed without overflow

		lo = int(min(d/s+1, uint(outLen)))
	}

	// Outputs with start+i*stride < n.
	if start < n {
		d := uint(n) - uint(start) // n-start >= 1, computed without overflow

		hi = int(min((d-1)/s+1, uint(outLen)))
	}

	return lo, max(lo, hi)
}
