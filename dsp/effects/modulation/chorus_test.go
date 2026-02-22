package modulation

import (
	"math"
	"testing"
)

func TestChorusProcessInPlaceMatchesSample(t *testing.T) {
	c1, err := NewChorus()
	if err != nil {
		t.Fatalf("NewChorus() error = %v", err)
	}

	c2, err := NewChorus()
	if err != nil {
		t.Fatalf("NewChorus() error = %v", err)
	}

	input := make([]float64, 128)
	for i := range input {
		input[i] = math.Sin(2 * math.Pi * float64(i) / 31)
	}

	want := make([]float64, len(input))
	copy(want, input)

	for i := range want {
		want[i] = c1.ProcessSample(want[i])
	}

	got := make([]float64, len(input))
	copy(got, input)
	c2.ProcessInPlace(got)

	for i := range got {
		if diff := math.Abs(got[i] - want[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g diff=%g", i, got[i], want[i], diff)
		}
	}
}

func TestChorusResetRestoresState(t *testing.T) {
	c, err := NewChorus()
	if err != nil {
		t.Fatalf("NewChorus() error = %v", err)
	}

	in := make([]float64, 96)
	in[0] = 1

	out1 := make([]float64, len(in))
	for i := range in {
		out1[i] = c.ProcessSample(in[i])
	}

	c.Reset()

	out2 := make([]float64, len(in))
	for i := range in {
		out2[i] = c.ProcessSample(in[i])
	}

	for i := range out1 {
		if diff := math.Abs(out1[i] - out2[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch after reset: got=%g want=%g diff=%g", i, out2[i], out1[i], diff)
		}
	}
}

func TestChorusDelayIndependentOfRate(t *testing.T) {
	c, err := NewChorus()
	if err != nil {
		t.Fatalf("NewChorus() error = %v", err)
	}

	if err := c.SetDepth(0.003); err != nil {
		t.Fatalf("SetDepth() error = %v", err)
	}

	if err := c.SetSpeedHz(0.25); err != nil {
		t.Fatalf("SetSpeedHz() error = %v", err)
	}

	slowMax := c.maxDelay

	if err := c.SetSpeedHz(2.5); err != nil {
		t.Fatalf("SetSpeedHz() error = %v", err)
	}

	fastMax := c.maxDelay

	if slowMax != fastMax {
		t.Fatalf("max delay should not depend on rate: slow=%d fast=%d", slowMax, fastMax)
	}
}

func TestChorusBaseDelayIsNonZeroByDefault(t *testing.T) {
	c, err := NewChorus()
	if err != nil {
		t.Fatalf("NewChorus() error = %v", err)
	}

	if c.BaseDelay() <= 0 {
		t.Fatalf("base delay must be > 0, got %f", c.BaseDelay())
	}
}
