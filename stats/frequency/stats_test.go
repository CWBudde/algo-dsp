package frequency

import (
	"math"
	"math/cmplx"
	"testing"
)

const tolerance = 1e-9

func almostEqual(a, b, tol float64) bool {
	if math.IsInf(a, -1) && math.IsInf(b, -1) {
		return true
	}

	if math.IsInf(a, 1) && math.IsInf(b, 1) {
		return true
	}

	return math.Abs(a-b) <= tol
}

// makeSingleBinSpectrum creates a spectrum of given length with a single
// non-zero bin at the specified index.
func makeSingleBinSpectrum(n, bin int, amplitude float64) []float64 {
	mag := make([]float64, n)
	if bin >= 0 && bin < n {
		mag[bin] = amplitude
	}

	return mag
}

// makeFlatSpectrum creates a spectrum where all bins have the same magnitude.
func makeFlatSpectrum(n int, amplitude float64) []float64 {
	mag := make([]float64, n)
	for i := range mag {
		mag[i] = amplitude
	}

	return mag
}

func TestCalculateEmpty(t *testing.T) {
	s := Calculate(nil, 48000)
	if s.BinCount != 0 {
		t.Fatalf("expected BinCount=0, got %d", s.BinCount)
	}

	if !math.IsInf(s.DC_dB, -1) {
		t.Fatalf("expected DC_dB=-Inf, got %f", s.DC_dB)
	}

	if !math.IsInf(s.Sum_dB, -1) {
		t.Fatalf("expected Sum_dB=-Inf, got %f", s.Sum_dB)
	}

	if !math.IsInf(s.Average_dB, -1) {
		t.Fatalf("expected Average_dB=-Inf, got %f", s.Average_dB)
	}
}

func TestCalculateAllZero(t *testing.T) {
	mag := make([]float64, 513) // FFT size 1024

	s := Calculate(mag, 48000)
	if s.BinCount != 513 {
		t.Fatalf("expected BinCount=513, got %d", s.BinCount)
	}

	if s.Sum != 0 {
		t.Fatalf("expected Sum=0, got %f", s.Sum)
	}

	if s.Energy != 0 {
		t.Fatalf("expected Energy=0, got %f", s.Energy)
	}

	if s.Centroid != 0 {
		t.Fatalf("expected Centroid=0, got %f", s.Centroid)
	}

	if s.Flatness != 0 {
		t.Fatalf("expected Flatness=0, got %f", s.Flatness)
	}
}

func TestCalculateSingleBin(t *testing.T) {
	// Single bin at 1 kHz in a 1024-point FFT at 48 kHz.
	// FFTSize = 1024, so magnitude length = 513.
	// Bin frequency = i * 48000 / 1024.
	// For 1 kHz: bin = 1000 * 1024 / 48000 ≈ 21.33, use bin 21.
	const (
		fftSize    = 1024
		sampleRate = 48000.0
		n          = fftSize/2 + 1
		bin        = 21
		amplitude  = 2.0
	)

	mag := makeSingleBinSpectrum(n, bin, amplitude)
	s := Calculate(mag, sampleRate)

	expectedFreq := float64(bin) * sampleRate / float64(fftSize)

	// Centroid should be exactly the frequency of the non-zero bin.
	if !almostEqual(s.Centroid, expectedFreq, tolerance) {
		t.Fatalf("Centroid: got %f, want %f", s.Centroid, expectedFreq)
	}

	// Spread should be 0 (all energy in one bin).
	if !almostEqual(s.Spread, 0, tolerance) {
		t.Fatalf("Spread: got %f, want 0", s.Spread)
	}

	// Flatness should be 0 (only one non-zero bin among many).
	if s.Flatness > 0.01 {
		t.Fatalf("Flatness: got %f, want ~0", s.Flatness)
	}

	// Max should be at the correct bin.
	if s.MaxBin != bin {
		t.Fatalf("MaxBin: got %d, want %d", s.MaxBin, bin)
	}

	if !almostEqual(s.Max, amplitude, tolerance) {
		t.Fatalf("Max: got %f, want %f", s.Max, amplitude)
	}

	// Energy = amplitude^2.
	if !almostEqual(s.Energy, amplitude*amplitude, tolerance) {
		t.Fatalf("Energy: got %f, want %f", s.Energy, amplitude*amplitude)
	}
}

func TestCalculateFlatSpectrum(t *testing.T) {
	const (
		fftSize    = 256
		sampleRate = 44100.0
		n          = fftSize/2 + 1
		amplitude  = 1.0
	)

	mag := makeFlatSpectrum(n, amplitude)
	s := Calculate(mag, sampleRate)

	nyquist := sampleRate / 2

	// For a flat spectrum, the centroid is at the midpoint of the frequency range.
	// centroid = sum(f_i * 1) / sum(1) = mean(f_i) = (0 + nyquist) / 2 = nyquist / 2
	expectedCentroid := nyquist / 2
	if !almostEqual(s.Centroid, expectedCentroid, 1.0) {
		t.Fatalf("Centroid: got %f, want ~%f", s.Centroid, expectedCentroid)
	}

	// Flatness should be close to 1 for a perfectly flat spectrum.
	// (excluding DC bin from flatness, but all non-DC bins are equal -> flatness = 1)
	if !almostEqual(s.Flatness, 1.0, 1e-6) {
		t.Fatalf("Flatness: got %f, want ~1.0", s.Flatness)
	}

	// Range should be 0.
	if !almostEqual(s.Range, 0, tolerance) {
		t.Fatalf("Range: got %f, want 0", s.Range)
	}
}

func TestCalculateTwoBins(t *testing.T) {
	// Two bins: bin 10 with amplitude 3, bin 20 with amplitude 1.
	const (
		fftSize    = 512
		sampleRate = 44100.0
		n          = fftSize/2 + 1
	)

	mag := make([]float64, n)
	mag[10] = 3.0
	mag[20] = 1.0

	s := Calculate(mag, sampleRate)

	f10 := float64(10) * sampleRate / float64(fftSize)
	f20 := float64(20) * sampleRate / float64(fftSize)

	// Centroid = (f10*3 + f20*1) / (3+1)
	expectedCentroid := (f10*3 + f20*1) / 4
	if !almostEqual(s.Centroid, expectedCentroid, tolerance) {
		t.Fatalf("Centroid: got %f, want %f", s.Centroid, expectedCentroid)
	}

	// Sum = 4
	if !almostEqual(s.Sum, 4.0, tolerance) {
		t.Fatalf("Sum: got %f, want 4", s.Sum)
	}

	// Energy = 9 + 1 = 10
	if !almostEqual(s.Energy, 10.0, tolerance) {
		t.Fatalf("Energy: got %f, want 10", s.Energy)
	}
}

func TestCalculateDCOnly(t *testing.T) {
	const (
		fftSize    = 128
		sampleRate = 16000.0
		n          = fftSize/2 + 1
	)

	mag := make([]float64, n)
	mag[0] = 5.0

	s := Calculate(mag, sampleRate)

	if !almostEqual(s.DC, 5.0, tolerance) {
		t.Fatalf("DC: got %f, want 5", s.DC)
	}

	// Centroid should be 0 (all energy in DC bin).
	if !almostEqual(s.Centroid, 0, tolerance) {
		t.Fatalf("Centroid: got %f, want 0", s.Centroid)
	}

	// Flatness should be 0 (only DC has energy, and DC is excluded from flatness).
	if s.Flatness != 0 {
		t.Fatalf("Flatness: got %f, want 0", s.Flatness)
	}
}

func TestCalculateSingleElement(t *testing.T) {
	// A single-element spectrum (DC only, no Nyquist).
	mag := []float64{3.5}
	s := Calculate(mag, 48000)

	if s.BinCount != 1 {
		t.Fatalf("BinCount: got %d, want 1", s.BinCount)
	}

	if !almostEqual(s.DC, 3.5, tolerance) {
		t.Fatalf("DC: got %f, want 3.5", s.DC)
	}

	if !almostEqual(s.Energy, 3.5*3.5, tolerance) {
		t.Fatalf("Energy: got %f, want %f", s.Energy, 3.5*3.5)
	}
	// Shape descriptors should be zero for a single bin.
	if s.Centroid != 0 {
		t.Fatalf("Centroid: got %f, want 0", s.Centroid)
	}
}

func TestCalculateFromComplexMatchesCalculate(t *testing.T) {
	spectrum := []complex128{
		complex(1, 0),
		complex(0, 2),
		complex(3, 4),
		complex(-1, 1),
		complex(0.5, -0.5),
	}

	sampleRate := 44100.0
	s1 := CalculateFromComplex(spectrum, sampleRate)

	mag := make([]float64, len(spectrum))
	for i, c := range spectrum {
		mag[i] = cmplx.Abs(c)
	}

	s2 := Calculate(mag, sampleRate)

	if s1.BinCount != s2.BinCount {
		t.Fatalf("BinCount mismatch: %d vs %d", s1.BinCount, s2.BinCount)
	}

	if !almostEqual(s1.Sum, s2.Sum, tolerance) {
		t.Fatalf("Sum mismatch: %f vs %f", s1.Sum, s2.Sum)
	}

	if !almostEqual(s1.Energy, s2.Energy, tolerance) {
		t.Fatalf("Energy mismatch: %f vs %f", s1.Energy, s2.Energy)
	}

	if !almostEqual(s1.Centroid, s2.Centroid, tolerance) {
		t.Fatalf("Centroid mismatch: %f vs %f", s1.Centroid, s2.Centroid)
	}

	if !almostEqual(s1.Flatness, s2.Flatness, tolerance) {
		t.Fatalf("Flatness mismatch: %f vs %f", s1.Flatness, s2.Flatness)
	}

	if !almostEqual(s1.Rolloff, s2.Rolloff, tolerance) {
		t.Fatalf("Rolloff mismatch: %f vs %f", s1.Rolloff, s2.Rolloff)
	}

	if !almostEqual(s1.Bandwidth, s2.Bandwidth, tolerance) {
		t.Fatalf("Bandwidth mismatch: %f vs %f", s1.Bandwidth, s2.Bandwidth)
	}
}

func TestIndividualFunctionsMatchCalculate(t *testing.T) {
	const (
		fftSize    = 512
		sampleRate = 48000.0
		n          = fftSize/2 + 1
	)

	// Create a spectrum with a few non-zero bins.
	mag := make([]float64, n)
	mag[10] = 1.0
	mag[20] = 2.0
	mag[30] = 0.5
	mag[50] = 1.5

	s := Calculate(mag, sampleRate)

	// Centroid.
	cent := Centroid(mag, sampleRate)
	if !almostEqual(cent, s.Centroid, tolerance) {
		t.Fatalf("Centroid: individual=%f, Calculate=%f", cent, s.Centroid)
	}

	// Flatness.
	flat := Flatness(mag)
	if !almostEqual(flat, s.Flatness, tolerance) {
		t.Fatalf("Flatness: individual=%f, Calculate=%f", flat, s.Flatness)
	}

	// Rolloff (85%).
	roll := Rolloff(mag, sampleRate, 0.85)
	if !almostEqual(roll, s.Rolloff, tolerance) {
		t.Fatalf("Rolloff: individual=%f, Calculate=%f", roll, s.Rolloff)
	}

	// Bandwidth.
	bw := Bandwidth(mag, sampleRate)
	if !almostEqual(bw, s.Bandwidth, tolerance) {
		t.Fatalf("Bandwidth: individual=%f, Calculate=%f", bw, s.Bandwidth)
	}
}

func TestCentroidSingleBin(t *testing.T) {
	const (
		fftSize    = 1024
		sampleRate = 48000.0
		n          = fftSize/2 + 1
		bin        = 100
	)

	mag := makeSingleBinSpectrum(n, bin, 1.0)
	expectedFreq := float64(bin) * sampleRate / float64(fftSize)

	cent := Centroid(mag, sampleRate)
	if !almostEqual(cent, expectedFreq, tolerance) {
		t.Fatalf("Centroid: got %f, want %f", cent, expectedFreq)
	}
}

func TestCentroidEmpty(t *testing.T) {
	c := Centroid(nil, 48000)
	if c != 0 {
		t.Fatalf("Centroid of nil: got %f, want 0", c)
	}

	c = Centroid([]float64{0}, 48000)
	if c != 0 {
		t.Fatalf("Centroid of single zero: got %f, want 0", c)
	}
}

func TestFlatnessPerfectlyFlat(t *testing.T) {
	// All non-DC bins equal -> flatness = 1.
	mag := makeFlatSpectrum(129, 1.0)

	flat := Flatness(mag)
	if !almostEqual(flat, 1.0, 1e-9) {
		t.Fatalf("Flatness of flat spectrum: got %f, want 1.0", flat)
	}
}

func TestFlatnessSingleTone(t *testing.T) {
	// Only one non-DC bin is non-zero -> flatness ~ 0.
	mag := make([]float64, 129)
	mag[50] = 1.0

	flat := Flatness(mag)
	if flat > 0.01 {
		t.Fatalf("Flatness of single tone: got %f, want ~0", flat)
	}
}

func TestFlatnessAllZero(t *testing.T) {
	mag := make([]float64, 129)

	flat := Flatness(mag)
	if flat != 0 {
		t.Fatalf("Flatness of all-zero: got %f, want 0", flat)
	}
}

func TestFlatnessEmpty(t *testing.T) {
	flat := Flatness(nil)
	if flat != 0 {
		t.Fatalf("Flatness of nil: got %f, want 0", flat)
	}
}

func TestRolloffKnownDistribution(t *testing.T) {
	// Create a spectrum with known energy distribution.
	// 5 bins, each with magnitude 1 => energy per bin = 1, total = 5.
	// 85% of 5 = 4.25, so rolloff at bin 4 (cumulative energy reaches 4 >= 4.25 at bin 4? No, 4.25).
	// Actually: cumulative at bin 0=1, bin 1=2, bin 2=3, bin 3=4, bin 4=5.
	// 85% of 5 = 4.25. Cumulative reaches 4.25 between bin 3 and bin 4, but we return bin 4 (first >= threshold).
	// Wait, cumulative at bin 4 = 5 >= 4.25. But bin 3 = 4 < 4.25. So rolloff bin = 4.
	mag := []float64{1, 1, 1, 1, 1}
	sampleRate := 8.0 // Nyquist = 4 Hz, fftSize = 8, binFreq = i * 8 / 8 = i Hz
	roll := Rolloff(mag, sampleRate, 0.85)

	expectedFreq := binFreq(4, sampleRate, 5) // bin 4 frequency
	if !almostEqual(roll, expectedFreq, tolerance) {
		t.Fatalf("Rolloff: got %f, want %f", roll, expectedFreq)
	}
}

func TestRolloffConcentratedEnergy(t *testing.T) {
	// All energy in bin 0 (DC).
	mag := make([]float64, 33) // FFTSize = 64
	mag[0] = 10.0

	roll := Rolloff(mag, 48000, 0.85)
	if !almostEqual(roll, 0, tolerance) {
		t.Fatalf("Rolloff DC-only: got %f, want 0", roll)
	}
}

func TestRolloffEmpty(t *testing.T) {
	roll := Rolloff(nil, 48000, 0.85)
	if roll != 0 {
		t.Fatalf("Rolloff empty: got %f, want 0", roll)
	}
}

func TestBandwidthSinglePeak(t *testing.T) {
	// Create a spectrum with a peak at bin 50 and a triangle shape around it.
	const (
		fftSize    = 1024
		sampleRate = 48000.0
		n          = fftSize/2 + 1
		peakBin    = 50
		peakAmp    = 10.0
	)

	mag := make([]float64, n)
	// Create a triangular peak spanning about 20 bins.
	for i := 40; i <= 60; i++ {
		dist := math.Abs(float64(i - peakBin))
		mag[i] = peakAmp * math.Max(0, 1-dist/10)
	}

	bandWidth := Bandwidth(mag, sampleRate)
	binWidth := sampleRate / float64(fftSize)

	// The -3 dB point is at peak/sqrt(2) ≈ 7.07.
	// The triangle reaches 7.07 at dist ≈ 2.93 bins from center.
	// So bandwidth ≈ 2 * 2.93 * binWidth ≈ 5.86 * binWidth.
	expectedBW := 2 * (1 - 1/math.Sqrt2) * 10 * binWidth
	if math.Abs(bandWidth-expectedBW) > 2*binWidth {
		t.Fatalf("Bandwidth: got %f, expected ~%f (binWidth=%f)", bandWidth, expectedBW, binWidth)
	}

	if bandWidth <= 0 {
		t.Fatalf("Bandwidth should be positive, got %f", bandWidth)
	}
}

func TestBandwidthFlat(t *testing.T) {
	// A flat spectrum: peak is everywhere, so -3 dB threshold = 1/sqrt(2).
	// All bins are above threshold, so bandwidth = Nyquist - 0 = Nyquist.
	mag := makeFlatSpectrum(129, 1.0)
	sampleRate := 44100.0
	bw := Bandwidth(mag, sampleRate)

	nyquist := sampleRate / 2
	if !almostEqual(bw, nyquist, 1.0) {
		t.Fatalf("Bandwidth flat: got %f, want ~%f", bw, nyquist)
	}
}

func TestBandwidthZero(t *testing.T) {
	mag := make([]float64, 129)

	bw := Bandwidth(mag, 48000)
	if bw != 0 {
		t.Fatalf("Bandwidth zero: got %f, want 0", bw)
	}
}

func TestBandwidthEmpty(t *testing.T) {
	bw := Bandwidth(nil, 48000)
	if bw != 0 {
		t.Fatalf("Bandwidth empty: got %f, want 0", bw)
	}
}

// Table-driven tests.
func TestCalculateTableDriven(t *testing.T) {
	tests := []struct {
		name       string
		magnitude  []float64
		sampleRate float64
		checkFn    func(t *testing.T, s Stats)
	}{
		{
			name:       "nil_magnitude",
			magnitude:  nil,
			sampleRate: 48000,
			checkFn: func(t *testing.T, s Stats) {
				t.Helper()

				if s.BinCount != 0 {
					t.Fatalf("BinCount: got %d, want 0", s.BinCount)
				}
			},
		},
		{
			name:       "single_bin_value",
			magnitude:  []float64{7.0},
			sampleRate: 48000,
			checkFn: func(t *testing.T, s Stats) {
				t.Helper()

				if s.BinCount != 1 {
					t.Fatalf("BinCount: got %d, want 1", s.BinCount)
				}

				if !almostEqual(s.DC, 7.0, tolerance) {
					t.Fatalf("DC: got %f, want 7.0", s.DC)
				}
			},
		},
		{
			name:       "two_equal_bins",
			magnitude:  []float64{1, 1},
			sampleRate: 44100,
			checkFn: func(t *testing.T, s Stats) {
				t.Helper()

				if s.BinCount != 2 {
					t.Fatalf("BinCount: got %d, want 2", s.BinCount)
				}
				// Centroid = (0*1 + 22050*1) / 2 = 11025
				if !almostEqual(s.Centroid, 11025, 1.0) {
					t.Fatalf("Centroid: got %f, want 11025", s.Centroid)
				}
			},
		},
		{
			name:       "all_zeros_large",
			magnitude:  make([]float64, 4097),
			sampleRate: 96000,
			checkFn: func(t *testing.T, s Stats) {
				t.Helper()

				if s.Energy != 0 {
					t.Fatalf("Energy: got %f, want 0", s.Energy)
				}

				if s.Centroid != 0 {
					t.Fatalf("Centroid: got %f, want 0", s.Centroid)
				}
			},
		},
		{
			name: "monotonically_increasing",
			magnitude: func() []float64 {
				mag := make([]float64, 33)
				for i := range mag {
					mag[i] = float64(i)
				}

				return mag
			}(),
			sampleRate: 48000,
			checkFn: func(t *testing.T, s Stats) {
				t.Helper()

				if s.MaxBin != 32 {
					t.Fatalf("MaxBin: got %d, want 32", s.MaxBin)
				}

				if s.MinBin != 0 {
					t.Fatalf("MinBin: got %d, want 0", s.MinBin)
				}

				if !almostEqual(s.Min, 0, tolerance) {
					t.Fatalf("Min: got %f, want 0", s.Min)
				}

				if !almostEqual(s.Max, 32, tolerance) {
					t.Fatalf("Max: got %f, want 32", s.Max)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Calculate(tt.magnitude, tt.sampleRate)
			tt.checkFn(t, s)
		})
	}
}

func TestDBConversion(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		want  float64
	}{
		{"unity", 1.0, 0},
		{"ten", 10.0, 20},
		{"hundred", 100.0, 40},
		{"tenth", 0.1, -20},
		{"zero", 0, math.Inf(-1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toDB(tt.value)
			if !almostEqual(got, tt.want, 1e-9) {
				t.Fatalf("toDB(%f): got %f, want %f", tt.value, got, tt.want)
			}
		})
	}
}

func TestSpreadSingleBin(t *testing.T) {
	const (
		fftSize    = 512
		sampleRate = 48000.0
		n          = fftSize/2 + 1
	)

	mag := makeSingleBinSpectrum(n, 50, 1.0)

	s := Calculate(mag, sampleRate)
	if !almostEqual(s.Spread, 0, tolerance) {
		t.Fatalf("Spread for single bin: got %f, want 0", s.Spread)
	}
}

func TestSpreadTwoBinsSymmetric(t *testing.T) {
	// Two equal bins equidistant from center.
	const (
		fftSize    = 512
		sampleRate = 48000.0
		n          = fftSize/2 + 1
	)

	mag := make([]float64, n)
	mag[100] = 1.0
	mag[200] = 1.0
	s := Calculate(mag, sampleRate)

	f100 := float64(100) * sampleRate / float64(fftSize)
	f200 := float64(200) * sampleRate / float64(fftSize)
	// Spread = sqrt(((f100-cent)^2*1 + (f200-cent)^2*1) / 2) = |f200-f100|/2
	expectedSpread := (f200 - f100) / 2
	if !almostEqual(s.Spread, expectedSpread, 1.0) {
		t.Fatalf("Spread: got %f, want %f", s.Spread, expectedSpread)
	}
}

func TestPowerComputation(t *testing.T) {
	mag := []float64{1, 2, 3, 4, 5}
	s := Calculate(mag, 48000)

	expectedEnergy := 1.0 + 4.0 + 9.0 + 16.0 + 25.0
	if !almostEqual(s.Energy, expectedEnergy, tolerance) {
		t.Fatalf("Energy: got %f, want %f", s.Energy, expectedEnergy)
	}

	expectedPower := expectedEnergy / 5.0
	if !almostEqual(s.Power, expectedPower, tolerance) {
		t.Fatalf("Power: got %f, want %f", s.Power, expectedPower)
	}
}
