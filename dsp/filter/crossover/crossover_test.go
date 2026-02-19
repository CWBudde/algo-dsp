package crossover

import (
	"math"
	"math/cmplx"
	"testing"
)

const tolerance = 0.05 // dB

// TestNew_ValidParameters checks successful construction.
func TestNew_ValidParameters(t *testing.T) {
	tests := []struct {
		freq  float64
		order int
		sr    float64
	}{
		{1000, 2, 48000},
		{1000, 4, 48000},
		{1000, 8, 48000},
		{500, 4, 44100},
		{100, 12, 96000},
	}
	for _, tt := range tests {
		xo, err := New(tt.freq, tt.order, tt.sr)
		if err != nil {
			t.Errorf("New(%.0f, %d, %.0f): unexpected error: %v", tt.freq, tt.order, tt.sr, err)
			continue
		}
		if xo.Freq() != tt.freq {
			t.Errorf("Freq() = %v, want %v", xo.Freq(), tt.freq)
		}
		if xo.Order() != tt.order {
			t.Errorf("Order() = %v, want %v", xo.Order(), tt.order)
		}
		if xo.SampleRate() != tt.sr {
			t.Errorf("SampleRate() = %v, want %v", xo.SampleRate(), tt.sr)
		}
	}
}

// TestNew_InvalidParameters checks all error paths.
func TestNew_InvalidParameters(t *testing.T) {
	tests := []struct {
		name  string
		freq  float64
		order int
		sr    float64
	}{
		{"odd order", 1000, 3, 48000},
		{"zero order", 1000, 0, 48000},
		{"negative order", 1000, -2, 48000},
		{"zero freq", 0, 4, 48000},
		{"negative freq", -100, 4, 48000},
		{"freq at Nyquist", 24000, 4, 48000},
		{"freq above Nyquist", 25000, 4, 48000},
		{"zero sample rate", 1000, 4, 0},
		{"negative sample rate", 1000, 4, -44100},
	}
	for _, tt := range tests {
		_, err := New(tt.freq, tt.order, tt.sr)
		if err == nil {
			t.Errorf("%s: expected error, got nil", tt.name)
		}
	}
}

// TestCrossover_AllpassFrequencyResponse verifies LP + HP = allpass
// by checking the frequency response magnitude at many frequencies.
func TestCrossover_AllpassFrequencyResponse(t *testing.T) {
	sr := 48000.0
	orders := []int{2, 4, 8, 12}

	for _, order := range orders {
		xo, err := New(1000, order, sr)
		if err != nil {
			t.Fatalf("LR%d: %v", order, err)
		}

		lpChain := xo.LP()
		hpChain := xo.HP()

		freqs := []float64{20, 50, 100, 200, 500, 1000, 2000, 5000, 10000, 20000}
		for _, f := range freqs {
			if f >= sr/2 {
				continue
			}
			lpH := lpChain.Response(f, sr)
			hpH := hpChain.Response(f, sr)
			sumMag := 20 * math.Log10(cmplx.Abs(lpH+hpH))

			if math.Abs(sumMag) > tolerance {
				t.Errorf("LR%d sum at %.0f Hz: %.4f dB (want 0 ±%.2f dB)", order, f, sumMag, tolerance)
				break
			}
		}
	}
}

// TestCrossover_ProcessSample verifies sample-by-sample processing works.
func TestCrossover_ProcessSample(t *testing.T) {
	xo, err := New(1000, 4, 48000)
	if err != nil {
		t.Fatal(err)
	}

	// Feed impulse and verify outputs are finite.
	lo, hi := xo.ProcessSample(1.0)
	if math.IsNaN(lo) || math.IsInf(lo, 0) {
		t.Errorf("lo is not finite: %v", lo)
	}
	if math.IsNaN(hi) || math.IsInf(hi, 0) {
		t.Errorf("hi is not finite: %v", hi)
	}

	// Feed zeros — outputs should decay toward zero.
	for i := 0; i < 1000; i++ {
		lo, hi = xo.ProcessSample(0.0)
	}
	if math.Abs(lo) > 1e-10 {
		t.Errorf("lo should have decayed: %v", lo)
	}
	if math.Abs(hi) > 1e-10 {
		t.Errorf("hi should have decayed: %v", hi)
	}
}

// TestCrossover_AllpassImpulseSum verifies the sum of LP + HP impulse
// responses equals an allpass impulse response (energy preserved, not
// necessarily 1.0 at sample 0).
func TestCrossover_AllpassImpulseSum(t *testing.T) {
	xo, _ := New(1000, 4, 48000)

	// Compute the energy of the sum: should match input energy.
	n := 4096
	sumEnergy := 0.0
	for i := 0; i < n; i++ {
		x := 0.0
		if i == 0 {
			x = 1.0
		}
		lo, hi := xo.ProcessSample(x)
		s := lo + hi
		sumEnergy += s * s
	}

	// For an allpass filter fed an impulse of amplitude 1, total energy = 1.
	if math.Abs(sumEnergy-1.0) > 0.001 {
		t.Errorf("allpass impulse energy = %v, want 1.0", sumEnergy)
	}
}

// TestCrossover_ProcessBlock_Empty verifies that an empty input slice is a no-op.
func TestCrossover_ProcessBlock_Empty(t *testing.T) {
	xo, _ := New(1000, 4, 48000)
	// Must not panic.
	xo.ProcessBlock([]float64{}, []float64{}, []float64{})
}

// TestCrossover_ProcessBlock verifies block processing matches sample-by-sample.
func TestCrossover_ProcessBlock(t *testing.T) {
	sr := 48000.0
	n := 128

	// Create two identical crossovers.
	xoSample, _ := New(1000, 4, sr)
	xoBlock, _ := New(1000, 4, sr)

	// Input: impulse followed by zeros.
	input := make([]float64, n)
	input[0] = 1.0

	// Sample-by-sample.
	loS := make([]float64, n)
	hiS := make([]float64, n)
	for i, x := range input {
		loS[i], hiS[i] = xoSample.ProcessSample(x)
	}

	// Block.
	loB := make([]float64, n)
	hiB := make([]float64, n)
	xoBlock.ProcessBlock(input, loB, hiB)

	for i := range loS {
		if math.Abs(loS[i]-loB[i]) > 1e-12 {
			t.Errorf("lo[%d]: sample=%.15e block=%.15e", i, loS[i], loB[i])
		}
		if math.Abs(hiS[i]-hiB[i]) > 1e-12 {
			t.Errorf("hi[%d]: sample=%.15e block=%.15e", i, hiS[i], hiB[i])
		}
	}
}

// TestCrossover_Reset verifies state clearing.
func TestCrossover_Reset(t *testing.T) {
	xo, _ := New(1000, 4, 48000)

	// Process something.
	xo.ProcessSample(1.0)
	xo.ProcessSample(0.5)

	// Reset and process impulse again.
	xo.Reset()
	lo1, hi1 := xo.ProcessSample(1.0)

	// Create fresh crossover and compare.
	xoFresh, _ := New(1000, 4, 48000)
	lo2, hi2 := xoFresh.ProcessSample(1.0)

	if math.Abs(lo1-lo2) > 1e-15 || math.Abs(hi1-hi2) > 1e-15 {
		t.Errorf("reset mismatch: lo=%v/%v hi=%v/%v", lo1, lo2, hi1, hi2)
	}
}

// TestNewMultiBand_ThreeWay verifies a 3-way crossover (2 frequencies, 3 bands).
func TestNewMultiBand_ThreeWay(t *testing.T) {
	sr := 48000.0
	mb, err := NewMultiBand([]float64{500, 5000}, 4, sr)
	if err != nil {
		t.Fatal(err)
	}
	if mb.NumBands() != 3 {
		t.Fatalf("expected 3 bands, got %d", mb.NumBands())
	}
	if len(mb.Stages()) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(mb.Stages()))
	}
}

// TestNewMultiBand_Errors tests error conditions.
func TestNewMultiBand_Errors(t *testing.T) {
	tests := []struct {
		name  string
		freqs []float64
		order int
		sr    float64
	}{
		{"empty freqs", []float64{}, 4, 48000},
		{"non-ascending", []float64{5000, 500}, 4, 48000},
		{"duplicate", []float64{1000, 1000}, 4, 48000},
		{"odd order", []float64{1000}, 3, 48000},
		{"freq at nyquist", []float64{24000}, 4, 48000},
	}
	for _, tt := range tests {
		_, err := NewMultiBand(tt.freqs, tt.order, tt.sr)
		if err == nil {
			t.Errorf("%s: expected error", tt.name)
		}
	}
}

// TestMultiBand_ProcessSample_EnergyPreservation verifies that the sum
// of all band outputs preserves impulse energy (allpass property).
func TestMultiBand_ProcessSample_EnergyPreservation(t *testing.T) {
	sr := 48000.0
	mb, err := NewMultiBand([]float64{500, 5000}, 4, sr)
	if err != nil {
		t.Fatal(err)
	}

	n := 8192
	sumEnergy := 0.0
	for i := 0; i < n; i++ {
		x := 0.0
		if i == 0 {
			x = 1.0
		}
		bands := mb.ProcessSample(x)
		s := 0.0
		for _, b := range bands {
			s += b
		}
		sumEnergy += s * s
	}

	// Allpass: total energy of impulse response = 1.
	if math.Abs(sumEnergy-1.0) > 0.01 {
		t.Errorf("3-way allpass impulse energy = %v, want 1.0", sumEnergy)
	}
}

// TestMultiBand_ProcessBlock verifies block processing matches sample-by-sample.
func TestMultiBand_ProcessBlock(t *testing.T) {
	sr := 48000.0
	n := 128

	mbSample, _ := NewMultiBand([]float64{500, 5000}, 4, sr)
	mbBlock, _ := NewMultiBand([]float64{500, 5000}, 4, sr)

	input := make([]float64, n)
	input[0] = 1.0

	// Sample-by-sample.
	sampleBands := make([][]float64, 3)
	for i := range sampleBands {
		sampleBands[i] = make([]float64, n)
	}
	for i, x := range input {
		bands := mbSample.ProcessSample(x)
		for b := range bands {
			sampleBands[b][i] = bands[b]
		}
	}

	// Block.
	blockBands := mbBlock.ProcessBlock(input)

	for b := 0; b < 3; b++ {
		for i := range sampleBands[b] {
			if math.Abs(sampleBands[b][i]-blockBands[b][i]) > 1e-12 {
				t.Errorf("band %d sample %d: sample=%.15e block=%.15e", b, i, sampleBands[b][i], blockBands[b][i])
			}
		}
	}
}

// TestMultiBand_Reset verifies state clearing for multi-band crossover.
func TestMultiBand_Reset(t *testing.T) {
	mb, _ := NewMultiBand([]float64{500, 5000}, 4, 48000)

	// Process some samples.
	mb.ProcessSample(1.0)
	mb.ProcessSample(0.5)
	mb.Reset()

	// Compare with fresh.
	mbFresh, _ := NewMultiBand([]float64{500, 5000}, 4, 48000)

	bands1 := mb.ProcessSample(1.0)
	bands2 := mbFresh.ProcessSample(1.0)
	for i := range bands1 {
		if math.Abs(bands1[i]-bands2[i]) > 1e-15 {
			t.Errorf("band %d reset mismatch: %v vs %v", i, bands1[i], bands2[i])
		}
	}
}

// TestMultiBand_FourWay verifies a 4-way crossover.
func TestMultiBand_FourWay(t *testing.T) {
	sr := 48000.0
	mb, err := NewMultiBand([]float64{200, 2000, 10000}, 4, sr)
	if err != nil {
		t.Fatal(err)
	}
	if mb.NumBands() != 4 {
		t.Fatalf("expected 4 bands, got %d", mb.NumBands())
	}

	// Verify outputs are finite.
	bands := mb.ProcessSample(1.0)
	for i, b := range bands {
		if math.IsNaN(b) || math.IsInf(b, 0) {
			t.Errorf("band %d: not finite: %v", i, b)
		}
	}
}
