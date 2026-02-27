package pass

import (
	"testing"
)

// TestButterworthLP_Basic verifies basic Butterworth lowpass functionality.
func TestButterworthLP_Basic(t *testing.T) {
	sr := 48000.0

	sections := ButterworthLP(1000, 4, sr)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections for order 4, got %d", len(sections))
	}

	for _, s := range sections {
		assertFiniteCoefficients(t, s)
		assertStableSection(t, s)
	}
}

// TestButterworthHP_Basic verifies basic Butterworth highpass functionality.
func TestButterworthHP_Basic(t *testing.T) {
	sr := 48000.0

	sections := ButterworthHP(1000, 4, sr)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections for order 4, got %d", len(sections))
	}

	for _, s := range sections {
		assertFiniteCoefficients(t, s)
		assertStableSection(t, s)
	}
}
