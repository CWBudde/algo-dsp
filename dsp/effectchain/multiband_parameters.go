package effectchain

// Shared envelope/gain parameters remain authoritative until perBand is enabled,
// so materializing new descriptor defaults cannot change old presets.
func multibandParameters(sampleRate float64) []ParameterDescriptor {
	parameters := []ParameterDescriptor{
		integerParameter("bands", 3, 2, 4),
		integerParameter("order", 4, 2, 24),
		numberParameter("cross1Hz", 250, 40, sampleRate*0.2),
		numberParameter("cross2Hz", 3000, 140, sampleRate*0.45),
		numberParameter("cross3Hz", 8000, 240, sampleRate*0.49),
		numberParameter("lowThresholdDB", -20, -80, 0),
		numberParameter("lowRatio", 2.5, 1, 20),
		numberParameter("midThresholdDB", -18, -80, 0),
		numberParameter("midRatio", 3, 1, 20),
		numberParameter("upperThresholdDB", -16, -80, 0),
		numberParameter("upperRatio", 3.5, 1, 20),
		numberParameter("highThresholdDB", -14, -80, 0),
		numberParameter("highRatio", 4, 1, 20),
		numberParameter("attackMs", 8, 0.1, 1000),
		numberParameter("releaseMs", 120, 1, 5000),
		numberParameter("kneeDB", 6, 0, 24),
		numberParameter("makeupGainDB", 0, 0, 24),
		booleanParameter("autoMakeup", 0),
		enumParameter("topology", "feedforward", "feedforward", "feedback"),
		booleanParameter("perBand", 0),
		integerParameter("responseBand", 0, 0, 3),
	}
	for _, prefix := range []string{"low", "mid", "upper", "high"} {
		parameters = append(parameters,
			numberParameter(prefix+"AttackMs", 8, 0.1, 1000),
			numberParameter(prefix+"ReleaseMs", 120, 1, 5000),
			numberParameter(prefix+"KneeDB", 6, 0, 24),
			numberParameter(prefix+"MakeupGainDB", 0, 0, 24),
			booleanParameter(prefix+"AutoMakeup", 0),
		)
	}

	return parameters
}

func multibandPrefix(index, bands int) string {
	if index == 0 {
		return "low"
	}

	if index == 1 {
		return "mid"
	}

	if bands == 4 && index == 2 {
		return "upper"
	}

	return "high"
}
