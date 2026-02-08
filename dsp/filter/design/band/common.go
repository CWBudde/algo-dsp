package band

import (
	"errors"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

var ErrInvalidParams = errors.New("band: invalid parameters")

// bandParams validates and converts band filter parameters from Hz to rad/sample.
func bandParams(sampleRate, f0Hz, bandwidthHz float64, order int) (float64, float64, error) {
	if sampleRate <= 0 || f0Hz <= 0 || bandwidthHz <= 0 {
		return 0, 0, ErrInvalidParams
	}

	if f0Hz >= sampleRate*0.5 {
		return 0, 0, ErrInvalidParams
	}

	if order <= 2 || order%2 != 0 {
		return 0, 0, ErrInvalidParams
	}

	fl := f0Hz - bandwidthHz*0.5
	fh := f0Hz + bandwidthHz*0.5

	if fl <= 0 || fh >= sampleRate*0.5 {
		return 0, 0, ErrInvalidParams
	}

	w0 := 2 * math.Pi * f0Hz / sampleRate
	wb := 2 * math.Pi * bandwidthHz / sampleRate

	if !(w0 > 0 && w0 < math.Pi && wb > 0 && wb < math.Pi) {
		return 0, 0, ErrInvalidParams
	}

	return w0, wb, nil
}

// passthroughSections returns a single passthrough section (unity gain).
func passthroughSections() []biquad.Coefficients {
	return []biquad.Coefficients{{B0: 1, B1: 0, B2: 0, A1: 0, A2: 0}}
}

// ln10over20 is the precomputed constant ln(10)/20, used to convert dB to
// linear scale via exp(db * ln10/20) instead of the slower pow(10, db/20).
const ln10over20 = 0.11512925464970228 // math.Ln10 / 20.0

func db2Lin(db float64) float64 {
	return math.Exp(db * ln10over20)
}
