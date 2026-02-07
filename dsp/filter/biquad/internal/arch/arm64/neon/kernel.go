//go:build arm64 && !purego

package neon

//go:noescape
func processBlockNEON(
	buf []float64,
	b0, b1, b2 float64,
	a1, a2 float64,
	d0, d1 float64,
) (newD0, newD1 float64)
