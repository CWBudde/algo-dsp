package effects

import "math"

// DAVQuarticSaturation applies the signed quartic saturation used by the
// Delphi ASIO & VST OpAmp and Scary effects. The curve approaches ±1.
func DAVQuarticSaturation(x float64) float64 {
	a := math.Abs(x)
	if a >= 1<<16 {
		// The exact result rounds to unity here. Avoid overflow in a^4.
		return math.Copysign(1, x)
	}

	q := a * (12 + a*(6+a*(3+a)))

	result := q / (q + 24)
	if x < 0 {
		return -result
	}

	return result
}
