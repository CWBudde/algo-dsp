package core

import (
	"errors"
	"fmt"
)

var (
	// ErrNegativePad is returned by [PadReflect] when a pad length is negative.
	ErrNegativePad = errors.New("core: negative pad length")

	// ErrPadTooLong is returned by [PadReflect] when a pad length is not
	// shorter than the input, which reflect padding cannot express.
	ErrPadTooLong = errors.New("core: reflect pad must be shorter than the input")

	// ErrShortBuffer is returned when a destination buffer is too short for
	// the requested output.
	ErrShortBuffer = errors.New("core: destination buffer too short")

	// ErrOverlap is returned when a destination buffer overlaps a source
	// buffer and in-place operation is not supported.
	ErrOverlap = errors.New("core: destination overlaps source")
)

// PadReflect writes x, reflect-padded by left samples on the left and right
// samples on the right, into dst[:left+len(x)+right].
//
// With n = len(x) the output is
//
//	dst[left+i]     = x[i]       for i in [0, n)
//	dst[left-1-j]   = x[j+1]     for j in [0, left)
//	dst[left+n+j]   = x[n-2-j]   for j in [0, right)
//
// so the signal is mirrored about its first and last samples without
// repeating them: x = [1 2 3 4] padded by 2 on both sides gives
// [3 2 1 2 3 4 3 2]. This matches numpy.pad(mode="reflect") and
// torch.nn.functional.pad(mode="reflect").
//
// Note that scipy's "reflect" mode (scipy.ndimage, used by the median filters
// in dsp/separate) mirrors including the edge sample, i.e. [2 1 1 2 3 4 4 3]
// for the example above. That is numpy's "symmetric" mode and deliberately
// not what PadReflect does.
//
// As in numpy and torch, each non-zero pad must be shorter than the input,
// otherwise [ErrPadTooLong] is returned; this rejects any non-zero pad of an
// empty x. Negative pads return [ErrNegativePad], a dst shorter than
// left+len(x)+right returns [ErrShortBuffer], and an output range
// dst[:left+len(x)+right] that overlaps x returns [ErrOverlap]: in-place
// padding is not supported. With left = right = 0, x is copied into dst.
//
// Elements of dst beyond left+len(x)+right are left untouched. PadReflect
// does not allocate.
func PadReflect(dst, x []float64, left, right int) error {
	n := len(x)

	if left < 0 || right < 0 {
		return fmt.Errorf("%w: left %d, right %d", ErrNegativePad, left, right)
	}

	if (left > 0 && left >= n) || (right > 0 && right >= n) {
		return fmt.Errorf("%w: left %d, right %d for %d samples", ErrPadTooLong, left, right, n)
	}

	total := left + n + right
	if len(dst) < total {
		return fmt.Errorf("%w: need %d, have %d", ErrShortBuffer, total, len(dst))
	}

	if Overlaps(dst[:total], x) {
		return fmt.Errorf("%w: in-place reflect padding is not supported", ErrOverlap)
	}

	copy(dst[left:left+n], x)

	for j := range left {
		dst[left-1-j] = x[j+1]
	}

	tail := dst[left+n : total]
	for j := range tail {
		tail[j] = x[n-2-j]
	}

	return nil
}
