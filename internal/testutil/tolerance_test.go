package testutil

import (
	"math"
	"testing"
)

func TestMaxAbsDiff(t *testing.T) {
	a := []float64{1.0, 2.0, 3.0}
	b := []float64{1.0, 2.1, 3.0}

	d, err := MaxAbsDiff(a, b)
	if err != nil {
		t.Fatalf("MaxAbsDiff error: %v", err)
	}

	if math.Abs(d-0.1) > 1e-15 {
		t.Fatalf("MaxAbsDiff = %v, want 0.1", d)
	}
}

func TestMaxAbsDiffLengthMismatch(t *testing.T) {
	_, err := MaxAbsDiff([]float64{1}, []float64{1, 2})
	if err == nil {
		t.Fatal("expected error for length mismatch")
	}
}

func TestMaxAbsDiffIdentical(t *testing.T) {
	a := []float64{1, 2, 3}

	d, err := MaxAbsDiff(a, a)
	if err != nil {
		t.Fatalf("MaxAbsDiff error: %v", err)
	}

	if d != 0 {
		t.Fatalf("MaxAbsDiff = %v, want 0 for identical slices", d)
	}
}
