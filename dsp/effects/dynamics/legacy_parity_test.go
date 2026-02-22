package dynamics

import (
	"math"
	"testing"
)

func TestLegacyParityFeedforwardPeakHardKnee(t *testing.T) {
	const sampleRate = 48000.0

	in := makeLegacyParitySignal()

	c, _ := NewCompressor(sampleRate)
	_ = c.SetAutoMakeup(false)
	_ = c.SetMakeupGain(0)
	_ = c.SetKnee(0)
	_ = c.SetThreshold(-18)
	_ = c.SetRatio(4)
	_ = c.SetAttack(10)
	_ = c.SetRelease(100)
	_ = c.SetTopology(DynamicsTopologyFeedforward)
	_ = c.SetDetectorMode(DetectorModePeak)

	got := make([]float64, len(in))
	for i := range in {
		got[i] = c.ProcessSample(in[i])
	}

	want := simulateLegacyFeedforwardPeak(in, sampleRate, -18, 4, 10, 100)
	assertVectorClose(t, got, want, 1e-9)
}

func TestLegacyParityFeedbackPeakHardKnee(t *testing.T) {
	const sampleRate = 48000.0

	in := makeLegacyParitySignal()

	c, _ := NewCompressor(sampleRate)
	_ = c.SetAutoMakeup(false)
	_ = c.SetMakeupGain(0)
	_ = c.SetKnee(0)
	_ = c.SetThreshold(-18)
	_ = c.SetRatio(4)
	_ = c.SetAttack(10)
	_ = c.SetRelease(100)
	_ = c.SetTopology(DynamicsTopologyFeedback)
	_ = c.SetDetectorMode(DetectorModePeak)
	_ = c.SetFeedbackRatioScale(true)

	got := make([]float64, len(in))
	for i := range in {
		got[i] = c.ProcessSample(in[i])
	}

	want := simulateLegacyFeedbackPeak(in, sampleRate, -18, 4, 10, 100)
	assertVectorClose(t, got, want, 1e-9)
}

func TestLegacyParityFeedforwardRMSHardKnee(t *testing.T) {
	const sampleRate = 48000.0

	in := makeLegacyParitySignal()

	c, _ := NewCompressor(sampleRate)
	_ = c.SetAutoMakeup(false)
	_ = c.SetMakeupGain(0)
	_ = c.SetKnee(0)
	_ = c.SetThreshold(-20)
	_ = c.SetRatio(3)
	_ = c.SetAttack(8)
	_ = c.SetRelease(90)
	_ = c.SetTopology(DynamicsTopologyFeedforward)
	_ = c.SetDetectorMode(DetectorModeRMS)
	_ = c.SetRMSWindow(10)

	got := make([]float64, len(in))
	for i := range in {
		got[i] = c.ProcessSample(in[i])
	}

	want := simulateLegacyFeedforwardRMS(in, sampleRate, -20, 3, 8, 90, 10)
	assertVectorClose(t, got, want, 1e-9)
}

func TestLegacyCharacterizationTemporalBehavior(t *testing.T) {
	c, _ := NewCompressor(48000)
	_ = c.SetAutoMakeup(false)
	_ = c.SetMakeupGain(0)
	_ = c.SetKnee(0)
	_ = c.SetThreshold(-24)
	_ = c.SetRatio(6)
	_ = c.SetAttack(5)
	_ = c.SetRelease(80)
	_ = c.SetTopology(DynamicsTopologyFeedback)
	_ = c.SetFeedbackRatioScale(true)

	// Step up
	for i := 0; i < 1500; i++ {
		_ = c.ProcessSample(0.8)
	}

	grAfterStep := c.GetMetrics().GainReduction
	if grAfterStep >= 1.0 {
		t.Fatalf("expected gain reduction after step, got %f", grAfterStep)
	}

	// Burst then decay
	for i := 0; i < 200; i++ {
		_ = c.ProcessSample(0.9)
	}

	for i := 0; i < 4000; i++ {
		_ = c.ProcessSample(0.0)
	}

	if c.peakLevel > 0.1 {
		t.Fatalf("expected feedback recovery after burst, peak=%f", c.peakLevel)
	}
}

func TestLegacyParityNoAllocsInPlace(t *testing.T) {
	c, _ := NewCompressor(48000)

	buf := make([]float64, 256)
	for i := range buf {
		buf[i] = 0.3
	}

	allocs := testing.AllocsPerRun(200, func() {
		c.ProcessInPlace(buf)
	})
	if allocs != 0 {
		t.Fatalf("expected zero allocations for ProcessInPlace, got %f", allocs)
	}
}

func makeLegacyParitySignal() []float64 {
	out := make([]float64, 4096)
	for i := range out {
		x := float64(i)
		switch {
		case i < 1024:
			out[i] = 0.08 * math.Sin(2*math.Pi*440*x/48000)
		case i < 2048:
			out[i] = 0.7 * math.Sin(2*math.Pi*440*x/48000)
		case i < 3072:
			out[i] = 0.2 * math.Sin(2*math.Pi*440*x/48000)
		default:
			out[i] = 0.9 * math.Sin(2*math.Pi*880*x/48000)
		}
	}

	return out
}

func simulateLegacyFeedforwardPeak(in []float64, sr, thresholdDB, ratio, attackMs, releaseMs float64) []float64 {
	out := make([]float64, len(in))
	threshold := mathPower10(thresholdDB / 20.0)
	legacyRatio := 1.0 / ratio
	attack := 1.0 - math.Exp(-math.Ln2/(attackMs*0.001*sr))
	release := math.Exp(-math.Ln2 / (releaseMs * 0.001 * sr))
	peak := 0.0
	makeup0 := 1.0
	makeup1 := makeup0 * math.Pow(threshold, 1.0-legacyRatio)

	for i, s := range in {
		a := math.Abs(s)
		if a > peak {
			peak += (a - peak) * attack
		} else {
			peak = a + (peak-a)*release
		}

		gain := makeup0
		if peak >= threshold {
			gain = makeup1 * math.Pow(peak, legacyRatio-1.0)
		}

		out[i] = s * gain
	}

	return out
}

func simulateLegacyFeedbackPeak(in []float64, sr, thresholdDB, ratio, attackMs, releaseMs float64) []float64 {
	out := make([]float64, len(in))
	threshold := mathPower10(thresholdDB / 20.0)
	legacyRatio := 1.0 / ratio
	attack := 1.0 - math.Exp(-math.Ln2/(attackMs*0.001*sr*ratio))
	release := math.Exp(-math.Ln2 / (releaseMs * 0.001 * sr * ratio))
	makeup0 := 1.0
	makeup1 := math.Pow(threshold, (1.0-legacyRatio)*ratio)

	peak := 0.0

	prevAbs := 0.0
	for i, s := range in {
		if prevAbs > peak {
			peak += (prevAbs - peak) * attack
		} else {
			peak = prevAbs + (peak-prevAbs)*release
		}

		gain := 1.0
		if peak >= threshold {
			gain = makeup1 * math.Pow(peak, 1.0-ratio)
		}

		y := s * gain
		out[i] = makeup0 * y
		prevAbs = math.Abs(y)
	}

	return out
}

func simulateLegacyFeedforwardRMS(in []float64, sr, thresholdDB, ratio, attackMs, releaseMs, rmsMs float64) []float64 {
	out := make([]float64, len(in))
	threshold := mathPower10(thresholdDB / 20.0)
	legacyRatio := 1.0 / ratio
	attack := 1.0 - math.Exp(-math.Ln2/(attackMs*0.001*sr))
	release := math.Exp(-math.Ln2 / (releaseMs * 0.001 * sr))
	makeup0 := 1.0
	makeup1 := makeup0 * math.Pow(threshold, 1.0-legacyRatio)

	size := int(math.Round(sr * 0.001 * rmsMs))
	if size < 1 {
		size = 1
	}

	buf := make([]float64, size)
	pos := 0
	sum := 0.0
	peak := 0.0

	for i, s := range in {
		sq := s * s
		sum += sq - buf[pos]
		buf[pos] = sq

		pos++
		if pos >= size {
			pos = 0
		}

		rms := math.Sqrt(sum / float64(size))
		if rms > peak {
			peak += (rms - peak) * attack
		} else {
			peak = rms + (peak-rms)*release
		}

		gain := makeup0
		if peak >= threshold {
			gain = makeup1 * math.Pow(peak, legacyRatio-1.0)
		}

		out[i] = s * gain
	}

	return out
}

func assertVectorClose(t *testing.T, got, want []float64, tol float64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("length mismatch: got=%d want=%d", len(got), len(want))
	}

	for i := range got {
		if math.Abs(got[i]-want[i]) > tol {
			t.Fatalf("vector mismatch at %d: got=%0.12f want=%0.12f diff=%g", i, got[i], want[i], math.Abs(got[i]-want[i]))
		}
	}
}
