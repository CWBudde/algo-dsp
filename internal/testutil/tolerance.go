package testutil

import (
	"fmt"
	"math"
	"testing"
)

// RequireSliceNearlyEqual fails t if got and want differ in length or if
// any element pair exceeds eps (absolute tolerance).
func RequireSliceNearlyEqual(t *testing.T, got, want []float64, eps float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got %d, want %d", len(got), len(want))
	}
	for i := range got {
		diff := math.Abs(got[i] - want[i])
		if diff > eps {
			t.Fatalf("index %d: got %v, want %v (diff %v > eps %v)", i, got[i], want[i], diff, eps)
		}
	}
}

// RequireFinite fails t if any element is NaN or Inf.
func RequireFinite(t *testing.T, data []float64) {
	t.Helper()
	for i, v := range data {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatalf("index %d: non-finite value %v", i, v)
		}
	}
}

// MaxAbsDiff returns the maximum absolute difference between two slices.
// Returns an error if the slices differ in length.
func MaxAbsDiff(a, b []float64) (float64, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("length mismatch: %d vs %d", len(a), len(b))
	}
	maxDiff := 0.0
	for i := range a {
		d := math.Abs(a[i] - b[i])
		if d > maxDiff {
			maxDiff = d
		}
	}
	return maxDiff, nil
}
