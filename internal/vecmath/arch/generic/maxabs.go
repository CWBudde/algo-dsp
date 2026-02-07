package generic

import "math"

// MaxAbs returns the maximum absolute value in x.
// Returns 0 for an empty slice.
func MaxAbs(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}

	max := math.Abs(x[0])
	for i := 1; i < len(x); i++ {
		v := math.Abs(x[i])
		if v > max {
			max = v
		}
	}
	return max
}
