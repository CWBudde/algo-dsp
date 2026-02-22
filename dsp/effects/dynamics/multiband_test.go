package dynamics

import (
	"fmt"
	"math"
	"testing"
)

// --- Constructor tests ---

func TestNewMultibandCompressor(t *testing.T) {
	tests := []struct {
		name    string
		freqs   []float64
		order   int
		sr      float64
		wantErr bool
	}{
		{"2-band LR4", []float64{1000}, 4, 48000, false},
		{"3-band LR4", []float64{500, 5000}, 4, 48000, false},
		{"4-band LR8", []float64{200, 2000, 10000}, 8, 48000, false},
		{"2-band LR2", []float64{1000}, 2, 48000, false},
		{"2-band LR12", []float64{1000}, 12, 48000, false},
		{"7-band max", []float64{100, 300, 1000, 3000, 8000, 16000}, 4, 48000, false},

		// Error cases
		{"no freqs", []float64{}, 4, 48000, true},
		{"odd order", []float64{1000}, 3, 48000, true},
		{"zero order", []float64{1000}, 0, 48000, true},
		{"negative order", []float64{1000}, -2, 48000, true},
		{"order too high", []float64{1000}, 26, 48000, true},
		{"zero sample rate", []float64{1000}, 4, 0, true},
		{"negative sample rate", []float64{1000}, 4, -44100, true},
		{"NaN sample rate", []float64{1000}, 4, math.NaN(), true},
		{"freq at nyquist", []float64{24000}, 4, 48000, true},
		{"freq above nyquist", []float64{25000}, 4, 48000, true},
		{"freq below min", []float64{10}, 4, 48000, true},
		{"non-ascending freqs", []float64{5000, 500}, 4, 48000, true},
		{"duplicate freqs", []float64{1000, 1000}, 4, 48000, true},
		{"NaN freq", []float64{math.NaN()}, 4, 48000, true},
		{"too many bands", []float64{100, 200, 400, 800, 1600, 3200, 6400, 12800}, 4, 48000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc, err := NewMultibandCompressor(tt.freqs, tt.order, tt.sr)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewMultibandCompressor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if mc == nil {
					t.Fatal("NewMultibandCompressor() returned nil without error")
				}

				wantBands := len(tt.freqs) + 1
				if mc.NumBands() != wantBands {
					t.Errorf("NumBands() = %d, want %d", mc.NumBands(), wantBands)
				}

				if mc.CrossoverOrder() != tt.order {
					t.Errorf("CrossoverOrder() = %d, want %d", mc.CrossoverOrder(), tt.order)
				}

				if mc.SampleRate() != tt.sr {
					t.Errorf("SampleRate() = %v, want %v", mc.SampleRate(), tt.sr)
				}
			}
		})
	}
}

func TestNewMultibandCompressorWithConfig(t *testing.T) {
	t.Run("valid 3-band config", func(t *testing.T) {
		autoTrue := true
		configs := []BandConfig{
			{ThresholdDB: Float64Ptr(-30), Ratio: 2.0, KneeDB: Float64Ptr(6.0), AttackMs: 20, ReleaseMs: 200, AutoMakeup: &autoTrue},
			{ThresholdDB: Float64Ptr(-20), Ratio: 4.0, KneeDB: Float64Ptr(3.0), AttackMs: 10, ReleaseMs: 100},
			{ThresholdDB: Float64Ptr(-15), Ratio: 6.0, KneeDB: Float64Ptr(0.0), AttackMs: 5, ReleaseMs: 50},
		}

		mc, err := NewMultibandCompressorWithConfig([]float64{500, 5000}, 4, 48000, configs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mc.Band(0).Threshold() != -30 {
			t.Errorf("band 0 threshold = %f, want -30", mc.Band(0).Threshold())
		}

		if mc.Band(1).Ratio() != 4.0 {
			t.Errorf("band 1 ratio = %f, want 4.0", mc.Band(1).Ratio())
		}

		if mc.Band(2).Attack() != 5.0 {
			t.Errorf("band 2 attack = %f, want 5.0", mc.Band(2).Attack())
		}
	})

	t.Run("wrong config count", func(t *testing.T) {
		configs := []BandConfig{{}, {}} // 2 configs for 3 bands

		_, err := NewMultibandCompressorWithConfig([]float64{500, 5000}, 4, 48000, configs)
		if err == nil {
			t.Error("expected error for wrong config count")
		}
	})

	t.Run("invalid band config", func(t *testing.T) {
		configs := []BandConfig{
			{Ratio: 0.5}, // Invalid ratio
			{},
		}

		_, err := NewMultibandCompressorWithConfig([]float64{1000}, 4, 48000, configs)
		if err == nil {
			t.Error("expected error for invalid band config")
		}
	})
}

// --- Accessor tests ---

func TestMultibandAccessors(t *testing.T) {
	freqs := []float64{500, 5000}

	mc, err := NewMultibandCompressor(freqs, 4, 48000)
	if err != nil {
		t.Fatal(err)
	}

	// CrossoverFreqs returns a copy
	got := mc.CrossoverFreqs()
	if len(got) != 2 || got[0] != 500 || got[1] != 5000 {
		t.Errorf("CrossoverFreqs() = %v, want [500, 5000]", got)
	}
	// Modify the copy — original should be unaffected
	got[0] = 999

	got2 := mc.CrossoverFreqs()
	if got2[0] != 500 {
		t.Error("CrossoverFreqs() returned a reference, not a copy")
	}

	// Band returns compressors
	for i := 0; i < mc.NumBands(); i++ {
		if mc.Band(i) == nil {
			t.Errorf("Band(%d) returned nil", i)
		}
	}

	// Crossover is accessible
	if mc.Crossover() == nil {
		t.Error("Crossover() returned nil")
	}
}

// --- Per-band setter tests ---

func TestMultibandSetBandParams(t *testing.T) {
	mc, _ := NewMultibandCompressor([]float64{1000}, 4, 48000)

	t.Run("valid band index", func(t *testing.T) {
		if err := mc.SetBandThreshold(0, -30); err != nil {
			t.Errorf("SetBandThreshold: %v", err)
		}

		if mc.Band(0).Threshold() != -30 {
			t.Errorf("threshold = %f, want -30", mc.Band(0).Threshold())
		}

		if err := mc.SetBandRatio(1, 8.0); err != nil {
			t.Errorf("SetBandRatio: %v", err)
		}

		if mc.Band(1).Ratio() != 8.0 {
			t.Errorf("ratio = %f, want 8.0", mc.Band(1).Ratio())
		}

		if err := mc.SetBandKnee(0, 12.0); err != nil {
			t.Errorf("SetBandKnee: %v", err)
		}

		if mc.Band(0).Knee() != 12.0 {
			t.Errorf("knee = %f, want 12.0", mc.Band(0).Knee())
		}

		if err := mc.SetBandAttack(0, 5.0); err != nil {
			t.Errorf("SetBandAttack: %v", err)
		}

		if mc.Band(0).Attack() != 5.0 {
			t.Errorf("attack = %f, want 5.0", mc.Band(0).Attack())
		}

		if err := mc.SetBandRelease(1, 200.0); err != nil {
			t.Errorf("SetBandRelease: %v", err)
		}

		if mc.Band(1).Release() != 200.0 {
			t.Errorf("release = %f, want 200.0", mc.Band(1).Release())
		}

		if err := mc.SetBandMakeupGain(0, 3.0); err != nil {
			t.Errorf("SetBandMakeupGain: %v", err)
		}

		if mc.Band(0).MakeupGain() != 3.0 {
			t.Errorf("makeup = %f, want 3.0", mc.Band(0).MakeupGain())
		}

		if mc.Band(0).AutoMakeup() {
			t.Error("auto makeup should be disabled after SetBandMakeupGain")
		}

		if err := mc.SetBandAutoMakeup(0, true); err != nil {
			t.Errorf("SetBandAutoMakeup: %v", err)
		}

		if !mc.Band(0).AutoMakeup() {
			t.Error("auto makeup should be re-enabled")
		}
	})

	t.Run("invalid band index", func(t *testing.T) {
		if err := mc.SetBandThreshold(-1, -20); err == nil {
			t.Error("expected error for negative band index")
		}

		if err := mc.SetBandThreshold(2, -20); err == nil {
			t.Error("expected error for out-of-range band index")
		}

		if err := mc.SetBandRatio(-1, 2.0); err == nil {
			t.Error("expected error for negative band index")
		}

		if err := mc.SetBandKnee(5, 6.0); err == nil {
			t.Error("expected error for out-of-range band index")
		}

		if err := mc.SetBandAttack(5, 10.0); err == nil {
			t.Error("expected error for out-of-range band index")
		}

		if err := mc.SetBandRelease(5, 100.0); err == nil {
			t.Error("expected error for out-of-range band index")
		}

		if err := mc.SetBandMakeupGain(5, 3.0); err == nil {
			t.Error("expected error for out-of-range band index")
		}

		if err := mc.SetBandAutoMakeup(5, true); err == nil {
			t.Error("expected error for out-of-range band index")
		}

		if err := mc.SetBandConfig(5, BandConfig{}); err == nil {
			t.Error("expected error for out-of-range band index")
		}
	})

	t.Run("invalid parameter value", func(t *testing.T) {
		if err := mc.SetBandRatio(0, 0.5); err == nil {
			t.Error("expected error for invalid ratio")
		}

		if err := mc.SetBandThreshold(0, math.NaN()); err == nil {
			t.Error("expected error for NaN threshold")
		}
	})
}

func TestMultibandSetAllBands(t *testing.T) {
	mc, _ := NewMultibandCompressor([]float64{500, 5000}, 4, 48000)

	if err := mc.SetAllBandsThreshold(-15); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < mc.NumBands(); i++ {
		if mc.Band(i).Threshold() != -15 {
			t.Errorf("band %d threshold = %f, want -15", i, mc.Band(i).Threshold())
		}
	}

	if err := mc.SetAllBandsRatio(3.0); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < mc.NumBands(); i++ {
		if mc.Band(i).Ratio() != 3.0 {
			t.Errorf("band %d ratio = %f, want 3.0", i, mc.Band(i).Ratio())
		}
	}

	if err := mc.SetAllBandsKnee(10.0); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < mc.NumBands(); i++ {
		if mc.Band(i).Knee() != 10.0 {
			t.Errorf("band %d knee = %f, want 10.0", i, mc.Band(i).Knee())
		}
	}

	if err := mc.SetAllBandsAttack(15.0); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < mc.NumBands(); i++ {
		if mc.Band(i).Attack() != 15.0 {
			t.Errorf("band %d attack = %f, want 15.0", i, mc.Band(i).Attack())
		}
	}

	if err := mc.SetAllBandsRelease(250.0); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < mc.NumBands(); i++ {
		if mc.Band(i).Release() != 250.0 {
			t.Errorf("band %d release = %f, want 250.0", i, mc.Band(i).Release())
		}
	}
}

func TestMultibandSetAllBandsInvalid(t *testing.T) {
	mc, _ := NewMultibandCompressor([]float64{1000}, 4, 48000)

	if err := mc.SetAllBandsRatio(0.5); err == nil {
		t.Error("expected error for invalid ratio")
	}

	if err := mc.SetAllBandsThreshold(math.NaN()); err == nil {
		t.Error("expected error for NaN threshold")
	}

	if err := mc.SetAllBandsKnee(-1); err == nil {
		t.Error("expected error for negative knee")
	}

	if err := mc.SetAllBandsAttack(0.01); err == nil {
		t.Error("expected error for too-small attack")
	}

	if err := mc.SetAllBandsRelease(0.1); err == nil {
		t.Error("expected error for too-small release")
	}
}

// --- Processing tests ---

func TestMultibandProcessSampleZero(t *testing.T) {
	mc, _ := NewMultibandCompressor([]float64{1000}, 4, 48000)
	mc.Reset()

	for i := 0; i < 100; i++ {
		output := mc.ProcessSample(0)
		if output != 0 {
			t.Errorf("ProcessSample(0) = %f, want 0 at sample %d", output, i)
			break
		}
	}
}

func TestMultibandProcessSampleFinite(t *testing.T) {
	mc, _ := NewMultibandCompressor([]float64{500, 5000}, 4, 48000)

	// Process an impulse followed by zeros
	for i := 0; i < 1000; i++ {
		x := 0.0
		if i == 0 {
			x = 0.5
		}

		output := mc.ProcessSample(x)
		if math.IsNaN(output) || math.IsInf(output, 0) {
			t.Fatalf("ProcessSample produced non-finite output at sample %d: %v", i, output)
		}
	}
}

func TestMultibandProcessSampleMulti(t *testing.T) {
	mc, _ := NewMultibandCompressor([]float64{500, 5000}, 4, 48000)

	bands := mc.ProcessSampleMulti(0.5)
	if len(bands) != 3 {
		t.Fatalf("ProcessSampleMulti returned %d bands, want 3", len(bands))
	}

	for i, b := range bands {
		if math.IsNaN(b) || math.IsInf(b, 0) {
			t.Errorf("band %d: non-finite value: %v", i, b)
		}
	}
}

func TestMultibandProcessInPlace(t *testing.T) {
	sr := 48000.0
	n := 256

	// Create two identical compressors
	mc1, _ := NewMultibandCompressor([]float64{1000}, 4, sr)
	mc2, _ := NewMultibandCompressor([]float64{1000}, 4, sr)

	// Disable auto makeup for deterministic comparison
	for i := 0; i < mc1.NumBands(); i++ {
		_ = mc1.SetBandAutoMakeup(i, false)
		_ = mc1.SetBandMakeupGain(i, 0)
		_ = mc2.SetBandAutoMakeup(i, false)
		_ = mc2.SetBandMakeupGain(i, 0)
	}

	// Generate test signal (sine with varying amplitude)
	input := make([]float64, n)
	for i := range input {
		input[i] = 0.3 * math.Sin(2*math.Pi*440*float64(i)/sr)
	}

	// Process with ProcessSample
	want := make([]float64, n)
	for i, x := range input {
		want[i] = mc1.ProcessSample(x)
	}

	// Process with ProcessInPlace
	got := make([]float64, n)
	copy(got, input)
	mc2.ProcessInPlace(got)

	// Compare
	const tol = 1e-10

	for i := range got {
		diff := math.Abs(got[i] - want[i])
		if diff > tol {
			t.Errorf("sample %d: ProcessInPlace = %.15e, ProcessSample = %.15e, diff = %g",
				i, got[i], want[i], diff)

			break
		}
	}
}

func TestMultibandProcessInPlaceEmpty(t *testing.T) {
	mc, _ := NewMultibandCompressor([]float64{1000}, 4, 48000)
	// Must not panic
	mc.ProcessInPlace([]float64{})
}

func TestMultibandProcessInPlaceMulti(t *testing.T) {
	mc, _ := NewMultibandCompressor([]float64{500, 5000}, 4, 48000)

	input := make([]float64, 128)
	input[0] = 0.5

	bands := mc.ProcessInPlaceMulti(input)
	if len(bands) != 3 {
		t.Fatalf("ProcessInPlaceMulti returned %d bands, want 3", len(bands))
	}

	for i, band := range bands {
		if len(band) != 128 {
			t.Errorf("band %d length = %d, want 128", i, len(band))
		}
	}
}

func TestMultibandProcessInPlaceMultiEmpty(t *testing.T) {
	mc, _ := NewMultibandCompressor([]float64{1000}, 4, 48000)

	bands := mc.ProcessInPlaceMulti([]float64{})
	if len(bands) != 2 {
		t.Fatalf("expected 2 bands, got %d", len(bands))
	}

	for i, band := range bands {
		if len(band) != 0 {
			t.Errorf("band %d should be empty, got length %d", i, len(band))
		}
	}
}

// --- Compression behavior tests ---

func TestMultibandCompressesLoudSignal(t *testing.T) {
	mc, _ := NewMultibandCompressor([]float64{1000}, 4, 48000)

	// Configure for heavy compression with no auto makeup
	for i := 0; i < mc.NumBands(); i++ {
		_ = mc.SetBandThreshold(i, -20)
		_ = mc.SetBandRatio(i, 10.0)
		_ = mc.SetBandKnee(i, 0)
		_ = mc.SetBandAttack(i, 0.1) // Very fast attack
		_ = mc.SetBandAutoMakeup(i, false)
		_ = mc.SetBandMakeupGain(i, 0)
	}

	// Feed loud signal for many samples to let the envelope settle
	var lastOutput float64
	for i := 0; i < 5000; i++ {
		lastOutput = mc.ProcessSample(0.8)
	}

	// Output should be significantly reduced
	if math.Abs(lastOutput) >= 0.8 {
		t.Errorf("expected compression, output magnitude = %f", math.Abs(lastOutput))
	}
}

func TestMultibandBandIndependence(t *testing.T) {
	mc, _ := NewMultibandCompressor([]float64{1000}, 4, 48000)

	// Configure band 0 (low) with heavy compression, band 1 (high) with none
	_ = mc.SetBandThreshold(0, -30)
	_ = mc.SetBandRatio(0, 20.0)
	_ = mc.SetBandAutoMakeup(0, false)
	_ = mc.SetBandMakeupGain(0, 0)

	_ = mc.SetBandRatio(1, 1.0) // No compression

	// Verify settings are independent
	if mc.Band(0).Ratio() != 20.0 {
		t.Errorf("band 0 ratio = %f, want 20.0", mc.Band(0).Ratio())
	}

	if mc.Band(1).Ratio() != 1.0 {
		t.Errorf("band 1 ratio = %f, want 1.0", mc.Band(1).Ratio())
	}
}

// --- State management tests ---

func TestMultibandReset(t *testing.T) {
	mc, _ := NewMultibandCompressor([]float64{1000}, 4, 48000)

	// Process some samples
	for i := 0; i < 100; i++ {
		mc.ProcessSample(0.5)
	}

	// Reset
	mc.Reset()

	// Create fresh instance for comparison
	mcFresh, _ := NewMultibandCompressor([]float64{1000}, 4, 48000)

	// Both should produce identical output for an impulse
	out1 := mc.ProcessSample(0.3)
	out2 := mcFresh.ProcessSample(0.3)

	if math.Abs(out1-out2) > 1e-15 {
		t.Errorf("reset mismatch: %v vs %v", out1, out2)
	}
}

func TestMultibandMetrics(t *testing.T) {
	mc, _ := NewMultibandCompressor([]float64{1000}, 4, 48000)
	mc.ResetMetrics()

	// Process loud signal
	for i := 0; i < 500; i++ {
		mc.ProcessSample(0.8)
	}

	metrics := mc.GetMetrics()
	if len(metrics.Bands) != 2 {
		t.Fatalf("expected 2 band metrics, got %d", len(metrics.Bands))
	}

	// At least one band should have non-zero input peak
	hasInput := false

	for _, bm := range metrics.Bands {
		if bm.InputPeak > 0 {
			hasInput = true
			break
		}
	}

	if !hasInput {
		t.Error("expected at least one band to have non-zero input peak")
	}

	// Reset and verify
	mc.ResetMetrics()

	metrics = mc.GetMetrics()
	for i, bm := range metrics.Bands {
		if bm.InputPeak != 0 || bm.OutputPeak != 0 {
			t.Errorf("band %d: metrics not reset: InputPeak=%f, OutputPeak=%f",
				i, bm.InputPeak, bm.OutputPeak)
		}
	}
}

// --- Crossover order tests ---

func TestMultibandDifferentOrders(t *testing.T) {
	orders := []int{2, 4, 8, 12}

	for _, order := range orders {
		t.Run(fmt.Sprintf("LR%d", order), func(t *testing.T) {
			mc, err := NewMultibandCompressor([]float64{1000}, order, 48000)
			if err != nil {
				t.Fatalf("NewMultibandCompressor: %v", err)
			}

			if mc.CrossoverOrder() != order {
				t.Errorf("CrossoverOrder() = %d, want %d", mc.CrossoverOrder(), order)
			}

			// Process an impulse to verify it works
			output := mc.ProcessSample(0.5)
			if math.IsNaN(output) || math.IsInf(output, 0) {
				t.Fatalf("non-finite output with order %d: %v", order, output)
			}
		})
	}
}

// --- Energy preservation test ---

func TestMultibandEnergyPreservation(t *testing.T) {
	// With ratio 1.0 (no compression) and no makeup gain, the multiband
	// compressor should pass the signal through with near-unity gain
	// (allpass from the crossover).
	mc, _ := NewMultibandCompressor([]float64{500, 5000}, 4, 48000)

	// Set all bands to unity (no compression)
	for i := 0; i < mc.NumBands(); i++ {
		_ = mc.SetBandRatio(i, 1.0)
		_ = mc.SetBandAutoMakeup(i, false)
		_ = mc.SetBandMakeupGain(i, 0)
	}

	// Feed an impulse and measure output energy
	n := 8192
	energy := 0.0

	for i := 0; i < n; i++ {
		x := 0.0
		if i == 0 {
			x = 1.0
		}

		y := mc.ProcessSample(x)
		energy += y * y
	}

	// Energy should be ~1.0 (allpass property)
	if math.Abs(energy-1.0) > 0.02 {
		t.Errorf("impulse energy = %f, want ~1.0 (allpass)", energy)
	}
}

// --- SetBandConfig test ---

func TestSetBandConfig(t *testing.T) {
	mc, _ := NewMultibandCompressor([]float64{1000}, 4, 48000)

	autoFalse := false
	cfg := BandConfig{
		ThresholdDB:  Float64Ptr(-25),
		Ratio:        6.0,
		KneeDB:       Float64Ptr(8.0),
		AttackMs:     15.0,
		ReleaseMs:    150.0,
		MakeupGainDB: Float64Ptr(5.0),
		AutoMakeup:   &autoFalse,
	}

	if err := mc.SetBandConfig(0, cfg); err != nil {
		t.Fatal(err)
	}

	b := mc.Band(0)
	if b.Threshold() != -25 {
		t.Errorf("threshold = %f, want -25", b.Threshold())
	}

	if b.Ratio() != 6.0 {
		t.Errorf("ratio = %f, want 6.0", b.Ratio())
	}

	if b.Knee() != 8.0 {
		t.Errorf("knee = %f, want 8.0", b.Knee())
	}

	if b.Attack() != 15.0 {
		t.Errorf("attack = %f, want 15.0", b.Attack())
	}

	if b.Release() != 150.0 {
		t.Errorf("release = %f, want 150.0", b.Release())
	}

	if b.AutoMakeup() {
		t.Error("auto makeup should be false")
	}
}
