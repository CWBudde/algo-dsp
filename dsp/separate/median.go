package separate

import (
	"fmt"
	"math"
)

// MedianFilter is a sliding-window median filter of odd length with
// "reflect" edge handling: the signal is mirrored at its ends including the
// edge sample (d c b a | a b c d | d c b a), the mode scipy.ndimage
// calls "reflect" and the one librosa's HPSS uses. The mirroring repeats
// periodically, so kernels longer than the signal are handled as scipy does.
//
// The window is kept sorted; each step removes the outgoing sample by binary
// search and inserts the incoming one by shifting, which costs O(log k + k)
// per output sample for a kernel of length k but no allocation.
//
// A MedianFilter holds a scratch window and is not safe for concurrent use.
type MedianFilter struct {
	size int
	half int
	win  []float64 // sorted window, length size
}

// NewMedianFilter returns a median filter of the given length, which must be
// odd and >= 1. A length of 1 copies its input.
func NewMedianFilter(size int) (*MedianFilter, error) {
	if size < 1 || size%2 == 0 {
		return nil, fmt.Errorf("%w: median length %d must be odd and >= 1", ErrInvalidKernel, size)
	}

	return &MedianFilter{
		size: size,
		half: size / 2,
		win:  make([]float64, size),
	}, nil
}

// Size returns the filter length.
func (m *MedianFilter) Size() int { return m.size }

// Filter writes the median of the window centred on each sample of src into
// dst. dst must have at least len(src) elements and must not overlap src
// (unless the filter length is 1). src must not contain NaN; ±Inf is
// allowed. Filter does not allocate.
func (m *MedianFilter) Filter(dst, src []float64) error {
	if len(dst) < len(src) {
		return fmt.Errorf("%w: dst has %d samples, src %d", ErrShapeMismatch, len(dst), len(src))
	}

	for i, v := range src {
		if math.IsNaN(v) {
			return fmt.Errorf("%w: NaN at index %d", ErrInvalidValue, i)
		}
	}

	m.filter(dst, src)

	return nil
}

// filter is Filter without validation.
func (m *MedianFilter) filter(dst, src []float64) {
	n := len(src)
	if n == 0 {
		return
	}

	if m.size == 1 {
		copy(dst, src)
		return
	}

	r := m.half
	win := m.win

	for j := -r; j <= r; j++ {
		insertSorted(win[:j+r], src[reflectIndex(j, n)])
	}

	for i := range n {
		dst[i] = win[r]

		if i == n-1 {
			break
		}

		replaceSorted(win, src[reflectIndex(i-r, n)], src[reflectIndex(i+r+1, n)])
	}
}

// reflectIndex maps a virtual index j onto [0, n) by mirroring at the edges
// including the edge sample, with period 2n.
func reflectIndex(j, n int) int {
	if j >= 0 && j < n {
		return j
	}

	period := 2 * n

	j %= period
	if j < 0 {
		j += period
	}

	if j >= n {
		j = period - 1 - j
	}

	return j
}

// insertSorted inserts v into the sorted slice win[:len(win)] extended by one
// element (the caller guarantees cap > len).
func insertSorted(win []float64, v float64) {
	p := len(win)
	win = win[:p+1]

	for p > 0 && win[p-1] > v {
		win[p] = win[p-1]
		p--
	}

	win[p] = v
}

// replaceSorted removes one occurrence of out from the sorted window and
// inserts in, keeping the window sorted. out must be present.
//
// Both ranks are found by branch-free counting, which beats a binary search
// plus element-wise shifting on the short windows HPSS uses, because the
// comparisons on audio data are unpredictable.
func replaceSorted(win []float64, out, in float64) {
	p, q := 0, 0

	for _, v := range win {
		p += b2i(v < out)
		q += b2i(v < in)
	}

	// p is the first index of out. q is the insertion index of in once out
	// is removed.
	q -= b2i(out < in)

	for j := p; j < q; j++ {
		win[j] = win[j+1]
	}

	for j := p; j > q; j-- {
		win[j] = win[j-1]
	}

	win[q] = in
}

func b2i(b bool) int {
	if b {
		return 1
	}

	return 0
}
