package effects

import "math"

// DAVContinuousTanh4 evaluates the DAV fourth-degree continuous exp2-based
// tanh approximation with the original binary32 rounding. Large arguments
// take the saturation limit, and NaN propagates.
func DAVContinuousTanh4(input float32) float32 {
	const (
		twiceLog2E = float64(float32(2 * math.Log2E))
		a          = float64(float32(.693118326815805763))
		b          = float64(float32(.240239617833581720))
		c          = float64(float32(.0559538423222786241))
		d          = float64(float32(.00960540553453761992))
	)
	x := float64(input)
	if math.IsNaN(x) {
		return input
	}
	if math.Abs(x) >= 32 {
		return float32(math.Copysign(1, x))
	}
	exponent := float64(float32(twiceLog2E * x))
	integral := math.RoundToEven(exponent)
	fraction := exponent - integral
	power := float64(float32(math.Ldexp(1+fraction*(a+fraction*(b+fraction*(c+fraction*d))), int(integral))))
	return float32((power - 1) / (power + 1))
}
