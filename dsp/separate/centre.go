package separate

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/stft"
)

// MidSide converts a stereo pair to mid = (l+r)/2 and side = (l−r)/2. The
// pair is restored by l = mid+side, r = mid−side ([LeftRight]), exactly up
// to one rounding per sample. l and r must have the same length.
//
// The mid channel holds everything common to both channels, so it is the
// crudest "centre" extraction: a centre-panned source ends up entirely in
// mid, but so does half of every other source panned anywhere except hard
// opposite. [CentreExtractor] separates more selectively.
func MidSide(l, r []float64) (mid, side []float64, err error) {
	if len(l) != len(r) {
		return nil, nil, fmt.Errorf("%w: left %d, right %d samples", ErrShapeMismatch, len(l), len(r))
	}

	mid = make([]float64, len(l))
	side = make([]float64, len(l))
	midSide(mid, side, l, r)

	return mid, side, nil
}

// MidSideInto is [MidSide] writing into mid and side, which must have the
// length of l and r. mid may alias l and side may alias r. It does not
// allocate.
func MidSideInto(mid, side, l, r []float64) error {
	n := len(l)
	if len(r) != n || len(mid) != n || len(side) != n {
		return fmt.Errorf("%w: lengths mid %d, side %d, left %d, right %d",
			ErrShapeMismatch, len(mid), len(side), n, len(r))
	}

	midSide(mid, side, l, r)

	return nil
}

func midSide(mid, side, l, r []float64) {
	for i := range l {
		a, b := l[i], r[i]
		mid[i] = 0.5 * (a + b)
		side[i] = 0.5 * (a - b)
	}
}

// LeftRight is the inverse of [MidSide]: l = mid+side, r = mid−side.
func LeftRight(mid, side []float64) (l, r []float64, err error) {
	if len(mid) != len(side) {
		return nil, nil, fmt.Errorf("%w: mid %d, side %d samples", ErrShapeMismatch, len(mid), len(side))
	}

	l = make([]float64, len(mid))
	r = make([]float64, len(mid))

	for i := range mid {
		l[i] = mid[i] + side[i]
		r[i] = mid[i] - side[i]
	}

	return l, r, nil
}

// CentreExtractor is a heuristic STFT-domain centre extractor, a cheap
// "vocals-ish / centre" split for stereo mixes. It is not a source
// separator: it finds what is panned to the centre, which in typical pop
// mixes is lead vocal, bass, kick and snare together.
//
// For every bin with left and right spectra L and R it computes the
// coherence
//
//	c = max(0, 2·Re(L·conj(R)) / (|L|² + |R|²))
//
// which is 1 exactly when L = R (same magnitude and phase) and falls with
// level difference (c = 2g/(1+g²) for a gain ratio g) or phase difference
// (c ∝ cos Δφ). The centre mask is c^e (exponent e, [WithCentreExponent]),
// and the centre spectrum is C = c^e·(L+R)/2. Bins where both channels are
// zero get mask 0.
//
// The outputs are a mono centre signal and the two side signals L−C and
// R−C, so centre+sideL and centre+sideR reconstruct the input channels.
//
// A CentreExtractor holds an STFT with scratch buffers and is not safe for
// concurrent use; see [CentreExtractor.Clone].
type CentreExtractor struct {
	cfg  centreConfig
	stft *stft.STFT
}

// NewCentreExtractor returns a centre extractor. Without options it uses
// exponent 2 and (for [CentreExtractor.SeparateSignal]) a 2048-point STFT
// with hop 512, periodic Hann window and centred zero-padded framing.
func NewCentreExtractor(opts ...CentreOption) (*CentreExtractor, error) {
	cfg := defaultCentreConfig()

	err := applyOptions(&cfg, opts)
	if err != nil {
		return nil, err
	}

	t, err := cfg.stft.build()
	if err != nil {
		return nil, err
	}

	return &CentreExtractor{cfg: cfg, stft: t}, nil
}

// Clone returns an independent CentreExtractor with the same configuration,
// for use in another goroutine.
func (c *CentreExtractor) Clone() *CentreExtractor {
	return &CentreExtractor{cfg: c.cfg, stft: c.stft.Clone()}
}

// STFT returns a copy of the transform [CentreExtractor.SeparateSignal] uses.
func (c *CentreExtractor) STFT() *stft.STFT { return c.stft.Clone() }

// Mask returns the centre mask c^e for every bin of the left and right
// spectrograms, which must have the same shape.
func (c *CentreExtractor) Mask(left, right [][]complex128) ([][]float64, error) {
	err := sameShape(left, right)
	if err != nil {
		return nil, err
	}

	mask := make([][]float64, len(left))
	for i := range left {
		mask[i] = make([]float64, len(left[i]))
	}

	c.maskInto(mask, left, right)

	return mask, nil
}

// MaskInto is [CentreExtractor.Mask] writing into mask, which must have the
// shape of left and right. It does not allocate.
func (c *CentreExtractor) MaskInto(mask [][]float64, left, right [][]complex128) error {
	err := sameShape(left, right)
	if err != nil {
		return err
	}

	err = sameShape(left, mask)
	if err != nil {
		return err
	}

	c.maskInto(mask, left, right)

	return nil
}

func (c *CentreExtractor) maskInto(mask [][]float64, left, right [][]complex128) {
	e := c.cfg.exponent

	for i, lrow := range left {
		rrow, m := right[i], mask[i]

		for k, l := range lrow {
			m[k] = coherenceMask(l, rrow[k], e)
		}
	}
}

func coherenceMask(l, r complex128, e float64) float64 {
	lr, li, rr, ri := real(l), imag(l), real(r), imag(r)
	den := lr*lr + li*li + rr*rr + ri*ri

	if !(den > 0) {
		return 0
	}

	coh := 2 * (lr*rr + li*ri) / den
	switch {
	case !(coh > 0):
		return 0
	case coh >= 1:
		return 1
	}

	if e == 2 {
		return coh * coh
	}

	return math.Pow(coh, e)
}

// Separate splits stereo spectrograms into the centre spectrogram
// C = mask·(L+R)/2 and the side spectrograms L−C and R−C.
func (c *CentreExtractor) Separate(left, right [][]complex128) (centre, sideL, sideR [][]complex128, err error) {
	err = sameShape(left, right)
	if err != nil {
		return nil, nil, nil, err
	}

	centre = make([][]complex128, len(left))
	sideL = make([][]complex128, len(left))
	sideR = make([][]complex128, len(left))

	e := c.cfg.exponent

	for i, lrow := range left {
		rrow := right[i]
		centre[i] = make([]complex128, len(lrow))
		sideL[i] = make([]complex128, len(lrow))
		sideR[i] = make([]complex128, len(lrow))

		for k, l := range lrow {
			r := rrow[k]
			m := 0.5 * coherenceMask(l, r, e)
			ck := complex(m*(real(l)+real(r)), m*(imag(l)+imag(r)))
			centre[i][k] = ck
			sideL[i][k] = l - ck
			sideR[i][k] = r - ck
		}
	}

	return centre, sideL, sideR, nil
}

// SeparateSignal splits a stereo signal into a mono centre signal and the
// left and right side signals, all of len(l) samples. The centre is
// computed in the STFT domain and transformed back; the sides are then
// l−centre and r−centre in the time domain, so centre+sideL == l and
// centre+sideR == r up to one rounding per sample regardless of the STFT's
// reconstruction error.
func (c *CentreExtractor) SeparateSignal(l, r []float64) (centre, sideL, sideR []float64, err error) {
	if len(l) != len(r) {
		return nil, nil, nil, fmt.Errorf("%w: left %d, right %d samples", ErrShapeMismatch, len(l), len(r))
	}

	ls, err := c.stft.Forward(l)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("separate: forward STFT: %w", err)
	}

	rs, err := c.stft.Forward(r)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("separate: forward STFT: %w", err)
	}

	e := c.cfg.exponent

	// Write the centre spectrum into ls.
	for i, lrow := range ls {
		rrow := rs[i]

		for k, lv := range lrow {
			rv := rrow[k]
			m := 0.5 * coherenceMask(lv, rv, e)
			lrow[k] = complex(m*(real(lv)+real(rv)), m*(imag(lv)+imag(rv)))
		}
	}

	centre, err = c.stft.Inverse(ls, len(l))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("separate: inverse STFT: %w", err)
	}

	sideL = make([]float64, len(l))
	sideR = make([]float64, len(l))

	for i, cv := range centre {
		sideL[i] = l[i] - cv
		sideR[i] = r[i] - cv
	}

	return centre, sideL, sideR, nil
}
