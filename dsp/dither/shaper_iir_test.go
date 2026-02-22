package dither

import (
	"math"
	"testing"
)

func TestIIRShelfShaperCreation(t *testing.T) {
	shaper, err := NewIIRShelfShaper(10000, 44100)
	if err != nil {
		t.Fatal(err)
	}

	var _ NoiseShaper = shaper // verify interface
}

func TestIIRShelfShaperValidation(t *testing.T) {
	tests := []struct {
		name string
		freq float64
		sr   float64
	}{
		{"zero freq", 0, 44100},
		{"negative freq", -100, 44100},
		{"zero sr", 10000, 0},
		{"negative sr", 10000, -44100},
		{"NaN freq", math.NaN(), 44100},
		{"Inf freq", math.Inf(1), 44100},
		{"NaN sr", 10000, math.NaN()},
		{"Inf sr", 10000, math.Inf(1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewIIRShelfShaper(tt.freq, tt.sr)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestIIRShelfShaperPassthrough(t *testing.T) {
	shaper, _ := NewIIRShelfShaper(10000, 44100)

	// With no previous error, Shape should return input unchanged.
	got := shaper.Shape(1.0)
	if got != 1.0 {
		t.Errorf("first sample: got %v, want 1.0", got)
	}
}

func TestIIRShelfShaperReset(t *testing.T) {
	shaper, _ := NewIIRShelfShaper(10000, 44100)

	for range 100 {
		shaper.Shape(0.5)
		shaper.RecordError(0.01)
	}

	shaper.Reset()

	// After reset with zero error, Shape(0) should return 0.
	got := shaper.Shape(0)
	if got != 0 {
		t.Errorf("after reset: Shape(0) = %v, want 0", got)
	}
}

func TestIIRShelfShaperStability(t *testing.T) {
	shaper, _ := NewIIRShelfShaper(10000, 44100)

	for idx := range 10000 {
		val := shaper.Shape(0.5)
		shaper.RecordError(0.01 * float64(idx%10))

		if math.IsNaN(val) || math.IsInf(val, 0) {
			t.Fatalf("sample %d: got %v", idx, val)
		}
	}
}

func TestIIRShelfShaperShapesNoise(t *testing.T) {
	// Verify that the IIR shaper actually modifies the signal when
	// there is non-zero error feedback.
	shaper, _ := NewIIRShelfShaper(10000, 44100)

	// Feed a constant error and verify output differs from input.
	shaper.RecordError(0.5)

	got := shaper.Shape(1.0)
	if got == 1.0 {
		t.Error("IIR shaper should modify input when error is non-zero")
	}
}
