package effectchain

import (
	"fmt"
	"math"
)

var graphicCenters = [...]float64{31.5, 63, 125, 250, 500, 1000, 2000, 4000, 8000, 16000}

func extendedDescriptors(sr float64) []Descriptor {
	result := []Descriptor{
		descriptor("auto-wah", "Auto-wah", "Modulation", "generic", numberParameter("minFreqHz", 300, 20, sr*0.45), numberParameter("maxFreqHz", 2200, 21, sr*0.49), numberParameter("q", 0.8, 0.1, 20), numberParameter("sensitivity", 2, 0, 20), numberParameter("attackMs", 2, 0.1, 1000), numberParameter("releaseMs", 80, 1, 5000), numberParameter("mix", 1, 0, 1)),
		descriptor("frequency-shifter", "Frequency shifter", "Modulation", "generic", numberParameter("shiftHz", 100, 0, sr*0.49), enumParameter("direction", "up", "up", "down"), numberParameter("mix", 1, 0, 1)),
		descriptor("panner", "Stereo panner", "Spatial", "generic", numberParameter("position", 0, -1, 1), enumParameter("law", "equal-power", "equal-power", "linear", "compromise"), numberParameter("autoPanRateHz", 0, 0, 20), numberParameter("autoPanDepth", 0, 0, 1)),
		descriptor("haas", "Haas delay", "Spatial", "generic", numberParameter("delayMs", 15, 0, 40), enumParameter("channel", "right", "right", "left")),
		descriptor("crosstalk", "Crosstalk simulator", "Spatial", "generic", numberParameter("diameter", 0.175, 0.01, 1), numberParameter("speedOfSound", 343, 100, 1000), numberParameter("crossfeedMix", 0.2, 0, 1), booleanParameter("polarityInvert", 0), enumParameter("preset", "handcrafted", "handcrafted", "ircam", "hdphx")),
		descriptor("filter-a-weighting", "A-weighting", "Filters", "eq"),
		descriptor("filter-c-weighting", "C-weighting", "Filters", "eq"),
	}

	for _, id := range []string{"eq-parametric", "dyn-eq"} {
		params := []ParameterDescriptor{integerParameter("bands", 3, 1, 8)}
		if id == "eq-parametric" {
			params[0] = integerParameter("bands", 6, 1, 8)
		} else {
			params = append(params, integerParameter("responseBand", 0, 0, 7))
		}

		for band := 1; band <= 8; band++ {
			prefix := fmt.Sprintf("band%d", band)
			frequency := dynamicEQBandDefault(band, sr)
			kind, q := "peak", 1.0
			choices := []string{"peak", "lowshelf", "highshelf"}

			if id == "eq-parametric" {
				frequency, kind, q = parametricBandDefault(band, sr)

				choices = append(choices, "highpass", "lowpass")
			}

			params = append(params, numberParameter(prefix+"FreqHz", frequency, 20, sr*0.49), numberParameter(prefix+"GainDB", 0, -24, 24), numberParameter(prefix+"Q", q, 0.2, 8), enumParameter(prefix+"Type", kind, choices...))
			if id == "eq-parametric" {
				order := integerParameter(prefix+"Order", 2, 2, 12)
				order.Step = 2
				params = append(params, order)
			}

			if id == "dyn-eq" {
				params = append(params, enumParameter(prefix+"Mode", "downward", "static", "downward", "upward", "upward-below"), numberParameter(prefix+"ThresholdDB", -24, -80, 0), numberParameter(prefix+"Ratio", 2, 1, 20), numberParameter(prefix+"KneeDB", 6, 0, 24), numberParameter(prefix+"AttackMs", 10, 0.1, 1000), numberParameter(prefix+"ReleaseMs", 100, 1, 5000), numberParameter(prefix+"RangeDB", 12, 0, 24))
			}
		}

		name, category := "Parametric EQ", "EQ"
		if id == "dyn-eq" {
			name, category = "Dynamic EQ", "Dynamics"
		}

		result = append(result, descriptor(id, name, category, "eq", params...))
	}

	params := []ParameterDescriptor{integerParameter("order", 4, 4, 12)}
	params[0].Step = 2

	for band, frequency := range graphicCenters {
		p := numberParameter(fmt.Sprintf("gain%dDB", band+1), 0, -24, 24)
		p.Label = fmt.Sprintf("%g Hz gain", min(frequency, sr*0.45))
		params = append(params, p)
	}

	result = append(result, descriptor("eq-graphic", "Graphic EQ", "EQ", "eq", params...))

	return result
}

// The six active bands span roughly equal distances on a logarithmic axis.
// Compress that span for low rates rather than piling upper handles at Nyquist.
func parametricBandDefault(band int, sr float64) (float64, string, float64) {
	frequencies := [...]float64{30, 100, 350, 1200, 4000, 14000, 7000, 18000}
	kinds := [...]string{"highpass", "lowshelf", "peak", "peak", "highshelf", "lowpass", "peak", "peak"}

	hz := frequencies[band-1]
	if sr*0.45 < 14000 && band <= 6 {
		hz = 30 * math.Pow(sr*0.45/30, math.Log(hz/30)/math.Log(14000.0/30))
	}

	q := 1.0
	if kinds[band-1] == "highpass" || kinds[band-1] == "lowpass" {
		q = math.Sqrt(0.5)
	}

	return min(hz, sr*0.45), kinds[band-1], q
}

// The three default dynamic bands cover bass, midrange and treble. Compress
// their logarithmic span at low rates to keep every handle distinct.
func dynamicEQBandDefault(band int, sr float64) float64 {
	frequencies := [...]float64{120, 1000, 8000, 60, 350, 2500, 14000, 18000}

	hz := frequencies[band-1]
	if sr*0.45 < 8000 && band <= 3 {
		hz = 120 * math.Pow(sr*0.45/120, math.Log(hz/120)/math.Log(8000.0/120))
	}

	return min(hz, sr*0.45)
}
