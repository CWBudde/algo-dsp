package design

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design/band"
	"github.com/cwbudde/algo-dsp/dsp/filter/design/pass"
	"github.com/cwbudde/algo-dsp/dsp/filter/design/shelving"
)

// ParametricBand designs one peak, lowshelf, highshelf, highpass or lowpass band
// at an even total digital filter order from 2 to 12. Order two preserves the
// RBJ designs, including their Q/resonance control. Higher orders use Butterworth
// responses: Q controls peak bandwidth (freqHz/Q, bounded away from DC/Nyquist)
// and is unused for passes/shelves. Pass gain is always ignored. Higher-order
// shelves cross at half gain at freqHz, and peak center gain stays gainDB.
// Sample rate and frequency must be finite, positive and below Nyquist; gain
// must be finite within ±24 dB and Q within [0.2, 8].
func ParametricBand(sampleRate, freqHz, gainDB, q float64, kind string, order int) ([]biquad.Coefficients, error) {
	if _, ok := normalizedW0(freqHz, sampleRate); !ok ||
		math.IsNaN(gainDB) || math.IsInf(gainDB, 0) || math.Abs(gainDB) > 24 ||
		math.IsNaN(q) || q < .2 || q > 8 || order < 2 || order > 12 || order%2 != 0 {
		return nil, fmt.Errorf("parametric band: invalid parameters")
	}

	if order == 2 {
		var c biquad.Coefficients

		switch kind {
		case "highpass":
			c = Highpass(freqHz, q, sampleRate)
		case "lowpass":
			c = Lowpass(freqHz, q, sampleRate)
		case "lowshelf":
			c = LowShelf(freqHz, gainDB, q, sampleRate)
		case "highshelf":
			c = HighShelf(freqHz, gainDB, q, sampleRate)
		case "peak":
			c = Peak(freqHz, gainDB, q, sampleRate)
		default:
			return nil, fmt.Errorf("parametric band: unknown type %q", kind)
		}

		return []biquad.Coefficients{c}, nil
	}

	switch kind {
	case "highpass":
		return pass.ButterworthHP(freqHz, order, sampleRate), nil
	case "lowpass":
		return pass.ButterworthLP(freqHz, order, sampleRate), nil
	case "peak":
		width := min(freqHz/q, 1.8*min(freqHz, sampleRate/2-freqHz))
		return band.ButterworthPeak(sampleRate, freqHz, width, gainDB, order)
	case "lowshelf", "highshelf":
		signedGain := gainDB
		if kind == "lowshelf" {
			signedGain = -gainDB
		}

		warped := math.Tan(math.Pi*freqHz/sampleRate) * math.Pow(10, signedGain/(40*float64(order)))

		center := sampleRate / math.Pi * math.Atan(warped)
		if kind == "lowshelf" {
			return shelving.ButterworthLowShelf(sampleRate, center, gainDB, order)
		}

		return shelving.ButterworthHighShelf(sampleRate, center, gainDB, order)
	default:
		return nil, fmt.Errorf("parametric band: unknown type %q", kind)
	}
}
