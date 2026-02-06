package thd

import (
	"math"
	"testing"
)

func TestCalculateFromMagnitudeKnownSpectrum(t *testing.T) {
	cfg := Config{
		SampleRate:      48000,
		FFTSize:         48000,
		FundamentalFreq: 1000,
		RangeLowerFreq:  20,
		RangeUpperFreq:  10000,
		CaptureBins:     0,
		RubNBuzzStart:   3,
		WindowType:      1,
	}

	mag := make([]float64, cfg.FFTSize/2+1)
	mag[1000] = 1.0       // Fundamental amplitude 1.0
	mag[2000] = 0.1 * 0.1 // H2 amplitude 0.1
	mag[3000] = 0.05 * 0.05
	mag[4500] = 0.02 * 0.02 // non-harmonic noise

	res := NewCalculator(cfg).CalculateFromMagnitude(mag)

	if math.Abs(res.FundamentalFreq-1000) > 1e-9 {
		t.Fatalf("fundamental freq mismatch: got %f", res.FundamentalFreq)
	}
	if math.Abs(res.FundamentalLevel-1.0) > 1e-9 {
		t.Fatalf("fundamental level mismatch: got %f", res.FundamentalLevel)
	}
	if math.Abs(res.THD-0.15) > 1e-12 {
		t.Fatalf("THD mismatch: got %.12f want %.12f", res.THD, 0.15)
	}
	if math.Abs(res.THDN-0.17) > 1e-12 {
		t.Fatalf("THDN mismatch: got %.12f want %.12f", res.THDN, 0.17)
	}
	if math.Abs(res.Noise-0.02) > 1e-12 {
		t.Fatalf("Noise mismatch: got %.12f want %.12f", res.Noise, 0.02)
	}
	if math.Abs(res.OddHD-0.05) > 1e-12 {
		t.Fatalf("OddHD mismatch: got %.12f want %.12f", res.OddHD, 0.05)
	}
	if math.Abs(res.EvenHD-0.1) > 1e-12 {
		t.Fatalf("EvenHD mismatch: got %.12f want %.12f", res.EvenHD, 0.1)
	}
	if math.Abs(res.RubNBuzz-0.05) > 1e-12 {
		t.Fatalf("RubNBuzz mismatch: got %.12f want %.12f", res.RubNBuzz, 0.05)
	}
	if len(res.Harmonics) != 2 {
		t.Fatalf("harmonic count mismatch: got %d want 2", len(res.Harmonics))
	}
	if math.Abs(res.Harmonics[0]-0.1) > 1e-12 || math.Abs(res.Harmonics[1]-0.05) > 1e-12 {
		t.Fatalf("harmonics mismatch: got %+v", res.Harmonics)
	}

	wantSINAD := 20 * math.Log10(1/0.17)
	if math.Abs(res.SINAD-wantSINAD) > 1e-12 {
		t.Fatalf("SINAD mismatch: got %f want %f", res.SINAD, wantSINAD)
	}
}

func TestCalculateAutodetectFundamental(t *testing.T) {
	cfg := Config{
		SampleRate:     48000,
		FFTSize:        48000,
		RangeLowerFreq: 20,
		RangeUpperFreq: 5000,
		CaptureBins:    0,
	}
	mag := make([]float64, cfg.FFTSize/2+1)
	mag[1000] = 0.8 * 0.8
	mag[1200] = 1.2 * 1.2
	mag[2400] = 0.1 * 0.1

	res := NewCalculator(cfg).CalculateFromMagnitude(mag)
	if math.Abs(res.FundamentalFreq-1200) > 1e-9 {
		t.Fatalf("auto fundamental mismatch: got %f", res.FundamentalFreq)
	}
	if len(res.Harmonics) == 0 {
		t.Fatalf("expected harmonics to include H2")
	}
}

func TestCalculateCaptureBins(t *testing.T) {
	cfg := Config{
		SampleRate:      48000,
		FFTSize:         48000,
		FundamentalFreq: 1000,
		RangeLowerFreq:  20,
		RangeUpperFreq:  5000,
		CaptureBins:     1,
	}

	mag := make([]float64, cfg.FFTSize/2+1)
	mag[999] = 0.2 * 0.2
	mag[1000] = 1.0 * 1.0
	mag[1001] = 0.2 * 0.2
	mag[2000] = 0.1 * 0.1
	mag[2001] = 0.05 * 0.05

	res := NewCalculator(cfg).CalculateFromMagnitude(mag)

	// Fundamental with capture bins is 1.4, harmonic is 0.15.
	if math.Abs(res.FundamentalLevel-1.4) > 1e-12 {
		t.Fatalf("fundamental capture mismatch: got %.12f", res.FundamentalLevel)
	}
	if math.Abs(res.THD-(0.15/1.4)) > 1e-12 {
		t.Fatalf("THD capture mismatch: got %.12f want %.12f", res.THD, 0.15/1.4)
	}
}

func TestAnalyzeSignalPureToneLowDistortion(t *testing.T) {
	sr := 48000.0
	n := 4096
	fundamentalBin := 64
	freq := float64(fundamentalBin) * sr / float64(n)

	signal := make([]float64, n)
	for i := range signal {
		signal[i] = math.Sin(2 * math.Pi * freq * float64(i) / sr)
	}

	res := AnalyzeSignal(signal, Config{
		SampleRate:      sr,
		FFTSize:         n,
		FundamentalFreq: freq,
		RangeLowerFreq:  20,
		RangeUpperFreq:  20000,
		CaptureBins:     0,
	})

	if res.FundamentalLevel <= 0 {
		t.Fatalf("expected positive fundamental level")
	}
	if res.THD > 1e-3 {
		t.Fatalf("expected near-zero THD, got %g", res.THD)
	}
}

func TestCalculateFromMagnitudeMultiToneHarmonicSeparation(t *testing.T) {
	cfg := Config{
		SampleRate:      48000,
		FFTSize:         48000,
		FundamentalFreq: 1000, // analyze tone A
		RangeLowerFreq:  20,
		RangeUpperFreq:  10000,
		CaptureBins:     0,
	}

	mag := make([]float64, cfg.FFTSize/2+1)

	// Tone A (fundamental under test) and its harmonics.
	mag[1000] = 1.0 * 1.0
	mag[2000] = 0.10 * 0.10 // H2(A)
	mag[3000] = 0.05 * 0.05 // H3(A)

	// Tone B and its harmonics (must not be counted as A's harmonics).
	mag[1300] = 0.80 * 0.80
	mag[2600] = 0.20 * 0.20 // H2(B)
	mag[3900] = 0.10 * 0.10 // H3(B)

	res := NewCalculator(cfg).CalculateFromMagnitude(mag)

	// THD for tone A should include only H2(A)+H3(A) = 0.15.
	if math.Abs(res.THD-0.15) > 1e-12 {
		t.Fatalf("THD mismatch: got %.12f want %.12f", res.THD, 0.15)
	}
	if len(res.Harmonics) != 2 {
		t.Fatalf("harmonic count mismatch: got %d want 2", len(res.Harmonics))
	}

	// THDN includes all bins except fundamental A.
	wantTHDN := 0.10 + 0.05 + 0.80 + 0.20 + 0.10
	if math.Abs(res.THDN-wantTHDN) > 1e-12 {
		t.Fatalf("THDN mismatch: got %.12f want %.12f", res.THDN, wantTHDN)
	}

	// Therefore Noise = THDN - THD should be exactly tone B + its harmonics.
	wantNoise := 0.80 + 0.20 + 0.10
	if math.Abs(res.Noise-wantNoise) > 1e-12 {
		t.Fatalf("Noise mismatch: got %.12f want %.12f", res.Noise, wantNoise)
	}
}
