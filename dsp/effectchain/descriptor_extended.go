package effectchain

import "fmt"

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
		params := []ParameterDescriptor{integerParameter("bands", 4, 1, 8)}

		for band := 1; band <= 8; band++ {
			prefix := fmt.Sprintf("band%d", band)
			frequency := min(80*float64(uint(1)<<uint(band-1)), sr*0.45)

			params = append(params, numberParameter(prefix+"FreqHz", frequency, 20, sr*0.49), numberParameter(prefix+"GainDB", 0, -24, 24), numberParameter(prefix+"Q", 1, 0.2, 8), enumParameter(prefix+"Type", "peak", "peak", "lowshelf", "highshelf"))
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
