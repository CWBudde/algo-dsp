package time

// SummaryStats holds the extrema and energy of a time-domain signal.
// Energy is the sum of squared samples accumulated in float64.
type SummaryStats struct {
	Min    float64
	Max    float64
	Energy float64
}

// Summary computes extrema and energy in one allocation-free pass over float32
// or float64 samples. It avoids the moments and decibel conversions in Calculate
// and does not copy float32 signals into a float64 buffer.
//
// Empty signals return zero-valued statistics. Extrema match Calculate: equal
// values retain the first sample's sign, a NaN first sample makes both extrema
// NaN, and later NaNs do not change the extrema. Any NaN makes Energy NaN;
// infinities make Energy positive infinity unless a NaN is also present.
func Summary[T ~float32 | ~float64](signal []T) SummaryStats {
	if len(signal) == 0 {
		return SummaryStats{}
	}

	minValue, maxValue := float64(signal[0]), float64(signal[0])

	var energy float64

	for _, sample := range signal {
		value := float64(sample)

		energy += value * value
		if value < minValue {
			minValue = value
		}

		if value > maxValue {
			maxValue = value
		}
	}

	return SummaryStats{Min: minValue, Max: maxValue, Energy: energy}
}
