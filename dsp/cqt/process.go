package cqt

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/conv"
	"github.com/cwbudde/algo-dsp/dsp/core"
)

// sample is the element type of the output slices.
type sample interface{ ~float32 | ~float64 }

// topLength returns the length of the top octave's signal for n input
// samples, or false if the signal is empty or too short to be decimated down
// to the lowest octave (every decimation needs at least 2 input samples).
func (t *Transform) topLength(n int) (int, bool) {
	if n < 1 {
		return 0, false
	}

	if t.factor > 1 {
		n = decimatedLength(n, lowpassTaps, t.factor)
		if n < 1 {
			return 0, false
		}
	}

	m := n
	for range t.nOctaves - 1 {
		m = decimatedLength(m, lowpassTaps, 2)
		if m < 1 {
			return 0, false
		}
	}

	return n, true
}

// decimatedLength is the output length of decimate: torch conv1d with
// padding (taps-1)/2 and stride factor, or 0 if the padded input is shorter
// than the filter.
func decimatedLength(n, taps, factor int) int {
	padded := n + 2*((taps-1)/2)
	if padded < taps {
		return 0
	}

	return (padded-taps)/factor + 1
}

// NumFrames returns the number of frames the transform produces for n input
// samples, or 0 if n samples are too short to be processed. All octaves
// yield the same number of frames, (L+2*(nfft/2)-nfft)/hop + 1 = L/hop + 1
// for a top-octave signal of L samples.
func (t *Transform) NumFrames(n int) int {
	top, ok := t.topLength(n)
	if !ok {
		return 0
	}

	return (top+2*(t.nFFT/2)-t.nFFT)/t.hop + 1
}

// OutputLen returns the number of output values for n input samples:
// NumFrames(n)*NumBins(), times 2 for complex output.
func (t *Transform) OutputLen(n int) int {
	l := t.NumFrames(n) * t.nBins
	if t.output == OutputComplex {
		l *= 2
	}

	return l
}

// Process returns the constant-Q transform of x in a newly allocated slice
// of OutputLen(len(x)) values. See [Transform.ProcessInto] for the layout.
func (t *Transform) Process(x []float64) ([]float64, error) {
	if t.NumFrames(len(x)) == 0 {
		return nil, t.tooShort(len(x))
	}

	dst := make([]float64, t.OutputLen(len(x)))

	err := process(t, dst, x)
	if err != nil {
		return nil, err
	}

	return dst, nil
}

// ProcessInto writes the constant-Q transform of x to dst[:OutputLen(len(x))]
// and leaves the rest of dst untouched.
//
// The layout is frame-major. With [OutputMagnitude], dst[frame*NumBins()+bin]
// is the magnitude of bin bin in frame frame. With [OutputComplex],
// dst[2*(frame*NumBins()+bin)] is the real part and the following element
// the imaginary part. Bins are ordered by frequency, lowest first.
//
// ProcessInto returns [ErrSignalTooShort] if NumFrames(len(x)) is 0 and
// [ErrShortDst] if dst is too short. It does not allocate once its scratch
// buffers have grown to the input length. dst may overlap x; x is then read
// from a copy, because the lower octaves are decimated from x after the top
// octave's results have been written.
func (t *Transform) ProcessInto(dst, x []float64) error {
	if core.Overlaps(dst, x) {
		err := t.checkLengths(len(dst), len(x))
		if err != nil {
			return err
		}

		t.in64 = core.EnsureLen(t.in64, len(x))
		copy(t.in64, x)
		x = t.in64
	}

	return process(t, dst, x)
}

// ProcessInto32 is [Transform.ProcessInto] for float32 input and output. The
// input is converted to float64 and the whole transform is computed in
// float64; only the result is rounded to float32.
func (t *Transform) ProcessInto32(dst, x []float32) error {
	err := t.checkLengths(len(dst), len(x))
	if err != nil {
		return err
	}

	t.in64 = core.EnsureLen(t.in64, len(x))
	for i, v := range x {
		t.in64[i] = float64(v)
	}

	return process(t, dst, t.in64)
}

func (t *Transform) tooShort(n int) error {
	return fmt.Errorf("%w: %d samples for %d octaves (early downsampling factor %d)",
		ErrSignalTooShort, n, t.nOctaves, t.factor)
}

func (t *Transform) checkLengths(nDst, nx int) error {
	if t.NumFrames(nx) == 0 {
		return t.tooShort(nx)
	}

	if need := t.OutputLen(nx); nDst < need {
		return fmt.Errorf("%w: %d values, need %d", ErrShortDst, nDst, need)
	}

	return nil
}

// process runs nnAudio CQT2010v2.forward: optional early downsampling, the
// top octave, then for each lower octave a 2:1 decimation and the same
// kernels with half the hop.
func process[T sample](t *Transform, dst []T, x []float64) error {
	err := t.checkLengths(len(dst), len(x))
	if err != nil {
		return err
	}

	frames := t.NumFrames(len(x))
	cur, slot := x, 0

	if t.factor > 1 {
		t.octave[0] = core.EnsureLen(t.octave[0], decimatedLength(len(x), lowpassTaps, t.factor))
		decimate(t.octave[0], x, t.earlyLowpass, t.factor)
		cur, slot = t.octave[0], 1
	}

	hop := t.hop
	// The stacked rows run from the lowest octave's first filter to the top
	// octave's last; the lowest nOctaves*nFilters-nBins rows are dropped.
	drop := t.nOctaves*t.nFilters - t.nBins

	for o := range t.nOctaves {
		if o > 0 {
			hop /= 2
			next := core.EnsureLen(t.octave[slot], decimatedLength(len(cur), lowpassTaps, 2))
			decimate(next, cur, t.lowpass, 2)
			t.octave[slot] = next
			cur, slot = next, 1-slot
		}

		binBase := (t.nOctaves-1-o)*t.nFilters - drop
		convolve(t, dst, cur, hop, binBase, frames)
	}

	return nil
}

// decimate is nnAudio's downsampling_by_n: torch conv1d (cross-correlation)
// of x with h, zero padding (len(h)-1)/2 on both sides and stride factor.
// dst must have decimatedLength(len(x), len(h), factor) elements and must not
// overlap x or h.
func decimate(dst, x, h []float64, factor int) {
	// Cannot fail: factor >= 2, h is a non-empty filter and dst is a scratch
	// buffer distinct from x and h.
	_ = conv.CorrelateStridedInto(dst, x, h, -(len(h)-1)/2, factor)
}

// convolve applies the kernels to one octave's signal x with stride hop and
// writes filter f to output bin binBase+f; filters below bin 0 are skipped.
func convolve[T sample](t *Transform, dst []T, x []float64, hop, binBase, frames int) {
	pad := t.nFFT / 2
	n := len(x)
	xp := core.EnsureLen(t.padded, n+2*pad)
	t.padded = xp

	if t.padding == PadReflect && pad < n {
		// Cannot fail: pad < n, xp has n+2*pad elements and is a scratch
		// buffer distinct from x.
		_ = core.PadReflect(xp, x, pad, pad)
	} else {
		// Constant padding, or torch's reflection pad rejected the octave
		// (pad >= n) and nnAudio fell back to zeros.
		clear(xp[:pad])
		copy(xp[pad:], x)
		clear(xp[pad+n:])
	}

	fFirst := max(0, -binBase)
	complexOut := t.output == OutputComplex

	for fr := range frames {
		base := fr * hop
		row := fr * t.nBins

		for f := fFirst; f < t.nFilters; f++ {
			k := &t.kernels[f]
			re, im := dot2(xp[base+k.start:], k.re, k.im)

			b := binBase + f
			s := t.scale[b]
			re *= s
			im *= -s // torch: CQT_imag = -conv1d(x, kernel_imag)

			if complexOut {
				dst[2*(row+b)] = T(re)
				dst[2*(row+b)+1] = T(im)
			} else {
				dst[row+b] = T(math.Sqrt(re*re + im*im))
			}
		}
	}
}

// dot2 returns the dot products of x[:len(a)] with a and with b in one pass.
//
// The kernel correlation deliberately does not use conv.CorrelateStridedInto,
// unlike decimate. On the basic-pitch benchmark (BenchmarkProcessIntoBasicPitch,
// Apple M5 Pro, 46.2) computing each filter's real and imaginary rows with two
// CorrelateStridedInto calls and scattering them into the frame-major output
// was 8-14% slower than this fused loop, and two vecmath.DotProduct calls per
// window (which read x twice) were 10% slower. Both exceed the 5% budget, so
// the fused real+imaginary loop stays.
func dot2(x, a, b []float64) (float64, float64) {
	n := len(a)
	x = x[:n]
	b = b[:n]

	var r0, r1, r2, r3, i0, i1, i2, i3 float64

	i := 0
	for ; i+4 <= n; i += 4 {
		x0, x1, x2, x3 := x[i], x[i+1], x[i+2], x[i+3]
		r0 += x0 * a[i]
		r1 += x1 * a[i+1]
		r2 += x2 * a[i+2]
		r3 += x3 * a[i+3]
		i0 += x0 * b[i]
		i1 += x1 * b[i+1]
		i2 += x2 * b[i+2]
		i3 += x3 * b[i+3]
	}

	for ; i < n; i++ {
		r0 += x[i] * a[i]
		i0 += x[i] * b[i]
	}

	return (r0 + r1) + (r2 + r3), (i0 + i1) + (i2 + i3)
}
