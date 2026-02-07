//go:build !purego && arm64

// Package neon contains arm64 NEON-accelerated vector math kernels.
package neon

// MaxAbs returns the maximum absolute value in x.
// Returns 0 for an empty slice.
func MaxAbs(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}
	return maxAbsNEON(x)
}

//go:noescape
func maxAbsNEON(x []float64) float64
