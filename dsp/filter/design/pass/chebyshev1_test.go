package pass

import (
	"testing"
)

// TestChebyshev1LP_Basic verifies basic Chebyshev Type I lowpass functionality
func TestChebyshev1LP_Basic(t *testing.T) {
	sr := 48000.0
	sections := Chebyshev1LP(1000, 4, 1.0, sr)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections for order 4, got %d", len(sections))
	}
	for _, s := range sections {
		assertFiniteCoefficients(t, s)
	}
}

// TestChebyshev1HP_Basic verifies basic Chebyshev Type I highpass functionality
func TestChebyshev1HP_Basic(t *testing.T) {
	sr := 48000.0
	sections := Chebyshev1HP(1000, 4, 1.0, sr)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections for order 4, got %d", len(sections))
	}
	for _, s := range sections {
		assertFiniteCoefficients(t, s)
	}
}
