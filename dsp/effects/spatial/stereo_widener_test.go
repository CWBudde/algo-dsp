package spatial

import (
	"math"
	"testing"
)

func TestStereoWidenerInPlaceMatchesProcessStereo(t *testing.T) {
	widener1, err := NewStereoWidener(48000, WithWidth(1.5))
	if err != nil {
		t.Fatalf("NewStereoWidener() error = %v", err)
	}

	widener2, err := NewStereoWidener(48000, WithWidth(1.5))
	if err != nil {
		t.Fatalf("NewStereoWidener() error = %v", err)
	}

	n := 128
	inL := make([]float64, n)
	inR := make([]float64, n)

	for i := range inL {
		inL[i] = math.Sin(2 * math.Pi * float64(i) / 29)
		inR[i] = math.Sin(2*math.Pi*float64(i)/29 + 0.3)
	}

	wantL := make([]float64, n)

	wantR := make([]float64, n)
	for i := range inL {
		wantL[i], wantR[i] = widener1.ProcessStereo(inL[i], inR[i])
	}

	gotL := make([]float64, n)
	gotR := make([]float64, n)

	copy(gotL, inL)
	copy(gotR, inR)

	if err := widener2.ProcessStereoInPlace(gotL, gotR); err != nil {
		t.Fatalf("ProcessStereoInPlace() error = %v", err)
	}

	for i := range gotL {
		if diff := math.Abs(gotL[i] - wantL[i]); diff > 1e-12 {
			t.Fatalf("left sample %d mismatch: got=%g want=%g diff=%g", i, gotL[i], wantL[i], diff)
		}

		if diff := math.Abs(gotR[i] - wantR[i]); diff > 1e-12 {
			t.Fatalf("right sample %d mismatch: got=%g want=%g diff=%g", i, gotR[i], wantR[i], diff)
		}
	}
}

func TestStereoWidenerInterleavedMatchesProcessStereo(t *testing.T) {
	widener1, err := NewStereoWidener(48000, WithWidth(2.0))
	if err != nil {
		t.Fatalf("NewStereoWidener() error = %v", err)
	}

	widener2, err := NewStereoWidener(48000, WithWidth(2.0))
	if err != nil {
		t.Fatalf("NewStereoWidener() error = %v", err)
	}

	n := 64
	inL := make([]float64, n)
	inR := make([]float64, n)

	for i := range inL {
		inL[i] = math.Sin(2 * math.Pi * float64(i) / 17)
		inR[i] = math.Cos(2 * math.Pi * float64(i) / 17)
	}

	wantL := make([]float64, n)

	wantR := make([]float64, n)
	for i := range inL {
		wantL[i], wantR[i] = widener1.ProcessStereo(inL[i], inR[i])
	}

	interleaved := make([]float64, 2*n)
	for i := range inL {
		interleaved[2*i] = inL[i]
		interleaved[2*i+1] = inR[i]
	}

	if err := widener2.ProcessInterleavedInPlace(interleaved); err != nil {
		t.Fatalf("ProcessInterleavedInPlace() error = %v", err)
	}

	for i := range n {
		if diff := math.Abs(interleaved[2*i] - wantL[i]); diff > 1e-12 {
			t.Fatalf("left sample %d mismatch: got=%g want=%g", i, interleaved[2*i], wantL[i])
		}

		if diff := math.Abs(interleaved[2*i+1] - wantR[i]); diff > 1e-12 {
			t.Fatalf("right sample %d mismatch: got=%g want=%g", i, interleaved[2*i+1], wantR[i])
		}
	}
}

func TestStereoWidenerWidthOnePassthrough(t *testing.T) {
	w, err := NewStereoWidener(48000, WithWidth(1.0))
	if err != nil {
		t.Fatalf("NewStereoWidener() error = %v", err)
	}

	tests := []struct {
		left, right float64
	}{
		{0.5, -0.3},
		{1.0, 1.0},
		{-1.0, 1.0},
		{0.0, 0.0},
		{0.7, 0.2},
	}

	for _, tt := range tests {
		outL, outR := w.ProcessStereo(tt.left, tt.right)
		if diff := math.Abs(outL - tt.left); diff > 1e-12 {
			t.Errorf("width=1 left: got=%g want=%g", outL, tt.left)
		}

		if diff := math.Abs(outR - tt.right); diff > 1e-12 {
			t.Errorf("width=1 right: got=%g want=%g", outR, tt.right)
		}
	}
}

func TestStereoWidenerWidthZeroMono(t *testing.T) {
	w, err := NewStereoWidener(48000, WithWidth(0.0))
	if err != nil {
		t.Fatalf("NewStereoWidener() error = %v", err)
	}

	tests := []struct {
		left, right float64
		wantMono    float64
	}{
		{0.5, -0.3, 0.1},
		{1.0, 1.0, 1.0},
		{-1.0, 1.0, 0.0},
		{0.8, 0.2, 0.5},
	}

	for _, tt := range tests {
		outL, outR := w.ProcessStereo(tt.left, tt.right)
		if diff := math.Abs(outL - tt.wantMono); diff > 1e-12 {
			t.Errorf("width=0 left: in=(%g,%g) got=%g want=%g", tt.left, tt.right, outL, tt.wantMono)
		}

		if diff := math.Abs(outR - tt.wantMono); diff > 1e-12 {
			t.Errorf("width=0 right: in=(%g,%g) got=%g want=%g", tt.left, tt.right, outR, tt.wantMono)
		}
	}
}

func TestStereoWidenerWidthTwoDoublesWidth(t *testing.T) {
	w, err := NewStereoWidener(48000, WithWidth(2.0))
	if err != nil {
		t.Fatalf("NewStereoWidener() error = %v", err)
	}

	left, right := 0.8, 0.2
	outL, outR := w.ProcessStereo(left, right)

	mid := (left + right) * 0.5
	side := (left - right) * 0.5
	expectL := mid + side*2
	expectR := mid - side*2

	if diff := math.Abs(outL - expectL); diff > 1e-12 {
		t.Errorf("width=2 left: got=%g want=%g", outL, expectL)
	}

	if diff := math.Abs(outR - expectR); diff > 1e-12 {
		t.Errorf("width=2 right: got=%g want=%g", outR, expectR)
	}
}

func TestStereoWidenerMonoInputUnchanged(t *testing.T) {
	// A mono signal (L == R) has no side component; any width should leave
	// the signal unchanged.
	for _, width := range []float64{0, 0.5, 1, 2, 4} {
		w, err := NewStereoWidener(48000, WithWidth(width))
		if err != nil {
			t.Fatalf("NewStereoWidener(width=%g) error = %v", width, err)
		}

		outL, outR := w.ProcessStereo(0.6, 0.6)
		if diff := math.Abs(outL - 0.6); diff > 1e-12 {
			t.Errorf("width=%g mono left: got=%g want=0.6", width, outL)
		}

		if diff := math.Abs(outR - 0.6); diff > 1e-12 {
			t.Errorf("width=%g mono right: got=%g want=0.6", width, outR)
		}
	}
}

func TestStereoWidenerResetRestoresState(t *testing.T) {
	widener, err := NewStereoWidener(48000, WithWidth(1.5), WithBassMonoFreq(120))
	if err != nil {
		t.Fatalf("NewStereoWidener() error = %v", err)
	}

	n := 96
	inL := make([]float64, n)
	inR := make([]float64, n)

	for i := range inL {
		inL[i] = math.Sin(2 * math.Pi * float64(i) / 23)
		inR[i] = math.Sin(2*math.Pi*float64(i)/23 + 0.5)
	}

	outL1 := make([]float64, n)

	outR1 := make([]float64, n)
	for i := range inL {
		outL1[i], outR1[i] = widener.ProcessStereo(inL[i], inR[i])
	}

	widener.Reset()

	outL2 := make([]float64, n)

	outR2 := make([]float64, n)
	for i := range inL {
		outL2[i], outR2[i] = widener.ProcessStereo(inL[i], inR[i])
	}

	for i := range outL1 {
		if diff := math.Abs(outL1[i] - outL2[i]); diff > 1e-12 {
			t.Fatalf("left sample %d mismatch after reset: got=%g want=%g", i, outL2[i], outL1[i])
		}

		if diff := math.Abs(outR1[i] - outR2[i]); diff > 1e-12 {
			t.Fatalf("right sample %d mismatch after reset: got=%g want=%g", i, outR2[i], outR1[i])
		}
	}
}

func TestStereoWidenerBassMonoCollapsesLow(t *testing.T) {
	// With bass mono enabled, a pure low-frequency stereo signal should
	// become approximately mono over time (once the filter settles).
	// Use width=1 so the side residual is not amplified beyond the
	// natural filter roll-off.
	const sampleRate = 48000.0

	w, err := NewStereoWidener(sampleRate, WithWidth(1.0), WithBassMonoFreq(200))
	if err != nil {
		t.Fatalf("NewStereoWidener() error = %v", err)
	}

	// Generate a 20 Hz sine with opposite polarity in L and R (pure side).
	// 20 Hz is far enough below the 200 Hz crossover (~3.3 octaves) that the
	// 2nd-order Butterworth HP attenuates it by ~40 dB.
	n := 9600 // 200 ms at 48 kHz — enough for filter settling
	outL := make([]float64, n)

	outR := make([]float64, n)
	for i := range n {
		phase := 2 * math.Pi * 20 * float64(i) / sampleRate
		l := math.Sin(phase)
		r := -math.Sin(phase) // opposite polarity = pure side
		outL[i], outR[i] = w.ProcessStereo(l, r)
	}

	// After filter settling (last 25%), L and R should be nearly identical
	// because the bass has been collapsed to mono.
	start := n * 75 / 100
	for i := start; i < n; i++ {
		diff := math.Abs(outL[i] - outR[i])
		if diff > 0.02 {
			t.Fatalf("sample %d: bass mono not effective: L=%g R=%g diff=%g", i, outL[i], outR[i], diff)
		}
	}
}

func TestStereoWidenerBassMonoPreservesHigh(t *testing.T) {
	// With bass mono enabled, high-frequency content should still be widened.
	const sampleRate = 48000.0

	widener, err := NewStereoWidener(sampleRate, WithWidth(2.0), WithBassMonoFreq(200))
	if err != nil {
		t.Fatalf("NewStereoWidener() error = %v", err)
	}

	// Generate a 5 kHz sine with stereo spread.
	n := 4800
	outL := make([]float64, n)

	outR := make([]float64, n)
	for i := range n {
		phase := 2 * math.Pi * 5000 * float64(i) / sampleRate
		l := math.Sin(phase)
		r := math.Sin(phase + 0.5) // offset phase = stereo content
		outL[i], outR[i] = widener.ProcessStereo(l, r)
	}

	// After settling, L and R should differ (stereo image preserved/widened).
	start := n * 80 / 100
	maxDiff := 0.0

	for i := start; i < n; i++ {
		diff := math.Abs(outL[i] - outR[i])
		if diff > maxDiff {
			maxDiff = diff
		}
	}

	if maxDiff < 0.1 {
		t.Fatalf("high frequency stereo image collapsed: maxDiff=%g", maxDiff)
	}
}

func TestStereoWidenerValidation(t *testing.T) {
	// Invalid sample rate.
	if _, err := NewStereoWidener(0); err == nil {
		t.Fatal("expected error for zero sample rate")
	}

	if _, err := NewStereoWidener(-1); err == nil {
		t.Fatal("expected error for negative sample rate")
	}

	if _, err := NewStereoWidener(math.NaN()); err == nil {
		t.Fatal("expected error for NaN sample rate")
	}

	if _, err := NewStereoWidener(math.Inf(1)); err == nil {
		t.Fatal("expected error for Inf sample rate")
	}

	// Invalid width.
	if _, err := NewStereoWidener(48000, WithWidth(-0.1)); err == nil {
		t.Fatal("expected error for negative width")
	}

	if _, err := NewStereoWidener(48000, WithWidth(5)); err == nil {
		t.Fatal("expected error for width > max")
	}

	if _, err := NewStereoWidener(48000, WithWidth(math.NaN())); err == nil {
		t.Fatal("expected error for NaN width")
	}

	// Invalid bass mono freq.
	if _, err := NewStereoWidener(48000, WithBassMonoFreq(10)); err == nil {
		t.Fatal("expected error for bass mono freq below min")
	}

	if _, err := NewStereoWidener(48000, WithBassMonoFreq(600)); err == nil {
		t.Fatal("expected error for bass mono freq above max")
	}

	if _, err := NewStereoWidener(48000, WithBassMonoFreq(math.NaN())); err == nil {
		t.Fatal("expected error for NaN bass mono freq")
	}

	// 0 is valid (disables bass mono).
	if _, err := NewStereoWidener(48000, WithBassMonoFreq(0)); err != nil {
		t.Fatalf("unexpected error for bass mono freq 0: %v", err)
	}
}

func TestStereoWidenerSetterValidation(t *testing.T) {
	w, err := NewStereoWidener(48000)
	if err != nil {
		t.Fatalf("NewStereoWidener() error = %v", err)
	}

	if err := w.SetWidth(-1); err == nil {
		t.Fatal("SetWidth: expected error for negative width")
	}

	if err := w.SetWidth(5); err == nil {
		t.Fatal("SetWidth: expected error for width > max")
	}

	if err := w.SetWidth(2); err != nil {
		t.Fatalf("SetWidth(2) unexpected error: %v", err)
	}

	if w.Width() != 2 {
		t.Fatalf("Width() = %g, want 2", w.Width())
	}

	if err := w.SetSampleRate(0); err == nil {
		t.Fatal("SetSampleRate: expected error for zero")
	}

	if err := w.SetSampleRate(44100); err != nil {
		t.Fatalf("SetSampleRate(44100) unexpected error: %v", err)
	}

	if w.SampleRate() != 44100 {
		t.Fatalf("SampleRate() = %g, want 44100", w.SampleRate())
	}

	if err := w.SetBassMonoFreq(10); err == nil {
		t.Fatal("SetBassMonoFreq: expected error for freq below min")
	}

	if err := w.SetBassMonoFreq(100); err != nil {
		t.Fatalf("SetBassMonoFreq(100) unexpected error: %v", err)
	}

	if w.BassMonoFreq() != 100 {
		t.Fatalf("BassMonoFreq() = %g, want 100", w.BassMonoFreq())
	}

	// Disable bass mono.
	if err := w.SetBassMonoFreq(0); err != nil {
		t.Fatalf("SetBassMonoFreq(0) unexpected error: %v", err)
	}

	if w.BassMonoFreq() != 0 {
		t.Fatalf("BassMonoFreq() = %g, want 0", w.BassMonoFreq())
	}
}

func TestStereoWidenerInterleavedOddLength(t *testing.T) {
	w, err := NewStereoWidener(48000)
	if err != nil {
		t.Fatalf("NewStereoWidener() error = %v", err)
	}

	buf := make([]float64, 3)
	if err := w.ProcessInterleavedInPlace(buf); err == nil {
		t.Fatal("ProcessInterleavedInPlace: expected error for odd-length buffer")
	}
}

func TestStereoWidenerMismatchedBufferLengths(t *testing.T) {
	w, err := NewStereoWidener(48000)
	if err != nil {
		t.Fatalf("NewStereoWidener() error = %v", err)
	}

	left := make([]float64, 4)

	right := make([]float64, 5)
	if err := w.ProcessStereoInPlace(left, right); err == nil {
		t.Fatal("ProcessStereoInPlace: expected error for mismatched lengths")
	}
}

func TestStereoWidenerSymmetry(t *testing.T) {
	// Widening should be symmetric: swapping L and R inputs should swap
	// L and R outputs.
	w, err := NewStereoWidener(48000, WithWidth(1.8))
	if err != nil {
		t.Fatalf("NewStereoWidener() error = %v", err)
	}

	left, right := 0.7, -0.4
	outL, outR := w.ProcessStereo(left, right)

	w.Reset()
	swappedL, swappedR := w.ProcessStereo(right, left)

	if diff := math.Abs(outL - swappedR); diff > 1e-12 {
		t.Errorf("symmetry broken: outL=%g swappedR=%g", outL, swappedR)
	}

	if diff := math.Abs(outR - swappedL); diff > 1e-12 {
		t.Errorf("symmetry broken: outR=%g swappedL=%g", outR, swappedL)
	}
}

func TestStereoWidenerEnergyPreservation(t *testing.T) {
	// At width=1, input energy should equal output energy.
	w, err := NewStereoWidener(48000, WithWidth(1.0))
	if err != nil {
		t.Fatalf("NewStereoWidener() error = %v", err)
	}

	n := 256
	inputEnergy := 0.0
	outputEnergy := 0.0

	for i := range n {
		l := math.Sin(2 * math.Pi * float64(i) / 31)
		r := math.Cos(2 * math.Pi * float64(i) / 31)
		inputEnergy += l*l + r*r
		outL, outR := w.ProcessStereo(l, r)
		outputEnergy += outL*outL + outR*outR
	}

	ratio := outputEnergy / inputEnergy
	if math.Abs(ratio-1) > 1e-10 {
		t.Errorf("energy ratio at width=1: %g (want ~1)", ratio)
	}
}
