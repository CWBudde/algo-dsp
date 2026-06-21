package reverb

import (
	"math"
	"testing"
)

func TestNewConvolutionReverbValidation(t *testing.T) {
	if _, err := NewConvolutionReverb(nil, 6); err == nil {
		t.Error("expected error for nil kernel")
	}

	if _, err := NewConvolutionReverb([]float64{}, 6); err == nil {
		t.Error("expected error for empty kernel")
	}
}

// TestConvolutionReverbImpulseResponse drives a unit impulse through the reverb
// with a wet-only mix; the output must reproduce the impulse-response kernel,
// delayed by the engine latency.
func TestConvolutionReverbImpulseResponse(t *testing.T) {
	kernel := []float64{0.5, -0.25, 0.125, 0.0625, -0.03125}

	r, err := NewConvolutionReverb(kernel, 6)
	if err != nil {
		t.Fatalf("NewConvolutionReverb: %v", err)
	}

	r.SetWetDry(1.0, 0.0) // wet only: output is the pure convolution

	latency := r.Latency()

	block := make([]float64, latency+len(kernel)+16)
	block[0] = 1.0 // unit impulse

	if err := r.ProcessInPlace(block); err != nil {
		t.Fatalf("ProcessInPlace: %v", err)
	}

	const tol = 1e-9

	// Before the latency offset the wet output is silent.
	for i := range latency {
		if math.Abs(block[i]) > tol {
			t.Errorf("output[%d] = %g, want 0 (pre-latency)", i, block[i])
		}
	}

	// The kernel appears exactly at the latency offset.
	for k, want := range kernel {
		got := block[latency+k]
		if math.Abs(got-want) > tol {
			t.Errorf("output[%d] = %g, want kernel[%d] = %g", latency+k, got, k, want)
		}
	}
}

// TestConvolutionReverbDryPath verifies the dry mix passes the input through
// unchanged when wet is zero.
func TestConvolutionReverbDryPath(t *testing.T) {
	r, err := NewConvolutionReverb([]float64{1.0, 0.5}, 6)
	if err != nil {
		t.Fatalf("NewConvolutionReverb: %v", err)
	}

	r.SetWetDry(0.0, 1.0) // dry only

	in := []float64{0.3, -0.7, 0.2, 0.9}
	block := append([]float64(nil), in...)

	if err := r.ProcessInPlace(block); err != nil {
		t.Fatalf("ProcessInPlace: %v", err)
	}

	for i := range in {
		if math.Abs(block[i]-in[i]) > 1e-12 {
			t.Errorf("dry output[%d] = %g, want %g", i, block[i], in[i])
		}
	}
}
