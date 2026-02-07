package ir

import (
	"math"
	"testing"
)

// makeExponentialDecay generates a synthetic IR with known RT60.
// h(t) = exp(-6.908 * t / rt60) where 6.908 = ln(10^3) ensures -60 dB at rt60.
func makeExponentialDecay(sampleRate float64, rt60 float64, durationSec float64) []float64 {
	n := int(sampleRate * durationSec)
	ir := make([]float64, n)
	decayRate := 6.9078 / rt60 // ln(10^3) / RT60
	for i := range ir {
		t := float64(i) / sampleRate
		ir[i] = math.Exp(-decayRate * t)
	}
	return ir
}

// makeImpulseWithReflection creates a simple IR: impulse at t=0 + reflection.
func makeImpulseWithReflection(sampleRate float64, reflectionDelayMs float64, reflectionAmp float64, length int) []float64 {
	ir := make([]float64, length)
	ir[0] = 1.0
	reflSample := int(reflectionDelayMs * 0.001 * sampleRate)
	if reflSample < length {
		ir[reflSample] = reflectionAmp
	}
	return ir
}

func TestAnalyzerAnalyze(t *testing.T) {
	sampleRate := 48000.0
	rt60 := 1.0 // 1 second RT60
	ir := makeExponentialDecay(sampleRate, rt60, 3.0)

	analyzer := NewAnalyzer(sampleRate)
	metrics, err := analyzer.Analyze(ir)
	if err != nil {
		t.Fatal(err)
	}

	// RT60 should be close to 1.0 seconds
	if math.Abs(metrics.RT60-rt60) > 0.05*rt60 {
		t.Errorf("RT60 = %.3f, want %.3f (±5%%)", metrics.RT60, rt60)
	}

	// Peak should be at index 0 for this simple decay
	if metrics.PeakIndex != 0 {
		t.Errorf("PeakIndex = %d, want 0", metrics.PeakIndex)
	}

	// Center time should be positive and reasonable
	if metrics.CenterTime <= 0 || metrics.CenterTime > rt60 {
		t.Errorf("CenterTime = %.3f, expected between 0 and %.3f", metrics.CenterTime, rt60)
	}

	// D50 should be between 0 and 1
	if metrics.D50 < 0 || metrics.D50 > 1 {
		t.Errorf("D50 = %.3f, expected in [0, 1]", metrics.D50)
	}

	// D80 should be >= D50
	if metrics.D80 < metrics.D50 {
		t.Errorf("D80 = %.3f < D50 = %.3f", metrics.D80, metrics.D50)
	}
}

func TestSchroederIntegral(t *testing.T) {
	sampleRate := 48000.0
	ir := makeExponentialDecay(sampleRate, 1.0, 3.0)

	analyzer := NewAnalyzer(sampleRate)
	schroeder, err := analyzer.SchroederIntegral(ir)
	if err != nil {
		t.Fatal(err)
	}

	if len(schroeder) != len(ir) {
		t.Fatalf("Schroeder length = %d, want %d", len(schroeder), len(ir))
	}

	// First sample should be 0 dB (all energy ahead)
	if math.Abs(schroeder[0]) > 0.01 {
		t.Errorf("Schroeder[0] = %.3f dB, want ~0 dB", schroeder[0])
	}

	// Should be monotonically decreasing
	for i := 1; i < len(schroeder); i++ {
		if schroeder[i] > schroeder[i-1]+0.001 { // small tolerance for numerical noise
			t.Errorf("Schroeder not monotonically decreasing at sample %d: %.3f > %.3f",
				i, schroeder[i], schroeder[i-1])
			break
		}
	}

	// For exponential decay, the Schroeder integral should also be linear in dB
	// Check linearity at 25% and 50% of the way through
	idx25 := len(schroeder) / 4
	idx50 := len(schroeder) / 2
	// Schroeder should still have significant dynamic range at 25%
	if schroeder[idx25] > -5 {
		t.Errorf("Schroeder[25%%] = %.1f dB, expected < -5 dB", schroeder[idx25])
	}
	if schroeder[idx50] >= schroeder[idx25] {
		t.Errorf("Schroeder[50%%] = %.1f >= Schroeder[25%%] = %.1f",
			schroeder[idx50], schroeder[idx25])
	}
}

func TestSchroederIntegralEmpty(t *testing.T) {
	analyzer := NewAnalyzer(48000)
	_, err := analyzer.SchroederIntegral(nil)
	if err != ErrEmptyIR {
		t.Errorf("SchroederIntegral(nil) = %v, want ErrEmptyIR", err)
	}
}

func TestRT60ExponentialDecay(t *testing.T) {
	sampleRate := 48000.0
	tests := []struct {
		name   string
		rt60   float64
		durSec float64
	}{
		{"short", 0.3, 1.5},
		{"medium", 1.0, 3.0},
		{"long", 2.5, 8.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ir := makeExponentialDecay(sampleRate, tt.rt60, tt.durSec)
			analyzer := NewAnalyzer(sampleRate)

			rt, err := analyzer.RT60(ir)
			if err != nil {
				t.Fatal(err)
			}

			tolerance := 0.05 * tt.rt60 // 5% tolerance
			if math.Abs(rt-tt.rt60) > tolerance {
				t.Errorf("RT60 = %.4f, want %.4f (±5%%)", rt, tt.rt60)
			}
		})
	}
}

func TestRT60NoDecay(t *testing.T) {
	// A single-sample IR has no meaningful decay
	ir := []float64{1.0}

	analyzer := NewAnalyzer(48000)
	_, err := analyzer.RT60(ir)
	if err != ErrNoDecay {
		t.Errorf("RT60(single sample) = %v, want ErrNoDecay", err)
	}
}

func TestRT60TooShortForRegression(t *testing.T) {
	// Very short IR where Schroeder can't reach -35 dB or -25 dB
	ir := []float64{1.0, 0.5}
	analyzer := NewAnalyzer(48000)
	_, err := analyzer.RT60(ir)
	if err != ErrNoDecay {
		t.Errorf("RT60(2 samples) = %v, want ErrNoDecay", err)
	}
}

func TestDefinition(t *testing.T) {
	sampleRate := 48000.0

	t.Run("all_early_energy", func(t *testing.T) {
		// Very short IR: all energy within 50ms
		ir := make([]float64, int(sampleRate*0.01)) // 10ms
		ir[0] = 1.0
		analyzer := NewAnalyzer(sampleRate)

		d50, err := analyzer.Definition(ir, 50)
		if err != nil {
			t.Fatal(err)
		}
		if d50 != 1.0 {
			t.Errorf("D50 = %.3f, want 1.0 for all-early IR", d50)
		}
	})

	t.Run("split_energy", func(t *testing.T) {
		// Equal impulses at t=0 and t=100ms
		ir := make([]float64, int(sampleRate*0.2))
		ir[0] = 1.0
		reflSample := int(100 * 0.001 * sampleRate) // 100ms
		ir[reflSample] = 1.0

		analyzer := NewAnalyzer(sampleRate)

		d50, err := analyzer.Definition(ir, 50)
		if err != nil {
			t.Fatal(err)
		}
		// Only the first impulse is within 50ms, so D50 ≈ 0.5
		if math.Abs(d50-0.5) > 0.01 {
			t.Errorf("D50 = %.3f, want ~0.5", d50)
		}

		d80, err := analyzer.Definition(ir, 80)
		if err != nil {
			t.Fatal(err)
		}
		// Only the first impulse is within 80ms, so D80 ≈ 0.5
		if math.Abs(d80-0.5) > 0.01 {
			t.Errorf("D80 = %.3f, want ~0.5", d80)
		}
	})

	t.Run("validation", func(t *testing.T) {
		analyzer := NewAnalyzer(48000)
		_, err := analyzer.Definition(nil, 50)
		if err != ErrEmptyIR {
			t.Errorf("Definition(nil) = %v, want ErrEmptyIR", err)
		}
		_, err = analyzer.Definition([]float64{1}, 0)
		if err != ErrInvalidTime {
			t.Errorf("Definition(t=0) = %v, want ErrInvalidTime", err)
		}
		_, err = analyzer.Definition([]float64{1}, -10)
		if err != ErrInvalidTime {
			t.Errorf("Definition(t=-10) = %v, want ErrInvalidTime", err)
		}
	})
}

func TestClarity(t *testing.T) {
	sampleRate := 48000.0

	t.Run("equal_split", func(t *testing.T) {
		// Equal impulses at t=0 and t=100ms → C80 = 0 dB (early == late)
		ir := make([]float64, int(sampleRate*0.2))
		ir[0] = 1.0
		reflSample := int(100 * 0.001 * sampleRate)
		ir[reflSample] = 1.0

		analyzer := NewAnalyzer(sampleRate)

		c80, err := analyzer.Clarity(ir, 80)
		if err != nil {
			t.Fatal(err)
		}
		// With boundary at 80ms, first impulse is early, second is late
		// Equal energy → C80 = 0 dB
		if math.Abs(c80) > 0.1 {
			t.Errorf("C80 = %.3f dB, want ~0 dB for equal early/late", c80)
		}
	})

	t.Run("mostly_early", func(t *testing.T) {
		// Strong early, weak late
		ir := make([]float64, int(sampleRate*0.2))
		ir[0] = 1.0
		reflSample := int(100 * 0.001 * sampleRate)
		ir[reflSample] = 0.1

		analyzer := NewAnalyzer(sampleRate)

		c80, err := analyzer.Clarity(ir, 80)
		if err != nil {
			t.Fatal(err)
		}
		// Early energy = 1.0, late = 0.01 → C80 = 10*log10(1/0.01) = 20 dB
		expected := 10 * math.Log10(1.0/0.01)
		if math.Abs(c80-expected) > 0.1 {
			t.Errorf("C80 = %.1f dB, want ~%.1f dB", c80, expected)
		}
	})

	t.Run("validation", func(t *testing.T) {
		analyzer := NewAnalyzer(48000)
		_, err := analyzer.Clarity(nil, 80)
		if err != ErrEmptyIR {
			t.Errorf("Clarity(nil) = %v, want ErrEmptyIR", err)
		}
		_, err = analyzer.Clarity([]float64{1}, 0)
		if err != ErrInvalidTime {
			t.Errorf("Clarity(t=0) = %v, want ErrInvalidTime", err)
		}
	})
}

func TestCenterTime(t *testing.T) {
	sampleRate := 48000.0

	t.Run("single_impulse", func(t *testing.T) {
		// Single impulse at t=0 → center time = 0
		ir := make([]float64, 1000)
		ir[0] = 1.0

		analyzer := NewAnalyzer(sampleRate)
		ct, err := analyzer.CenterTime(ir)
		if err != nil {
			t.Fatal(err)
		}
		if ct != 0 {
			t.Errorf("CenterTime = %g, want 0 for impulse at t=0", ct)
		}
	})

	t.Run("two_equal_impulses", func(t *testing.T) {
		// Equal impulses at t=0 and t=100ms → center = 50ms
		ir := make([]float64, int(sampleRate*0.2))
		ir[0] = 1.0
		reflSample := int(100 * 0.001 * sampleRate)
		ir[reflSample] = 1.0

		analyzer := NewAnalyzer(sampleRate)
		ct, err := analyzer.CenterTime(ir)
		if err != nil {
			t.Fatal(err)
		}

		expected := 0.05 // 50ms
		if math.Abs(ct-expected) > 0.001 {
			t.Errorf("CenterTime = %.4f, want ~%.4f", ct, expected)
		}
	})

	t.Run("validation", func(t *testing.T) {
		analyzer := NewAnalyzer(48000)
		_, err := analyzer.CenterTime(nil)
		if err != ErrEmptyIR {
			t.Errorf("CenterTime(nil) = %v, want ErrEmptyIR", err)
		}
	})
}

func TestFindImpulseStart(t *testing.T) {
	sampleRate := 48000.0

	t.Run("immediate_start", func(t *testing.T) {
		ir := make([]float64, 1000)
		ir[0] = 1.0

		analyzer := NewAnalyzer(sampleRate)
		idx, err := analyzer.FindImpulseStart(ir)
		if err != nil {
			t.Fatal(err)
		}
		if idx != 0 {
			t.Errorf("FindImpulseStart = %d, want 0", idx)
		}
	})

	t.Run("delayed_start", func(t *testing.T) {
		ir := make([]float64, 10000)
		// Silence then impulse at sample 5000
		ir[5000] = 1.0
		ir[5001] = 0.5

		analyzer := NewAnalyzer(sampleRate)
		idx, err := analyzer.FindImpulseStart(ir)
		if err != nil {
			t.Fatal(err)
		}
		// Threshold is 10% of peak, so should find sample 5000
		if idx != 5000 {
			t.Errorf("FindImpulseStart = %d, want 5000", idx)
		}
	})

	t.Run("noise_floor", func(t *testing.T) {
		ir := make([]float64, 10000)
		// Low-level noise before the impulse
		for i := 0; i < 5000; i++ {
			ir[i] = 0.001 * float64(i%2*2-1)
		}
		ir[5000] = 1.0

		analyzer := NewAnalyzer(sampleRate)
		idx, err := analyzer.FindImpulseStart(ir)
		if err != nil {
			t.Fatal(err)
		}
		// Noise is 0.001, peak is 1.0, threshold is 0.1 → should find sample 5000
		if idx != 5000 {
			t.Errorf("FindImpulseStart = %d, want 5000", idx)
		}
	})

	t.Run("empty", func(t *testing.T) {
		analyzer := NewAnalyzer(48000)
		_, err := analyzer.FindImpulseStart(nil)
		if err != ErrEmptyIR {
			t.Errorf("FindImpulseStart(nil) = %v, want ErrEmptyIR", err)
		}
	})
}

func TestAnalyzeValidation(t *testing.T) {
	analyzer := NewAnalyzer(48000)

	_, err := analyzer.Analyze(nil)
	if err != ErrEmptyIR {
		t.Errorf("Analyze(nil) = %v, want ErrEmptyIR", err)
	}

	analyzer2 := NewAnalyzer(0)
	_, err = analyzer2.Analyze([]float64{1})
	if err != ErrInvalidSampleRate {
		t.Errorf("Analyze(sr=0) = %v, want ErrInvalidSampleRate", err)
	}

	analyzer3 := NewAnalyzer(-1)
	_, err = analyzer3.Analyze([]float64{1})
	if err != ErrInvalidSampleRate {
		t.Errorf("Analyze(sr=-1) = %v, want ErrInvalidSampleRate", err)
	}
}

func TestEDT(t *testing.T) {
	sampleRate := 48000.0
	rt60 := 2.0
	ir := makeExponentialDecay(sampleRate, rt60, 6.0)

	analyzer := NewAnalyzer(sampleRate)
	metrics, err := analyzer.Analyze(ir)
	if err != nil {
		t.Fatal(err)
	}

	// For a perfect exponential decay, EDT should equal RT60
	tolerance := 0.10 * rt60 // 10% tolerance for EDT (more sensitive to early part)
	if math.Abs(metrics.EDT-rt60) > tolerance {
		t.Errorf("EDT = %.3f, want ~%.3f (±10%%)", metrics.EDT, rt60)
	}
}

func TestT20T30Consistency(t *testing.T) {
	sampleRate := 48000.0
	rt60 := 1.5
	ir := makeExponentialDecay(sampleRate, rt60, 5.0)

	analyzer := NewAnalyzer(sampleRate)
	metrics, err := analyzer.Analyze(ir)
	if err != nil {
		t.Fatal(err)
	}

	// For perfect exponential decay, T20 ≈ T30 ≈ RT60
	tolerance := 0.05 * rt60
	if math.Abs(metrics.T20-rt60) > tolerance {
		t.Errorf("T20 = %.4f, want %.4f (±5%%)", metrics.T20, rt60)
	}
	if math.Abs(metrics.T30-rt60) > tolerance {
		t.Errorf("T30 = %.4f, want %.4f (±5%%)", metrics.T30, rt60)
	}
}

func TestDefinitionAndClarityRelationship(t *testing.T) {
	// D(t) and C(t) are related: C(t) = 10*log10(D(t)/(1-D(t)))
	sampleRate := 48000.0
	ir := makeExponentialDecay(sampleRate, 1.0, 3.0)

	analyzer := NewAnalyzer(sampleRate)
	metrics, err := analyzer.Analyze(ir)
	if err != nil {
		t.Fatal(err)
	}

	// Check D50/C50 relationship
	if metrics.D50 > 0 && metrics.D50 < 1 {
		expectedC50 := 10 * math.Log10(metrics.D50/(1-metrics.D50))
		if math.Abs(metrics.C50-expectedC50) > 0.01 {
			t.Errorf("C50 = %.3f, expected %.3f from D50 = %.3f",
				metrics.C50, expectedC50, metrics.D50)
		}
	}

	// Check D80/C80 relationship
	if metrics.D80 > 0 && metrics.D80 < 1 {
		expectedC80 := 10 * math.Log10(metrics.D80/(1-metrics.D80))
		if math.Abs(metrics.C80-expectedC80) > 0.01 {
			t.Errorf("C80 = %.3f, expected %.3f from D80 = %.3f",
				metrics.C80, expectedC80, metrics.D80)
		}
	}
}

func TestNewAnalyzer(t *testing.T) {
	a := NewAnalyzer(44100)
	if a.SampleRate != 44100 {
		t.Errorf("SampleRate = %f, want 44100", a.SampleRate)
	}
}
