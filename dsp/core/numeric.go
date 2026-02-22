package core

import "math"

const defaultEpsilon = 1e-12

// Clamp limits value to the inclusive range [min, max].
func Clamp(value, min, max float64) float64 {
	if min > max {
		min, max = max, min
	}

	if value < min {
		return min
	}

	if value > max {
		return max
	}

	return value
}

// NearlyEqual reports whether a and b are equal within eps.
func NearlyEqual(a, b, eps float64) bool {
	if eps <= 0 {
		eps = defaultEpsilon
	}

	diff := math.Abs(a - b)
	if diff <= eps {
		return true
	}

	largest := math.Max(math.Abs(a), math.Abs(b))
	if largest == 0 {
		return diff <= eps
	}

	return diff/largest <= eps
}

// FlushDenormals converts tiny denormal-like values to exact zero.
// This can reduce denormal-related CPU slowdowns in hot DSP loops.
func FlushDenormals(x float64) float64 {
	const epsilon = 1e-30
	if x > -epsilon && x < epsilon {
		return 0
	}

	return x
}

// DBToLinear converts dB to linear amplitude (20*log10 convention).
func DBToLinear(db float64) float64 {
	return math.Pow(10, db/20)
}

// LinearToDB converts linear amplitude to dB (20*log10 convention).
// Returns -Inf for zero and NaN for negative values.
func LinearToDB(linear float64) float64 {
	if linear < 0 {
		return math.NaN()
	}

	if linear == 0 {
		return math.Inf(-1)
	}

	return 20 * math.Log10(linear)
}

// DBPowerToLinear converts dB to linear power (10*log10 convention).
func DBPowerToLinear(db float64) float64 {
	return math.Pow(10, db/10)
}

// LinearPowerToDB converts linear power to dB (10*log10 convention).
// Returns -Inf for zero and NaN for negative values.
func LinearPowerToDB(power float64) float64 {
	if power < 0 {
		return math.NaN()
	}

	if power == 0 {
		return math.Inf(-1)
	}

	return 10 * math.Log10(power)
}
