package pass

import (
	"testing"
)

// TestChebyshev2LP_Basic verifies basic Chebyshev Type II lowpass functionality
func TestChebyshev2LP_Basic(t *testing.T) {
	sr := 48000.0
	sections := Chebyshev2LP(1000, 4, 2.0, sr)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections for order 4, got %d", len(sections))
	}
	for _, s := range sections {
		assertFiniteCoefficients(t, s)
	}
}

// TestChebyshev2HP_Basic verifies basic Chebyshev Type II highpass functionality
func TestChebyshev2HP_Basic(t *testing.T) {
	sr := 48000.0
	sections := Chebyshev2HP(1000, 4, 2.0, sr)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections for order 4, got %d", len(sections))
	}
	for _, s := range sections {
		assertFiniteCoefficients(t, s)
	}
}
