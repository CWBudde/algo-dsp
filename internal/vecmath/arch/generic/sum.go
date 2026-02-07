package generic

// Sum returns the sum of all elements in x.
// Returns 0 for an empty slice.
func Sum(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}

	sum := 0.0
	for i := range x {
		sum += x[i]
	}
	return sum
}
