package separate

import (
	"fmt"
	"math"
)

// SoftMasks computes generalized Wiener (soft) masks from N non-negative
// magnitude estimates. For every frame i and bin k
//
//	dst[s][i][k] = E_s[i][k]^power / Σ_j E_j[i][k]^power
//
// where E_s = estimates[s]. With power = 2 on magnitudes this is the classic
// Wiener gain (ratio of power spectra); power = 1 gives magnitude-ratio
// masks, and power = +Inf gives binary masks that assign each bin to the
// largest estimate (ties split evenly). The masks of a bin always sum to 1
// (up to rounding): a bin where all estimates are zero gets 1/N for every
// source, so mask-based separations still add up to the mixture there. The
// computation is scaled by the largest estimate of each bin, so it neither
// overflows nor underflows for large powers.
//
// estimates must hold at least one source, all with the same shape
// (frames × bins, rows may differ in length between frames but must agree
// between sources); dst must hold one matrix of that shape per source.
// dst[s] may be estimates[s] (in-place). Estimates must be finite and
// non-negative; otherwise [ErrInvalidValue] is returned and dst is left
// partially written. SoftMasks does not allocate.
//
// SoftMasks is the building block for HPSS, for Wiener post-filters of other
// separators, and for noise-reduction gains (target and noise estimate as
// two sources).
func SoftMasks(dst, estimates [][][]float64, power float64) error {
	err := checkPower(power)
	if err != nil {
		return err
	}

	if len(estimates) == 0 {
		return fmt.Errorf("%w: no estimates", ErrShapeMismatch)
	}

	if len(dst) != len(estimates) {
		return fmt.Errorf("%w: %d masks for %d estimates", ErrShapeMismatch, len(dst), len(estimates))
	}

	ref := estimates[0]

	for s := range estimates {
		err = sameShape(ref, estimates[s])
		if err != nil {
			return fmt.Errorf("estimate %d: %w", s, err)
		}

		err = sameShape(ref, dst[s])
		if err != nil {
			return fmt.Errorf("mask %d: %w", s, err)
		}
	}

	n := float64(len(estimates))

	for i := range ref {
		for k := range ref[i] {
			peak := 0.0

			for s := range estimates {
				v := estimates[s][i][k]
				if !(v >= 0) || math.IsInf(v, 1) {
					return fmt.Errorf("%w: estimate %d frame %d bin %d is %g", ErrInvalidValue, s, i, k, v)
				}

				peak = max(peak, v)
			}

			if peak == 0 {
				for s := range dst {
					dst[s][i][k] = 1 / n
				}

				continue
			}

			sum := 0.0

			for s := range estimates {
				v := powUnit(estimates[s][i][k]/peak, power)
				dst[s][i][k] = v
				sum += v
			}

			for s := range dst {
				dst[s][i][k] /= sum
			}
		}
	}

	return nil
}

// ApplyMask multiplies every bin of spec by the real mask value of the same
// frame and bin and writes the result to dst. dst may be spec (in-place).
// All three must have the same shape. ApplyMask does not allocate.
func ApplyMask(dst, spec [][]complex128, mask [][]float64) error {
	err := sameShape(spec, dst)
	if err != nil {
		return fmt.Errorf("dst: %w", err)
	}

	err = sameShape(spec, mask)
	if err != nil {
		return fmt.Errorf("mask: %w", err)
	}

	for i, row := range spec {
		d, m := dst[i], mask[i]

		for k, c := range row {
			d[k] = complex(real(c)*m[k], imag(c)*m[k])
		}
	}

	return nil
}

// Magnitudes writes |spec[i][k]| into dst[i][k]. dst must have the shape of
// spec. Magnitudes does not allocate.
func Magnitudes(dst [][]float64, spec [][]complex128) error {
	err := sameShape(spec, dst)
	if err != nil {
		return err
	}

	magnitudes(dst, spec)

	return nil
}

func magnitudes(dst [][]float64, spec [][]complex128) {
	for i, row := range spec {
		d := dst[i]

		for k, c := range row {
			d[k] = math.Hypot(real(c), imag(c))
		}
	}
}

// softPair returns the two-source soft masks a^p/(a^p+b^p) and
// b^p/(a^p+b^p) for non-negative finite a and b; 0/0 splits evenly.
func softPair(a, b, power float64) (float64, float64) {
	peak := max(a, b)
	if peak == 0 {
		return 0.5, 0.5
	}

	a = powUnit(a/peak, power)
	b = powUnit(b/peak, power)
	sum := a + b

	return a / sum, b / sum
}

// powUnit returns x^p for x in [0, 1], with fast paths for common powers.
func powUnit(x, p float64) float64 {
	switch p {
	case 1:
		return x
	case 2:
		return x * x
	case math.Inf(1):
		if x == 1 {
			return 1
		}

		return 0
	default:
		return math.Pow(x, p)
	}
}

func checkPower(p float64) error {
	if !(p > 0) {
		return fmt.Errorf("%w: %g, must be > 0", ErrInvalidPower, p)
	}

	return nil
}

// sameShape reports whether b has the same number of rows as a and every row
// the same length.
func sameShape[A, B any](a [][]A, b [][]B) error {
	if len(a) != len(b) {
		return fmt.Errorf("%w: %d frames, want %d", ErrShapeMismatch, len(b), len(a))
	}

	for i := range a {
		if len(a[i]) != len(b[i]) {
			return fmt.Errorf("%w: frame %d has %d bins, want %d", ErrShapeMismatch, i, len(b[i]), len(a[i]))
		}
	}

	return nil
}

// newMatrix returns a frames × bins matrix backed by one contiguous slice.
func newMatrix[T any](frames, bins int) [][]T {
	data := make([]T, frames*bins)
	m := make([][]T, frames)

	for i := range m {
		m[i] = data[i*bins : (i+1)*bins : (i+1)*bins]
	}

	return m
}
