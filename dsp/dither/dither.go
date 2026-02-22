package dither

import "fmt"

// DitherType selects the probability distribution used for dither noise.
type DitherType int

const (
	// DitherNone applies no dither (plain rounding/truncation).
	DitherNone DitherType = iota
	// DitherRectangular uses a uniform (rectangular) PDF.
	DitherRectangular
	// DitherTriangular uses a triangular PDF (TPDF), the most common choice.
	DitherTriangular
	// DitherGaussian uses an exact Gaussian PDF.
	DitherGaussian
	// DitherFastGaussian uses an approximated Gaussian PDF (sum of uniform draws).
	DitherFastGaussian

	ditherTypeCount // sentinel for validation
)

var ditherTypeNames = [ditherTypeCount]string{
	"None", "Rectangular", "Triangular", "Gaussian", "FastGaussian",
}

// String returns the name of the dither type.
func (dt DitherType) String() string {
	if dt >= 0 && dt < ditherTypeCount {
		return ditherTypeNames[dt]
	}
	return fmt.Sprintf("DitherType(%d)", dt)
}

// Valid reports whether dt is a known dither type.
func (dt DitherType) Valid() bool {
	return dt >= 0 && dt < ditherTypeCount
}
