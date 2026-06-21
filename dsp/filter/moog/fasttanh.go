package moog

import "math"

// twoOverLn2 = 2 / ln(2). Used to turn tanh into a base-2 exponential:
// tanh(x) = (e^{2x} - 1) / (e^{2x} + 1) = (2^{(2/ln2)x} - 1) / (2^{(2/ln2)x} + 1).
const twoOverLn2 = 2.8853900817779268

// fastTanh approximates math.Tanh using fastPow2. It is a port of the legacy
// FastTanhContinousError4 routine and is used by the fast-tanh ladder variants.
// The input is clamped to the saturation region to keep the result finite.
func fastTanh(x float64) float64 {
	switch {
	case x >= 20:
		return 1
	case x <= -20:
		return -1
	}

	p := fastPow2(twoOverLn2 * x)

	return (p - 1) / (p + 1)
}

// fastPow2 approximates 2^x. The integer part scales the exponent by building
// the IEEE-754 exponent bits directly (cheaper than math.Ldexp); the fractional
// part uses the continuous-error degree-4 polynomial (CP2ContError4) from the
// legacy DAV_Approximations unit, whose coefficients are the minimax-tuned
// Taylor terms of 2^f for f in [0, 1).
//
// The argument range is bounded by the caller (fastTanh clamps its input), so
// the constructed exponent always stays within the normal float64 range.
func fastPow2(x float64) float64 {
	xi := math.Floor(x)
	f := x - xi

	frac := 1 + f*(0.693118326815805763+
		f*(0.240239617833581720+
			f*(0.0559538423222786241+
				f*0.00960540553453761992)))

	pow := math.Float64frombits(uint64(int64(xi)+1023) << 52) // 2^xi

	return frac * pow
}
