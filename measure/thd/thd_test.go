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
	mag[1000] = 1.0         // Fundamental amplitude 1.0
	mag[2000] = 0.1 * 0.1   // H2 amplitude 0.1
	mag[3000] = 0.05 * 0.05 // H3 amplitude 0.05
	mag[4500] = 0.02 * 0.02 // non-harmonic noise, amplitude 0.02

	res := NewCalculator(cfg).CalculateFromMagnitude(mag)

	if math.Abs(res.FundamentalFreq-1000) > 1e-9 {
		t.Fatalf("fundamental freq mismatch: got %f", res.FundamentalFreq)
	}

	if math.Abs(res.FundamentalLevel-1.0) > 1e-9 {
		t.Fatalf("fundamental level mismatch: got %f", res.FundamentalLevel)
	}

	// IEEE RSS definitions: sqrt(sum of component powers) / fundamental.
	wantTHD := math.Sqrt(0.1*0.1 + 0.05*0.05)
	if math.Abs(res.THD-wantTHD) > 1e-12 {
		t.Fatalf("THD mismatch: got %.12f want %.12f", res.THD, wantTHD)
	}

	wantTHDN := math.Sqrt(0.1*0.1 + 0.05*0.05 + 0.02*0.02)
	if math.Abs(res.THDN-wantTHDN) > 1e-12 {
		t.Fatalf("THDN mismatch: got %.12f want %.12f", res.THDN, wantTHDN)
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

	wantSINAD := 20 * math.Log10(1/wantTHDN)
	if math.Abs(res.SINAD-wantSINAD) > 1e-12 {
		t.Fatalf("SINAD mismatch: got %f want %f", res.SINAD, wantSINAD)
	}
}

// TestCalculateTwoOnePercentHarmonics pins the THD definition: two harmonics
// of 1%% each combine as RSS to 1.414%%, not to the 2%% a linear magnitude
// sum would produce.
func TestCalculateTwoOnePercentHarmonics(t *testing.T) {
	cfg := Config{
		SampleRate:      48000,
		FFTSize:         48000,
		FundamentalFreq: 1000,
		RangeLowerFreq:  20,
		RangeUpperFreq:  10000,
		CaptureBins:     0,
	}

	mag := make([]float64, cfg.FFTSize/2+1)
	mag[1000] = 1.0
	mag[2000] = 0.01 * 0.01
	mag[3000] = 0.01 * 0.01

	res := NewCalculator(cfg).CalculateFromMagnitude(mag)

	want := 0.01 * math.Sqrt2
	if math.Abs(res.THD-want) > 1e-12 {
		t.Fatalf("THD mismatch: got %.12f want %.12f (IEEE RSS)", res.THD, want)
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

	// Fundamental power over the capture window is 0.04+1+0.04 = 1.08;
	// the reported level is its RSS amplitude.
	wantLevel := math.Sqrt(1.08)
	if math.Abs(res.FundamentalLevel-wantLevel) > 1e-12 {
		t.Fatalf("fundamental capture mismatch: got %.12f want %.12f", res.FundamentalLevel, wantLevel)
	}

	// H2 power over its capture window is 0.01+0.0025 = 0.0125.
	wantTHD := math.Sqrt(0.0125 / 1.08)
	if math.Abs(res.THD-wantTHD) > 1e-12 {
		t.Fatalf("THD capture mismatch: got %.12f want %.12f", res.THD, wantTHD)
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

// TestAnalyzeSignalLongerThanFFTSize guards the former index-out-of-range
// panic when len(signal) > FFTSize: the analyzer must truncate, not crash.
func TestAnalyzeSignalLongerThanFFTSize(t *testing.T) {
	sr := 48000.0
	fftSize := 4096
	freq := 1500.0

	signal := make([]float64, 3*fftSize+17)
	for i := range signal {
		signal[i] = math.Sin(2 * math.Pi * freq * float64(i) / sr)
	}

	res := AnalyzeSignal(signal, Config{
		SampleRate:      sr,
		FFTSize:         fftSize,
		FundamentalFreq: freq,
		RangeLowerFreq:  20,
		RangeUpperFreq:  20000,
	})

	if res.FundamentalLevel <= 0 {
		t.Fatalf("expected positive fundamental level, got %g", res.FundamentalLevel)
	}
}

// TestAnalyzeSignalSineWithKnownNoiseFloor is the regression test for the
// linear-magnitude-summation defect: a pure sine plus a known white noise
// floor must report SINAD near the analytic value instead of a figure tens
// of dB low (the error grew with FFT size under the old math).
func TestAnalyzeSignalSineWithKnownNoiseFloor(t *testing.T) {
	sr := 48000.0
	n := 16384
	fundamentalBin := 512 // bin-centered 1.5 kHz tone
	freq := float64(fundamentalBin) * sr / float64(n)

	// White noise at -80 dB relative to the sine amplitude, from a
	// deterministic LCG so the test is reproducible.
	noiseAmp := math.Pow(10, -80.0/20)
	seed := uint64(0x2545F4914F6CDD1D)
	next := func() float64 {
		seed = seed*6364136223846793005 + 1442695040888963407
		// Map the top 53 bits to [-1, 1).
		return float64(seed>>11)/float64(1<<52) - 1
	}

	signal := make([]float64, n)
	noisePower := 0.0

	for i := range signal {
		w := noiseAmp * next()
		noisePower += w * w
		signal[i] = math.Sin(2*math.Pi*freq*float64(i)/sr) + w
	}

	noisePower /= float64(n)

	res := AnalyzeSignal(signal, Config{
		SampleRate:      sr,
		FFTSize:         n,
		FundamentalFreq: freq,
		RangeLowerFreq:  20,
		RangeUpperFreq:  20000,
		CaptureBins:     3,
	})

	// Analytic SINAD: fundamental power (1/2 for a unit sine) over the
	// in-band share of the measured noise power. The analysis window spans
	// 20 Hz..20 kHz of the 24 kHz Nyquist band.
	inBand := (20000.0 - 20.0) / (sr / 2)
	wantSINAD := 10 * math.Log10((1.0/2)/(noisePower*inBand))

	if math.Abs(res.SINAD-wantSINAD) > 1.5 {
		t.Fatalf("SINAD = %.2f dB, want %.2f dB +/- 1.5 dB", res.SINAD, wantSINAD)
	}
}

// TestAnalyzeSignalNonBinCentered checks that a fundamental that does not
// fall on a bin center still reports a sensible level and near-zero THD when
// analyzed through a window with a matching capture width.
func TestAnalyzeSignalNonBinCentered(t *testing.T) {
	sr := 48000.0
	n := 8192
	freq := 997.0 // deliberately off the bin grid

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
		CaptureBins:     4,
	})

	// A Hann-windowed unit sine has coherent gain 0.5, so the RSS level over
	// the main lobe is near 0.5 * n/2 scaled by the FFT convention; rather
	// than pin the absolute scale here, require the distortion metrics to
	// stay small and the level positive.
	if res.FundamentalLevel <= 0 {
		t.Fatalf("expected positive fundamental level")
	}

	if res.THD > 1e-3 {
		t.Fatalf("expected near-zero THD for a pure off-grid tone, got %g", res.THD)
	}

	if res.THDN > 0.02 {
		t.Fatalf("expected small THDN for a pure off-grid tone, got %g", res.THDN)
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

	// THD for tone A should include only H2(A)+H3(A), combined as RSS.
	wantTHD := math.Sqrt(0.10*0.10 + 0.05*0.05)
	if math.Abs(res.THD-wantTHD) > 1e-12 {
		t.Fatalf("THD mismatch: got %.12f want %.12f", res.THD, wantTHD)
	}

	if len(res.Harmonics) != 2 {
		t.Fatalf("harmonic count mismatch: got %d want 2", len(res.Harmonics))
	}

	// THDN includes all in-range power except the fundamental.
	thdnPower := 0.10*0.10 + 0.05*0.05 + 0.80*0.80 + 0.20*0.20 + 0.10*0.10

	wantTHDN := math.Sqrt(thdnPower)
	if math.Abs(res.THDN-wantTHDN) > 1e-12 {
		t.Fatalf("THDN mismatch: got %.12f want %.12f", res.THDN, wantTHDN)
	}

	// Noise is everything in range that is neither fundamental nor harmonic
	// of tone A: tone B plus its harmonics, combined as RSS.
	noisePower := 0.80*0.80 + 0.20*0.20 + 0.10*0.10

	wantNoise := math.Sqrt(noisePower)
	if math.Abs(res.Noise-wantNoise) > 1e-12 {
		t.Fatalf("Noise mismatch: got %.12f want %.12f", res.Noise, wantNoise)
	}
}
