package interp

import "testing"

func TestHermite4IdentityOnLinearRamp(t *testing.T) {
	xm1, x0, x1, x2 := -1.0, 0.0, 1.0, 2.0
	for _, tc := range []struct {
		t float64
		w float64
	}{
		{t: 0.0, w: 0.0},
		{t: 0.25, w: 0.25},
		{t: 0.5, w: 0.5},
		{t: 1.0, w: 1.0},
	} {
		got := Hermite4(tc.t, xm1, x0, x1, x2)
		if diff := got - tc.w; diff < -1e-12 || diff > 1e-12 {
			t.Fatalf("t=%v: got %v want %v", tc.t, got, tc.w)
		}
	}
}

func TestLagrangeInterpolator(t *testing.T) {
	l1 := NewLagrangeInterpolator(1)
	if got := l1.Interpolate([]float64{2, 4}, 0.25); got != 2.5 {
		t.Fatalf("order1 got %v want 2.5", got)
	}

	l3 := NewLagrangeInterpolator(3)
	got := l3.Interpolate([]float64{0, 1, 2, 3}, 0.5)
	if diff := got - 1.5; diff < -1e-12 || diff > 1e-12 {
		t.Fatalf("order3 got %v want 1.5", got)
	}
}
