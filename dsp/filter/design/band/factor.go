package band

import (
	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/internal/polyroot"
)

// splitFOSection factors a fourth-order digital section into two cascaded
// biquad sections. Delegates to the shared polyroot package.
func splitFOSection(b, a [5]float64) ([]biquad.Coefficients, error) {
	sections, err := polyroot.SplitFourthOrder(b, a)
	if err != nil {
		return nil, ErrInvalidParams
	}

	return sections, nil
}
