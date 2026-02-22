package dither

import "testing"

func TestDitherTypeString(t *testing.T) {
	tests := []struct {
		dt   DitherType
		want string
	}{
		{DitherNone, "None"},
		{DitherRectangular, "Rectangular"},
		{DitherTriangular, "Triangular"},
		{DitherGaussian, "Gaussian"},
		{DitherFastGaussian, "FastGaussian"},
		{DitherType(99), "DitherType(99)"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.dt.String(); got != tt.want {
				t.Errorf("DitherType(%d).String() = %q, want %q", tt.dt, got, tt.want)
			}
		})
	}
}

func TestDitherTypeValid(t *testing.T) {
	if !DitherTriangular.Valid() {
		t.Error("DitherTriangular should be valid")
	}
	if DitherType(99).Valid() {
		t.Error("DitherType(99) should be invalid")
	}
}
