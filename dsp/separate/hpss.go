package separate

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/stft"
)

// HPSS is a harmonic/percussive source separator after Fitzgerald (2010),
// with the optional margin variant of Driedger et al. (2014).
//
// The STFT magnitude |X| is median filtered across time (length
// harmonicKernel frames), which keeps steady tones and suppresses
// transients, giving H; and across frequency (length percussiveKernel
// bins), which keeps broadband transients and suppresses tones, giving P.
// Soft masks M_h = H^p/(H^p+P^p) and M_p = P^p/(H^p+P^p) are applied to the
// complex spectrogram X. See [WithMargin] for the variant with a residual.
//
// The three outputs always sum back to the input: the masks of every bin sum
// to 1, and a bin where H and P are both zero is split evenly.
//
// An HPSS keeps scratch buffers that are reused across calls of the same
// spectrogram shape, so it is not safe for concurrent use; see
// [HPSS.Clone].
type HPSS struct {
	cfg  hpssConfig
	stft *stft.STFT

	harmFilter *MedianFilter
	percFilter *MedianFilter

	// Scratch, sized for (frames, bins).
	frames, bins int
	mag          [][]float64
	harmEnh      [][]float64 // H, then the harmonic mask
	percEnh      [][]float64 // P, then the percussive mask
	col, colOut  []float64
}

// NewHPSS returns a harmonic/percussive separator. Without options it uses
// harmonic and percussive kernels of 17, power 2, no margin, and (for
// [HPSS.SeparateSignal]) a 2048-point STFT with hop 512, periodic Hann window
// and centred zero-padded framing.
func NewHPSS(opts ...Option) (*HPSS, error) {
	cfg := defaultHPSSConfig()

	err := applyOptions(&cfg, opts)
	if err != nil {
		return nil, err
	}

	t, err := cfg.stft.build()
	if err != nil {
		return nil, err
	}

	h := &HPSS{cfg: cfg, stft: t}
	h.initFilters()

	return h, nil
}

func (h *HPSS) initFilters() {
	// Kernel sizes are validated by the options, so these cannot fail.
	h.harmFilter, _ = NewMedianFilter(h.cfg.harmKernel)
	h.percFilter, _ = NewMedianFilter(h.cfg.percKernel)
}

// Clone returns an independent HPSS with the same configuration and its own
// scratch buffers, for use in another goroutine.
func (h *HPSS) Clone() *HPSS {
	c := &HPSS{cfg: h.cfg, stft: h.stft.Clone()}
	c.initFilters()

	return c
}

// STFT returns a copy of the transform [HPSS.SeparateSignal] uses, so that
// spectrograms passed to the spectrogram-level methods can be computed and
// inverted with the same settings.
func (h *HPSS) STFT() *stft.STFT { return h.stft.Clone() }

// HasMargin reports whether the margin variant (and so a non-zero residual)
// is enabled.
func (h *HPSS) HasMargin() bool { return h.cfg.margin }

// Masks returns the harmonic, percussive and residual masks of spec, a
// frame-major spectrogram (spec[frame][bin]) in which every frame has the
// same number of bins. harm+perc+resid is 1 in every bin; resid is all zero
// unless a margin is set.
func (h *HPSS) Masks(spec [][]complex128) (harm, perc, resid [][]float64, err error) {
	frames, bins, err := rectShape(spec)
	if err != nil {
		return nil, nil, nil, err
	}

	harm = newMatrix[float64](frames, bins)
	perc = newMatrix[float64](frames, bins)
	resid = newMatrix[float64](frames, bins)

	err = h.MasksInto(harm, perc, resid, spec)
	if err != nil {
		return nil, nil, nil, err
	}

	return harm, perc, resid, nil
}

// MasksInto is [HPSS.Masks] writing into caller-provided matrices of the
// shape of spec. After the first call for a given spectrogram shape it does
// not allocate.
func (h *HPSS) MasksInto(harm, perc, resid [][]float64, spec [][]complex128) error {
	err := checkOutputs(spec, harm, perc, resid)
	if err != nil {
		return err
	}

	err = h.prepare(spec)
	if err != nil {
		return err
	}

	for i := range spec {
		mh, mp, mr := h.harmEnh[i], h.percEnh[i], resid[i]
		copy(harm[i], mh)
		copy(perc[i], mp)

		if h.cfg.margin {
			for k := range mr {
				mr[k] = 1 - mh[k] - mp[k]
			}
		} else {
			clear(mr)
		}
	}

	return nil
}

// Separate splits spec into harmonic, percussive and residual spectrograms
// whose sum is spec (up to rounding). resid is all zero unless a margin is
// set.
func (h *HPSS) Separate(spec [][]complex128) (harm, perc, resid [][]complex128, err error) {
	frames, bins, err := rectShape(spec)
	if err != nil {
		return nil, nil, nil, err
	}

	harm = newMatrix[complex128](frames, bins)
	perc = newMatrix[complex128](frames, bins)
	resid = newMatrix[complex128](frames, bins)

	err = h.SeparateInto(harm, perc, resid, spec)
	if err != nil {
		return nil, nil, nil, err
	}

	return harm, perc, resid, nil
}

// SeparateInto is [HPSS.Separate] writing into caller-provided spectrograms
// of the shape of spec; any one of them may be spec itself. After the first
// call for a given spectrogram shape it does not allocate.
func (h *HPSS) SeparateInto(harm, perc, resid, spec [][]complex128) error {
	err := checkOutputs(spec, harm, perc, resid)
	if err != nil {
		return err
	}

	err = h.prepare(spec)
	if err != nil {
		return err
	}

	for i, row := range spec {
		mh, mp := h.harmEnh[i], h.percEnh[i]
		dh, dp, dr := harm[i], perc[i], resid[i]

		for k, x := range row {
			re, im := real(x), imag(x)
			hk := complex(re*mh[k], im*mh[k])
			pk := complex(re*mp[k], im*mp[k])

			rk := complex128(0)
			if h.cfg.margin {
				rk = x - hk - pk
			}

			dh[k], dp[k], dr[k] = hk, pk, rk
		}
	}

	return nil
}

// SeparateSignal splits x into harmonic, percussive and residual signals of
// len(x) samples each: x is transformed with the configured STFT, separated
// with [HPSS.Separate] and transformed back. With the default centred
// framing the outputs sum to x within the STFT's reconstruction error
// (~1e-15 relative). resid is all zero unless a margin is set.
func (h *HPSS) SeparateSignal(x []float64) (harm, perc, resid []float64, err error) {
	spec, err := h.stft.Forward(x)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("separate: forward STFT: %w", err)
	}

	bins := h.stft.Bins()
	hs := newMatrix[complex128](len(spec), bins)
	ps := newMatrix[complex128](len(spec), bins)

	// The residual reuses the input spectrogram.
	err = h.SeparateInto(hs, ps, spec, spec)
	if err != nil {
		return nil, nil, nil, err
	}

	harm, err = h.inverse(hs, len(x))
	if err != nil {
		return nil, nil, nil, err
	}

	perc, err = h.inverse(ps, len(x))
	if err != nil {
		return nil, nil, nil, err
	}

	if !h.cfg.margin {
		return harm, perc, make([]float64, len(x)), nil
	}

	resid, err = h.inverse(spec, len(x))
	if err != nil {
		return nil, nil, nil, err
	}

	return harm, perc, resid, nil
}

func (h *HPSS) inverse(spec [][]complex128, n int) ([]float64, error) {
	y, err := h.stft.Inverse(spec, n)
	if err != nil {
		return nil, fmt.Errorf("separate: inverse STFT: %w", err)
	}

	return y, nil
}

// checkOutputs verifies that the three outputs have the shape of spec.
func checkOutputs[T any](spec [][]complex128, harm, perc, resid [][]T) error {
	for j, o := range [3][][]T{harm, perc, resid} {
		err := sameShape(spec, o)
		if err != nil {
			return fmt.Errorf("output %d: %w", j, err)
		}
	}

	return nil
}

// prepare validates spec, then leaves the harmonic mask in h.harmEnh and the
// percussive mask in h.percEnh.
func (h *HPSS) prepare(spec [][]complex128) error {
	frames, bins, err := rectShape(spec)
	if err != nil {
		return err
	}

	h.ensureScratch(frames, bins)
	magnitudes(h.mag, spec)

	for i, row := range h.mag {
		for k, v := range row {
			if math.IsInf(v, 0) || math.IsNaN(v) {
				return fmt.Errorf("%w: frame %d bin %d is not finite", ErrInvalidValue, i, k)
			}
		}
	}

	h.enhance()
	h.computeMasks()

	return nil
}

// enhance median filters h.mag across time into h.harmEnh and across
// frequency into h.percEnh.
func (h *HPSS) enhance() {
	for k := range h.bins {
		for i := range h.frames {
			h.col[i] = h.mag[i][k]
		}

		h.harmFilter.filter(h.colOut, h.col)

		for i := range h.frames {
			h.harmEnh[i][k] = h.colOut[i]
		}
	}

	for i := range h.frames {
		h.percFilter.filter(h.percEnh[i], h.mag[i])
	}
}

// computeMasks replaces the enhanced magnitudes by the masks, in place.
func (h *HPSS) computeMasks() {
	p := h.cfg.power

	for i := range h.frames {
		hr, pr := h.harmEnh[i], h.percEnh[i]

		for k := range hr {
			hv, pv := hr[k], pr[k]

			if h.cfg.margin {
				hr[k], _ = softPair(hv, h.cfg.marginH*pv, p)
				pr[k], _ = softPair(pv, h.cfg.marginP*hv, p)
			} else {
				hr[k], pr[k] = softPair(hv, pv, p)
			}
		}
	}
}

func (h *HPSS) ensureScratch(frames, bins int) {
	if h.mag != nil && frames == h.frames && bins == h.bins {
		return
	}

	h.frames, h.bins = frames, bins
	h.mag = newMatrix[float64](frames, bins)
	h.harmEnh = newMatrix[float64](frames, bins)
	h.percEnh = newMatrix[float64](frames, bins)
	h.col = make([]float64, frames)
	h.colOut = make([]float64, frames)
}

// rectShape returns the shape of a spectrogram whose frames all have the
// same number of bins.
func rectShape[T any](spec [][]T) (frames, bins int, err error) {
	frames = len(spec)
	if frames == 0 {
		return 0, 0, nil
	}

	bins = len(spec[0])

	for i, row := range spec {
		if len(row) != bins {
			return 0, 0, fmt.Errorf("%w: frame %d has %d bins, frame 0 has %d", ErrShapeMismatch, i, len(row), bins)
		}
	}

	return frames, bins, nil
}
