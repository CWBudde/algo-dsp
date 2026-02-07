package time

import (
	"math"
	"testing"
)

const tolerance = 1e-10

func almostEqual(a, b, tol float64) bool {
	if math.IsInf(a, -1) && math.IsInf(b, -1) {
		return true
	}
	if math.IsInf(a, 1) && math.IsInf(b, 1) {
		return true
	}
	if math.IsNaN(a) && math.IsNaN(b) {
		return true
	}
	return math.Abs(a-b) <= tol
}

// generateSine creates a sine wave with the given amplitude, frequency, and sample rate.
// It generates exactly numCycles full cycles.
func generateSine(amplitude, freq, sampleRate float64, numCycles int) []float64 {
	samplesPerCycle := int(sampleRate / freq)
	n := samplesPerCycle * numCycles
	out := make([]float64, n)
	for i := range out {
		out[i] = amplitude * math.Sin(2*math.Pi*freq*float64(i)/sampleRate)
	}
	return out
}

// generateDC creates a constant signal.
func generateDC(value float64, length int) []float64 {
	out := make([]float64, length)
	for i := range out {
		out[i] = value
	}
	return out
}

// generateSquare creates a +val/-val alternating square wave.
func generateSquare(val float64, length int) []float64 {
	out := make([]float64, length)
	for i := range out {
		if i%2 == 0 {
			out[i] = val
		} else {
			out[i] = -val
		}
	}
	return out
}

// generateUniform creates a uniformly spaced signal from -1 to +1 (inclusive).
func generateUniform(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = -1 + 2*float64(i)/float64(n-1)
	}
	return out
}

func TestCalculate_DCSignal(t *testing.T) {
	signal := generateDC(1.0, 1000)
	s := Calculate(signal)

	if s.Length != 1000 {
		t.Errorf("Length: got %d, want 1000", s.Length)
	}
	if !almostEqual(s.DC, 1.0, tolerance) {
		t.Errorf("DC: got %g, want 1.0", s.DC)
	}
	if !almostEqual(s.RMS, 1.0, tolerance) {
		t.Errorf("RMS: got %g, want 1.0", s.RMS)
	}
	if !almostEqual(s.Peak, 1.0, tolerance) {
		t.Errorf("Peak: got %g, want 1.0", s.Peak)
	}
	if !almostEqual(s.CrestFactor, 1.0, tolerance) {
		t.Errorf("CrestFactor: got %g, want 1.0", s.CrestFactor)
	}
	if s.ZeroCrossings != 0 {
		t.Errorf("ZeroCrossings: got %d, want 0", s.ZeroCrossings)
	}
	if !almostEqual(s.Variance, 0, tolerance) {
		t.Errorf("Variance: got %g, want 0", s.Variance)
	}
	if !almostEqual(s.Skewness, 0, tolerance) {
		t.Errorf("Skewness: got %g, want 0", s.Skewness)
	}
	if !almostEqual(s.Max, 1.0, tolerance) {
		t.Errorf("Max: got %g, want 1.0", s.Max)
	}
	if !almostEqual(s.Min, 1.0, tolerance) {
		t.Errorf("Min: got %g, want 1.0", s.Min)
	}
	if !almostEqual(s.Range, 0, tolerance) {
		t.Errorf("Range: got %g, want 0", s.Range)
	}
	if !almostEqual(s.Energy, 1000, tolerance) {
		t.Errorf("Energy: got %g, want 1000", s.Energy)
	}
	if !almostEqual(s.Power, 1.0, tolerance) {
		t.Errorf("Power: got %g, want 1.0", s.Power)
	}
	// dB checks.
	if !almostEqual(s.DC_dB, 0, tolerance) {
		t.Errorf("DC_dB: got %g, want 0", s.DC_dB)
	}
	if !almostEqual(s.RMS_dB, 0, tolerance) {
		t.Errorf("RMS_dB: got %g, want 0", s.RMS_dB)
	}
	if !almostEqual(s.CrestFactor_dB, 0, tolerance) {
		t.Errorf("CrestFactor_dB: got %g, want 0", s.CrestFactor_dB)
	}
}

func TestCalculate_SineWave(t *testing.T) {
	// 1000 Hz sine at 48000 SR, 10 full cycles.
	signal := generateSine(1.0, 1000, 48000, 10)
	s := Calculate(signal)

	expectedRMS := 1.0 / math.Sqrt(2)
	if !almostEqual(s.RMS, expectedRMS, 1e-6) {
		t.Errorf("RMS: got %g, want %g", s.RMS, expectedRMS)
	}
	if !almostEqual(s.DC, 0, 1e-10) {
		t.Errorf("DC: got %g, want ~0", s.DC)
	}
	// Peak should be very close to 1.0 (discrete sampling may not hit exact 1.0).
	if !almostEqual(s.Peak, 1.0, 1e-3) {
		t.Errorf("Peak: got %g, want ~1.0", s.Peak)
	}
	expectedCrest := 1.0 / expectedRMS
	if !almostEqual(s.CrestFactor, expectedCrest, 1e-3) {
		t.Errorf("CrestFactor: got %g, want %g", s.CrestFactor, expectedCrest)
	}
	// Variance of sin = 0.5
	if !almostEqual(s.Variance, 0.5, 1e-6) {
		t.Errorf("Variance: got %g, want 0.5", s.Variance)
	}
	// Skewness of a symmetric sine wave over full cycles should be ~0.
	if !almostEqual(s.Skewness, 0, 1e-6) {
		t.Errorf("Skewness: got %g, want ~0", s.Skewness)
	}
	// Zero crossings: 2 per cycle nominally, but sin(0) = 0 exactly at
	// every half-cycle boundary (samples 0, 24, 48, ...), so the product
	// signal[i-1]*signal[i] is 0 rather than negative, losing one crossing
	// at the very start. This yields 19 crossings for 10 full cycles.
	if s.ZeroCrossings != 19 {
		t.Errorf("ZeroCrossings: got %d, want 19", s.ZeroCrossings)
	}
}

func TestCalculate_SquareWave(t *testing.T) {
	signal := generateSquare(1.0, 1000)
	s := Calculate(signal)

	if !almostEqual(s.DC, 0, tolerance) {
		t.Errorf("DC: got %g, want 0", s.DC)
	}
	if !almostEqual(s.RMS, 1.0, tolerance) {
		t.Errorf("RMS: got %g, want 1.0", s.RMS)
	}
	if !almostEqual(s.Peak, 1.0, tolerance) {
		t.Errorf("Peak: got %g, want 1.0", s.Peak)
	}
	if !almostEqual(s.CrestFactor, 1.0, tolerance) {
		t.Errorf("CrestFactor: got %g, want 1.0", s.CrestFactor)
	}
	if !almostEqual(s.Max, 1.0, tolerance) {
		t.Errorf("Max: got %g, want 1.0", s.Max)
	}
	if !almostEqual(s.Min, -1.0, tolerance) {
		t.Errorf("Min: got %g, want -1.0", s.Min)
	}
	if !almostEqual(s.Range, 2.0, tolerance) {
		t.Errorf("Range: got %g, want 2.0", s.Range)
	}
	// Every adjacent pair changes sign: 999 crossings.
	if s.ZeroCrossings != 999 {
		t.Errorf("ZeroCrossings: got %d, want 999", s.ZeroCrossings)
	}
	// Variance of +1/-1 square wave = 1.
	if !almostEqual(s.Variance, 1.0, tolerance) {
		t.Errorf("Variance: got %g, want 1.0", s.Variance)
	}
}

func TestCalculate_EmptySignal(t *testing.T) {
	s := Calculate(nil)

	if s.Length != 0 {
		t.Errorf("Length: got %d, want 0", s.Length)
	}
	if s.DC != 0 {
		t.Errorf("DC: got %g, want 0", s.DC)
	}
	if s.RMS != 0 {
		t.Errorf("RMS: got %g, want 0", s.RMS)
	}
	if !math.IsInf(s.DC_dB, -1) {
		t.Errorf("DC_dB: got %g, want -Inf", s.DC_dB)
	}
	if !math.IsInf(s.RMS_dB, -1) {
		t.Errorf("RMS_dB: got %g, want -Inf", s.RMS_dB)
	}
	if !math.IsInf(s.Peak_dB, -1) {
		t.Errorf("Peak_dB: got %g, want -Inf", s.Peak_dB)
	}
	if !math.IsInf(s.Range_dB, -1) {
		t.Errorf("Range_dB: got %g, want -Inf", s.Range_dB)
	}
	if !math.IsInf(s.CrestFactor_dB, -1) {
		t.Errorf("CrestFactor_dB: got %g, want -Inf", s.CrestFactor_dB)
	}
}

func TestCalculate_SingleSample(t *testing.T) {
	s := Calculate([]float64{3.5})

	if s.Length != 1 {
		t.Errorf("Length: got %d, want 1", s.Length)
	}
	if !almostEqual(s.DC, 3.5, tolerance) {
		t.Errorf("DC: got %g, want 3.5", s.DC)
	}
	if !almostEqual(s.RMS, 3.5, tolerance) {
		t.Errorf("RMS: got %g, want 3.5", s.RMS)
	}
	if !almostEqual(s.Peak, 3.5, tolerance) {
		t.Errorf("Peak: got %g, want 3.5", s.Peak)
	}
	if !almostEqual(s.CrestFactor, 1.0, tolerance) {
		t.Errorf("CrestFactor: got %g, want 1.0", s.CrestFactor)
	}
	if s.ZeroCrossings != 0 {
		t.Errorf("ZeroCrossings: got %d, want 0", s.ZeroCrossings)
	}
	if !almostEqual(s.Variance, 0, tolerance) {
		t.Errorf("Variance: got %g, want 0", s.Variance)
	}
}

func TestCalculate_ZeroSignal(t *testing.T) {
	signal := make([]float64, 100)
	s := Calculate(signal)

	if !almostEqual(s.DC, 0, tolerance) {
		t.Errorf("DC: got %g, want 0", s.DC)
	}
	if !almostEqual(s.RMS, 0, tolerance) {
		t.Errorf("RMS: got %g, want 0", s.RMS)
	}
	if !almostEqual(s.CrestFactor, 0, tolerance) {
		t.Errorf("CrestFactor: got %g, want 0", s.CrestFactor)
	}
	if !almostEqual(s.CrestFactor_dB, 0, tolerance) {
		t.Errorf("CrestFactor_dB: got %g, want 0", s.CrestFactor_dB)
	}
	if !math.IsInf(s.DC_dB, -1) {
		t.Errorf("DC_dB: got %g, want -Inf", s.DC_dB)
	}
	if !math.IsInf(s.RMS_dB, -1) {
		t.Errorf("RMS_dB: got %g, want -Inf", s.RMS_dB)
	}
	if !math.IsInf(s.Peak_dB, -1) {
		t.Errorf("Peak_dB: got %g, want -Inf", s.Peak_dB)
	}
}

func TestCalculate_UniformDistribution(t *testing.T) {
	// Large uniform distribution from -1 to +1 for known moments.
	n := 100001
	signal := generateUniform(n)
	s := Calculate(signal)

	// Mean should be ~0.
	if !almostEqual(s.DC, 0, 1e-10) {
		t.Errorf("DC: got %g, want ~0", s.DC)
	}
	// Population variance of uniform [-1, 1] = 1/3.
	if !almostEqual(s.Variance, 1.0/3.0, 1e-4) {
		t.Errorf("Variance: got %g, want %g", s.Variance, 1.0/3.0)
	}
	// Skewness should be ~0 (symmetric).
	if !almostEqual(s.Skewness, 0, 1e-4) {
		t.Errorf("Skewness: got %g, want ~0", s.Skewness)
	}
	// Excess kurtosis of uniform = -6/5 = -1.2.
	if !almostEqual(s.Kurtosis, -6.0/5.0, 1e-3) {
		t.Errorf("Kurtosis: got %g, want %g", s.Kurtosis, -6.0/5.0)
	}
}

func TestCalculate_MaxMinPositions(t *testing.T) {
	signal := []float64{0, 1, -2, 3, -4, 5}
	s := Calculate(signal)

	if s.MaxPos != 5 {
		t.Errorf("MaxPos: got %d, want 5", s.MaxPos)
	}
	if s.MinPos != 4 {
		t.Errorf("MinPos: got %d, want 4", s.MinPos)
	}
	if !almostEqual(s.Max, 5, tolerance) {
		t.Errorf("Max: got %g, want 5", s.Max)
	}
	if !almostEqual(s.Min, -4, tolerance) {
		t.Errorf("Min: got %g, want -4", s.Min)
	}
	if !almostEqual(s.Peak, 5, tolerance) {
		t.Errorf("Peak: got %g, want 5", s.Peak)
	}
}

func TestCalculate_dBValues(t *testing.T) {
	signal := generateDC(2.0, 100)
	s := Calculate(signal)

	wantdB := 20 * math.Log10(2.0)
	if !almostEqual(s.DC_dB, wantdB, tolerance) {
		t.Errorf("DC_dB: got %g, want %g", s.DC_dB, wantdB)
	}
	if !almostEqual(s.RMS_dB, wantdB, tolerance) {
		t.Errorf("RMS_dB: got %g, want %g", s.RMS_dB, wantdB)
	}
	if !almostEqual(s.Peak_dB, wantdB, tolerance) {
		t.Errorf("Peak_dB: got %g, want %g", s.Peak_dB, wantdB)
	}
}

func TestCalculate_NegativeDC(t *testing.T) {
	signal := generateDC(-0.5, 100)
	s := Calculate(signal)

	if !almostEqual(s.DC, -0.5, tolerance) {
		t.Errorf("DC: got %g, want -0.5", s.DC)
	}
	// ampTodB uses absolute value.
	wantdB := 20 * math.Log10(0.5)
	if !almostEqual(s.DC_dB, wantdB, tolerance) {
		t.Errorf("DC_dB: got %g, want %g", s.DC_dB, wantdB)
	}
}

// --- Individual function tests ---

func TestRMS(t *testing.T) {
	tests := []struct {
		name   string
		signal []float64
		want   float64
	}{
		{"empty", nil, 0},
		{"dc", generateDC(1.0, 100), 1.0},
		{"single", []float64{4.0}, 4.0},
		{"square", generateSquare(1.0, 1000), 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RMS(tt.signal)
			if !almostEqual(got, tt.want, 1e-10) {
				t.Errorf("RMS(%s): got %g, want %g", tt.name, got, tt.want)
			}
		})
	}
}

func TestDC(t *testing.T) {
	tests := []struct {
		name   string
		signal []float64
		want   float64
	}{
		{"empty", nil, 0},
		{"dc", generateDC(3.0, 100), 3.0},
		{"symmetric", generateSquare(1.0, 1000), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DC(tt.signal)
			if !almostEqual(got, tt.want, 1e-10) {
				t.Errorf("DC(%s): got %g, want %g", tt.name, got, tt.want)
			}
		})
	}
}

func TestPeak(t *testing.T) {
	tests := []struct {
		name   string
		signal []float64
		want   float64
	}{
		{"empty", nil, 0},
		{"positive", []float64{1, 2, 3}, 3},
		{"negative", []float64{-5, -1, -3}, 5},
		{"mixed", []float64{2, -7, 3}, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Peak(tt.signal)
			if !almostEqual(got, tt.want, tolerance) {
				t.Errorf("Peak(%s): got %g, want %g", tt.name, got, tt.want)
			}
		})
	}
}

func TestCrestFactor(t *testing.T) {
	tests := []struct {
		name   string
		signal []float64
		want   float64
	}{
		{"empty", nil, 0},
		{"dc", generateDC(1.0, 100), 1.0},
		{"zero", make([]float64, 10), 0},
		{"square", generateSquare(1.0, 1000), 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CrestFactor(tt.signal)
			if !almostEqual(got, tt.want, 1e-10) {
				t.Errorf("CrestFactor(%s): got %g, want %g", tt.name, got, tt.want)
			}
		})
	}
}

func TestZeroCrossings(t *testing.T) {
	tests := []struct {
		name   string
		signal []float64
		want   int
	}{
		{"empty", nil, 0},
		{"single", []float64{1}, 0},
		{"no_crossings", []float64{1, 2, 3}, 0},
		{"one_crossing", []float64{1, -1}, 1},
		{"alternating", generateSquare(1.0, 10), 9},
		{"through_zero", []float64{1, 0, -1}, 0}, // 1*0=0, 0*(-1)=0, neither < 0
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ZeroCrossings(tt.signal)
			if got != tt.want {
				t.Errorf("ZeroCrossings(%s): got %d, want %d", tt.name, got, tt.want)
			}
		})
	}
}

func TestMoments(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		mean, variance, skewness, kurtosis := Moments(nil)
		if mean != 0 || variance != 0 || skewness != 0 || kurtosis != 0 {
			t.Errorf("expected all zeros, got mean=%g var=%g skew=%g kurt=%g",
				mean, variance, skewness, kurtosis)
		}
	})

	t.Run("dc", func(t *testing.T) {
		mean, variance, skewness, kurtosis := Moments(generateDC(5.0, 1000))
		if !almostEqual(mean, 5.0, tolerance) {
			t.Errorf("mean: got %g, want 5.0", mean)
		}
		if !almostEqual(variance, 0, tolerance) {
			t.Errorf("variance: got %g, want 0", variance)
		}
		if !almostEqual(skewness, 0, tolerance) {
			t.Errorf("skewness: got %g, want 0", skewness)
		}
		if !almostEqual(kurtosis, 0, tolerance) {
			t.Errorf("kurtosis: got %g, want 0", kurtosis)
		}
	})

	t.Run("uniform", func(t *testing.T) {
		signal := generateUniform(100001)
		mean, variance, skewness, kurtosis := Moments(signal)

		if !almostEqual(mean, 0, 1e-10) {
			t.Errorf("mean: got %g, want ~0", mean)
		}
		if !almostEqual(variance, 1.0/3.0, 1e-4) {
			t.Errorf("variance: got %g, want %g", variance, 1.0/3.0)
		}
		if !almostEqual(skewness, 0, 1e-4) {
			t.Errorf("skewness: got %g, want ~0", skewness)
		}
		if !almostEqual(kurtosis, -6.0/5.0, 1e-3) {
			t.Errorf("kurtosis: got %g, want %g", kurtosis, -6.0/5.0)
		}
	})

	t.Run("matches_calculate", func(t *testing.T) {
		signal := generateSine(1.0, 1000, 48000, 5)
		s := Calculate(signal)
		mean, variance, skewness, kurtosis := Moments(signal)

		if !almostEqual(mean, s.DC, tolerance) {
			t.Errorf("mean mismatch: Moments=%g, Calculate=%g", mean, s.DC)
		}
		if !almostEqual(variance, s.Variance, tolerance) {
			t.Errorf("variance mismatch: Moments=%g, Calculate=%g", variance, s.Variance)
		}
		if !almostEqual(skewness, s.Skewness, tolerance) {
			t.Errorf("skewness mismatch: Moments=%g, Calculate=%g", skewness, s.Skewness)
		}
		if !almostEqual(kurtosis, s.Kurtosis, tolerance) {
			t.Errorf("kurtosis mismatch: Moments=%g, Calculate=%g", kurtosis, s.Kurtosis)
		}
	})
}

// --- Individual functions match Calculate ---

func TestIndividualFunctionsMatchCalculate(t *testing.T) {
	signals := map[string][]float64{
		"dc":     generateDC(2.5, 500),
		"sine":   generateSine(1.0, 1000, 48000, 5),
		"square": generateSquare(1.0, 1000),
	}

	for name, signal := range signals {
		t.Run(name, func(t *testing.T) {
			s := Calculate(signal)

			rms := RMS(signal)
			if !almostEqual(rms, s.RMS, tolerance) {
				t.Errorf("RMS: standalone=%g, Calculate=%g", rms, s.RMS)
			}

			dc := DC(signal)
			// DC uses Kahan summation so may differ very slightly from
			// Welford mean. Use a slightly looser tolerance.
			if !almostEqual(dc, s.DC, 1e-9) {
				t.Errorf("DC: standalone=%g, Calculate=%g", dc, s.DC)
			}

			peak := Peak(signal)
			if !almostEqual(peak, s.Peak, tolerance) {
				t.Errorf("Peak: standalone=%g, Calculate=%g", peak, s.Peak)
			}

			cf := CrestFactor(signal)
			if !almostEqual(cf, s.CrestFactor, tolerance) {
				t.Errorf("CrestFactor: standalone=%g, Calculate=%g", cf, s.CrestFactor)
			}

			zc := ZeroCrossings(signal)
			if zc != s.ZeroCrossings {
				t.Errorf("ZeroCrossings: standalone=%d, Calculate=%d", zc, s.ZeroCrossings)
			}
		})
	}
}

// --- Streaming stats tests ---

func TestStreamingStats_MatchesCalculate(t *testing.T) {
	signals := map[string][]float64{
		"dc":      generateDC(1.0, 1000),
		"sine":    generateSine(1.0, 1000, 48000, 10),
		"square":  generateSquare(1.0, 1000),
		"uniform": generateUniform(10001),
	}

	// Block sizes to split the signal into.
	blockSizes := []int{1, 7, 64, 256, 1000}

	for name, signal := range signals {
		for _, bs := range blockSizes {
			t.Run(name+"/block_"+itoa(bs), func(t *testing.T) {
				expected := Calculate(signal)
				ss := NewStreamingStats()

				for i := 0; i < len(signal); i += bs {
					end := i + bs
					if end > len(signal) {
						end = len(signal)
					}
					ss.Update(signal[i:end])
				}

				got := ss.Result()
				compareStats(t, got, expected)
			})
		}
	}
}

func TestStreamingStats_Empty(t *testing.T) {
	ss := NewStreamingStats()
	s := ss.Result()

	if s.Length != 0 {
		t.Errorf("Length: got %d, want 0", s.Length)
	}
	if !math.IsInf(s.DC_dB, -1) {
		t.Errorf("DC_dB: got %g, want -Inf", s.DC_dB)
	}
}

func TestStreamingStats_Reset(t *testing.T) {
	ss := NewStreamingStats()
	ss.Update([]float64{1, 2, 3, 4, 5})
	ss.Reset()

	s := ss.Result()
	if s.Length != 0 {
		t.Errorf("after Reset, Length: got %d, want 0", s.Length)
	}

	// Use after reset.
	ss.Update([]float64{10})
	s = ss.Result()
	if s.Length != 1 {
		t.Errorf("after Reset+Update, Length: got %d, want 1", s.Length)
	}
	if !almostEqual(s.DC, 10, tolerance) {
		t.Errorf("after Reset+Update, DC: got %g, want 10", s.DC)
	}
}

func TestStreamingStats_SingleSample(t *testing.T) {
	expected := Calculate([]float64{42})

	ss := NewStreamingStats()
	ss.Update([]float64{42})
	got := ss.Result()

	compareStats(t, got, expected)
}

func TestStreamingStats_SampleBySample(t *testing.T) {
	signal := generateSine(1.0, 1000, 48000, 2)
	expected := Calculate(signal)

	ss := NewStreamingStats()
	for _, x := range signal {
		ss.Update([]float64{x})
	}
	got := ss.Result()

	compareStats(t, got, expected)
}

// compareStats checks that two Stats structs are equal within tolerance.
func compareStats(t *testing.T, got, want Stats) {
	t.Helper()

	if got.Length != want.Length {
		t.Errorf("Length: got %d, want %d", got.Length, want.Length)
	}
	checkFloat(t, "DC", got.DC, want.DC)
	checkFloat(t, "DC_dB", got.DC_dB, want.DC_dB)
	checkFloat(t, "RMS", got.RMS, want.RMS)
	checkFloat(t, "RMS_dB", got.RMS_dB, want.RMS_dB)
	checkFloat(t, "Max", got.Max, want.Max)
	if got.MaxPos != want.MaxPos {
		t.Errorf("MaxPos: got %d, want %d", got.MaxPos, want.MaxPos)
	}
	checkFloat(t, "Min", got.Min, want.Min)
	if got.MinPos != want.MinPos {
		t.Errorf("MinPos: got %d, want %d", got.MinPos, want.MinPos)
	}
	checkFloat(t, "Peak", got.Peak, want.Peak)
	checkFloat(t, "Peak_dB", got.Peak_dB, want.Peak_dB)
	checkFloat(t, "Range", got.Range, want.Range)
	checkFloat(t, "Range_dB", got.Range_dB, want.Range_dB)
	checkFloat(t, "CrestFactor", got.CrestFactor, want.CrestFactor)
	checkFloat(t, "CrestFactor_dB", got.CrestFactor_dB, want.CrestFactor_dB)
	checkFloat(t, "Energy", got.Energy, want.Energy)
	checkFloat(t, "Power", got.Power, want.Power)
	if got.ZeroCrossings != want.ZeroCrossings {
		t.Errorf("ZeroCrossings: got %d, want %d", got.ZeroCrossings, want.ZeroCrossings)
	}
	checkFloat(t, "Variance", got.Variance, want.Variance)
	checkFloat(t, "Skewness", got.Skewness, want.Skewness)
	checkFloat(t, "Kurtosis", got.Kurtosis, want.Kurtosis)
}

func checkFloat(t *testing.T, name string, got, want float64) {
	t.Helper()
	if !almostEqual(got, want, tolerance) {
		t.Errorf("%s: got %g, want %g", name, got, want)
	}
}

// itoa converts an int to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
