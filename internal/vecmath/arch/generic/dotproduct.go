package generic

// DotProduct returns the dot product of a and b: sum(a[i] * b[i]).
// Returns 0 if slices are empty or have different lengths.
// Only the minimum length of the two slices is used.
func DotProduct(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return 0
	}

	sum := 0.0
	for i := 0; i < n; i++ {
		sum += a[i] * b[i]
	}
	return sum
}
