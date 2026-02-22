package design

import (
	"math"
	"testing"
)

func TestATHShape(t *testing.T) {
	// ATH should have a minimum around 3-4 kHz (most sensitive hearing range)
	// and rise at both low and high frequencies.
	ath1k := ATH(1000)
	ath3500 := ATH(3500)
	ath10k := ATH(10000)

	if ath3500 >= ath1k {
		t.Errorf("ATH(3.5kHz)=%g should be < ATH(1kHz)=%g", ath3500, ath1k)
	}

	if ath10k <= ath3500 {
		t.Errorf("ATH(10kHz)=%g should be > ATH(3.5kHz)=%g", ath10k, ath3500)
	}
}

func TestATHLowFrequencyClamp(t *testing.T) {
	// Very low frequencies should not produce NaN or Inf due to the clamp.
	for _, freq := range []float64{0, 1, 5, 10} {
		val := ATH(freq)
		if math.IsNaN(val) || math.IsInf(val, 0) {
			t.Errorf("ATH(%g) = %v", freq, val)
		}
	}
}

func TestATHHighFrequency(t *testing.T) {
	// ATH should rise sharply above ~15 kHz.
	ath15k := ATH(15000)
	ath20k := ATH(20000)

	if ath20k <= ath15k {
		t.Errorf("ATH(20kHz)=%g should be > ATH(15kHz)=%g", ath20k, ath15k)
	}
}

func TestATHKnownValue(t *testing.T) {
	// Verify against a hand-calculated reference at 1 kHz.
	// f = 1.0 kHz:
	//   3.640 * 1^(-0.8) = 3.640
	//   -6.800 * exp(-0.6 * (1-3.4)^2) = -6.800 * exp(-0.6*5.76) = -6.800 * exp(-3.456)
	//   +6.000 * exp(-0.15 * (1-8.7)^2) = +6.000 * exp(-0.15*59.29) = +6.000 * exp(-8.8935)
	//   +0.0006 * 1^4 = 0.0006
	exp3456 := math.Exp(-3.456)
	exp8894 := math.Exp(-8.8935)
	expected := 3.640 - 6.800*exp3456 + 6.000*exp8894 + 0.0006

	got := ATH(1000)

	if math.Abs(got-expected) > 1e-10 {
		t.Errorf("ATH(1000) = %g, want %g", got, expected)
	}
}

func TestCriticalBandwidthShape(t *testing.T) {
	// Critical bandwidth should increase monotonically with frequency.
	cb1k := CriticalBandwidth(1000)
	cb4k := CriticalBandwidth(4000)
	cb10k := CriticalBandwidth(10000)

	if cb1k <= 0 {
		t.Errorf("CriticalBandwidth(1kHz) = %g", cb1k)
	}

	if cb4k <= cb1k {
		t.Errorf("CB(4kHz)=%g should be > CB(1kHz)=%g", cb4k, cb1k)
	}

	if cb10k <= cb4k {
		t.Errorf("CB(10kHz)=%g should be > CB(4kHz)=%g", cb10k, cb4k)
	}
}

func TestCriticalBandwidthAtZero(t *testing.T) {
	// At 0 Hz: 25 + 75 * (1 + 0)^0.69 = 25 + 75 = 100
	got := CriticalBandwidth(0)
	if math.Abs(got-100) > 1e-10 {
		t.Errorf("CriticalBandwidth(0) = %g, want 100", got)
	}
}

func TestCriticalBandwidthKnownValue(t *testing.T) {
	// At 1 kHz: 25 + 75 * (1 + 1.4 * 1^2)^0.69 = 25 + 75 * 2.4^0.69
	expected := 25 + 75*math.Pow(2.4, 0.69)

	got := CriticalBandwidth(1000)
	if math.Abs(got-expected) > 1e-10 {
		t.Errorf("CriticalBandwidth(1000) = %g, want %g", got, expected)
	}
}
