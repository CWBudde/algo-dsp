package spectrum

import (
	"math"
	"testing"
)

func TestMagnitudePhasePower(t *testing.T) {
	bins := []complex128{3 + 4i, -1 - 1i, 0}

	mag := Magnitude(bins)
	if len(mag) != len(bins) {
		t.Fatalf("Magnitude length mismatch: got=%d want=%d", len(mag), len(bins))
	}

	if math.Abs(mag[0]-5) > 1e-12 {
		t.Fatalf("Magnitude[0]=%f want=5", mag[0])
	}

	pow := Power(bins)
	if math.Abs(pow[0]-25) > 1e-12 {
		t.Fatalf("Power[0]=%f want=25", pow[0])
	}

	phase := Phase(bins)
	if math.Abs(phase[0]-math.Atan2(4, 3)) > 1e-12 {
		t.Fatalf("Phase[0]=%f mismatch", phase[0])
	}
}

func TestComplexBinsAdapter(t *testing.T) {
	bins := SliceBins([]complex128{1 + 0i, 0 + 2i})

	mag := MagnitudeBins(bins)
	if len(mag) != 2 || math.Abs(mag[0]-1) > 1e-12 || math.Abs(mag[1]-2) > 1e-12 {
		t.Fatalf("unexpected MagnitudeBins output: %v", mag)
	}
}

func TestUnwrapPhase(t *testing.T) {
	in := []float64{2.8, -2.7, -2.6}

	out := UnwrapPhase(in)
	if len(out) != len(in) {
		t.Fatalf("unwrap length mismatch")
	}

	if out[1] <= out[0] {
		t.Fatalf("expected increasing unwrapped phase: %v", out)
	}

	if math.Abs((out[1]-out[0])-(2*math.Pi-5.5)) > 1e-12 {
		t.Fatalf("unexpected unwrap delta: %f", out[1]-out[0])
	}
}

func TestGroupDelayFromPhaseConstantDelay(t *testing.T) {
	fftSize := 1024
	delaySamples := 12.5
	n := 64

	phase := make([]float64, n)
	for k := range phase {
		w := 2 * math.Pi * float64(k) / float64(fftSize)
		phase[k] = -w * delaySamples
	}

	gd, err := GroupDelayFromPhase(phase, fftSize)
	if err != nil {
		t.Fatalf("GroupDelayFromPhase error: %v", err)
	}

	for i, v := range gd {
		if math.Abs(v-delaySamples) > 1e-9 {
			t.Fatalf("gd[%d]=%f want=%f", i, v, delaySamples)
		}
	}
}

func TestGroupDelaySeconds(t *testing.T) {
	phase := []float64{0, -2 * math.Pi / 8, -2 * 2 * math.Pi / 8}

	groupDelay, err := GroupDelaySeconds(phase, 8, 48000)
	if err != nil {
		t.Fatalf("GroupDelaySeconds error: %v", err)
	}

	if len(groupDelay) != len(phase) {
		t.Fatalf("group delay length mismatch")
	}

	if math.Abs(groupDelay[1]-1.0/48000.0) > 1e-12 {
		t.Fatalf("gd[1]=%e want=%e", groupDelay[1], 1.0/48000.0)
	}
}

func TestGroupDelayErrors(t *testing.T) {
	_, err := GroupDelayFromPhase([]float64{1}, 8)
	if err == nil {
		t.Fatalf("expected error for short phase")
	}

	_, err = GroupDelayFromPhase([]float64{1, 2}, 0)
	if err == nil {
		t.Fatalf("expected error for invalid fft size")
	}

	_, err = GroupDelaySeconds([]float64{1, 2}, 8, 0)
	if err == nil {
		t.Fatalf("expected error for invalid sample rate")
	}
}

func TestInterpolateLinear(t *testing.T) {
	x := []float64{0, 1, 2}
	y := []float64{0, 10, 20}
	q := []float64{-1, 0.5, 2, 3}

	out, err := InterpolateLinear(x, y, q)
	if err != nil {
		t.Fatalf("InterpolateLinear error: %v", err)
	}

	want := []float64{0, 5, 20, 20}
	for i := range want {
		if math.Abs(out[i]-want[i]) > 1e-12 {
			t.Fatalf("out[%d]=%f want=%f", i, out[i], want[i])
		}
	}
}

func TestInterpolateLinearErrors(t *testing.T) {
	if _, err := InterpolateLinear(nil, nil, []float64{1}); err == nil {
		t.Fatalf("expected error for empty x/y")
	}

	if _, err := InterpolateLinear([]float64{0, 1}, []float64{1}, []float64{1}); err == nil {
		t.Fatalf("expected error for mismatch")
	}

	if _, err := InterpolateLinear([]float64{0, 0}, []float64{1, 2}, []float64{1}); err == nil {
		t.Fatalf("expected error for non-monotonic x")
	}
}

func TestSmoothFractionalOctave(t *testing.T) {
	freq := []float64{100, 125, 160, 200, 250, 315}
	vals := []float64{1, 1, 9, 1, 1, 1}

	out, err := SmoothFractionalOctave(freq, vals, 1)
	if err != nil {
		t.Fatalf("SmoothFractionalOctave error: %v", err)
	}

	if len(out) != len(vals) {
		t.Fatalf("length mismatch")
	}

	if !(out[2] < vals[2]) {
		t.Fatalf("expected peak smoothing at center: out=%v", out)
	}

	if !(out[1] > vals[1]) {
		t.Fatalf("expected neighboring lift from smoothing: out=%v", out)
	}
}

func TestMagnitudeFromParts(t *testing.T) {
	re := []float64{3, -1, 0}
	im := []float64{4, -1, 0}
	dst := make([]float64, 3)
	MagnitudeFromParts(dst, re, im)

	if math.Abs(dst[0]-5) > 1e-12 {
		t.Fatalf("MagnitudeFromParts[0]=%f want=5", dst[0])
	}

	if math.Abs(dst[1]-math.Sqrt(2)) > 1e-12 {
		t.Fatalf("MagnitudeFromParts[1]=%f want=%f", dst[1], math.Sqrt(2))
	}

	if math.Abs(dst[2]-0) > 1e-12 {
		t.Fatalf("MagnitudeFromParts[2]=%f want=0", dst[2])
	}
}

func TestPowerFromParts(t *testing.T) {
	re := []float64{3, -1, 0}
	im := []float64{4, -1, 0}
	dst := make([]float64, 3)
	PowerFromParts(dst, re, im)

	if math.Abs(dst[0]-25) > 1e-12 {
		t.Fatalf("PowerFromParts[0]=%f want=25", dst[0])
	}

	if math.Abs(dst[1]-2) > 1e-12 {
		t.Fatalf("PowerFromParts[1]=%f want=2", dst[1])
	}

	if math.Abs(dst[2]-0) > 1e-12 {
		t.Fatalf("PowerFromParts[2]=%f want=0", dst[2])
	}
}

func TestSmoothFractionalOctaveErrors(t *testing.T) {
	if _, err := SmoothFractionalOctave(nil, nil, 3); err == nil {
		t.Fatalf("expected error for empty")
	}

	if _, err := SmoothFractionalOctave([]float64{1}, []float64{1, 2}, 3); err == nil {
		t.Fatalf("expected error for mismatch")
	}

	if _, err := SmoothFractionalOctave([]float64{1}, []float64{1}, 0); err == nil {
		t.Fatalf("expected error for invalid fraction")
	}

	if _, err := SmoothFractionalOctave([]float64{0, 2}, []float64{1, 2}, 3); err == nil {
		t.Fatalf("expected error for non-positive frequency")
	}

	if _, err := SmoothFractionalOctave([]float64{2, 2}, []float64{1, 2}, 3); err == nil {
		t.Fatalf("expected error for non-increasing frequency")
	}
}
