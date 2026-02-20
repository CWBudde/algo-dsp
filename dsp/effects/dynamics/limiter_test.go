package dynamics

import (
	"math"
	"testing"
)

func TestLimiterProcessInPlaceMatchesSample(t *testing.T) {
	l1, err := NewLimiter(48000)
	if err != nil {
		t.Fatalf("NewLimiter() error = %v", err)
	}
	l2, err := NewLimiter(48000)
	if err != nil {
		t.Fatalf("NewLimiter() error = %v", err)
	}

	if err := l1.SetThreshold(-3); err != nil {
		t.Fatalf("SetThreshold() error = %v", err)
	}
	if err := l1.SetRelease(80); err != nil {
		t.Fatalf("SetRelease() error = %v", err)
	}
	if err := l2.SetThreshold(-3); err != nil {
		t.Fatalf("SetThreshold() error = %v", err)
	}
	if err := l2.SetRelease(80); err != nil {
		t.Fatalf("SetRelease() error = %v", err)
	}

	in := []float64{0.0, 0.1, 0.5, 0.95, 1.3, -1.1, 0.8, -0.6, 0.2, 0.0}

	want := make([]float64, len(in))
	for i, x := range in {
		want[i] = l1.ProcessSample(x)
	}

	got := append([]float64(nil), in...)
	l2.ProcessInPlace(got)

	for i := range got {
		if math.Abs(got[i]-want[i]) > 1e-12 {
			t.Fatalf("sample %d: ProcessInPlace() = %.15f, ProcessSample() = %.15f", i, got[i], want[i])
		}
	}
}
