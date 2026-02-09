package pass

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

type bandSignature struct {
	spanDB       float64
	extrema      int
	minDB        float64
	maxDB        float64
	maxAbsDB     float64
	peakFreqHz   float64
	troughFreqHz float64
}

func measureBandSignature(sections []biquad.Coefficients, fStart, fEnd, step, sr float64) bandSignature {
	sig := bandSignature{
		minDB: math.MaxFloat64,
		maxDB: -math.MaxFloat64,
	}
	var vals []float64
	var freqs []float64
	for f := fStart; f <= fEnd; f += step {
		d := cascadeMagDB(sections, f, sr)
		vals = append(vals, d)
		freqs = append(freqs, f)
		if d < sig.minDB {
			sig.minDB = d
			sig.troughFreqHz = f
		}
		if d > sig.maxDB {
			sig.maxDB = d
			sig.peakFreqHz = f
		}
	}
	sig.spanDB = sig.maxDB - sig.minDB
	sig.maxAbsDB = math.Max(math.Abs(sig.maxDB), math.Abs(sig.minDB))
	for i := 1; i < len(vals)-1; i++ {
		if (vals[i] > vals[i-1] && vals[i] > vals[i+1]) || (vals[i] < vals[i-1] && vals[i] < vals[i+1]) {
			sig.extrema++
		}
	}
	return sig
}

func TestButterworthLP_FamilySignature(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	sections := ButterworthLP(fc, 4, sr)

	pass := measureBandSignature(sections, 10, 0.8*fc, 10, sr)
	stop := measureBandSignature(sections, 2*fc, 0.45*sr, 100, sr)

	if pass.spanDB > 1.0 {
		t.Fatalf("butterworth LP passband should be flat: span=%.3f dB", pass.spanDB)
	}
	if pass.extrema > 1 {
		t.Fatalf("butterworth LP passband should be monotonic/no ripple: extrema=%d", pass.extrema)
	}
	if stop.extrema != 0 {
		t.Fatalf("butterworth LP stopband should be monotonic: extrema=%d", stop.extrema)
	}
}

func TestButterworthHP_FamilySignature(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	sections := ButterworthHP(fc, 4, sr)

	pass := measureBandSignature(sections, 1.2*fc, 0.4*sr, 100, sr)
	stop := measureBandSignature(sections, 10, 0.5*fc, 10, sr)

	if pass.spanDB > 1.2 {
		t.Fatalf("butterworth HP passband should be flat: span=%.3f dB", pass.spanDB)
	}
	if pass.extrema != 0 {
		t.Fatalf("butterworth HP passband should be monotonic/no ripple: extrema=%d", pass.extrema)
	}
	if stop.extrema != 0 {
		t.Fatalf("butterworth HP stopband should be monotonic: extrema=%d", stop.extrema)
	}
}

func TestChebyshev1LP_FamilySignature(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	sections := Chebyshev1LP(fc, 4, 1.0, sr)

	pass := measureBandSignature(sections, 10, 0.8*fc, 10, sr)
	stop := measureBandSignature(sections, 2*fc, 0.45*sr, 100, sr)

	if pass.spanDB < 1.0 {
		t.Fatalf("chebyshev1 LP passband should ripple: span=%.3f dB", pass.spanDB)
	}
	if pass.extrema < 1 {
		t.Fatalf("chebyshev1 LP passband should have interior extrema: extrema=%d", pass.extrema)
	}
	if stop.extrema != 0 {
		t.Fatalf("chebyshev1 LP stopband should be monotonic (no equiripple): extrema=%d", stop.extrema)
	}
}

func TestChebyshev1HP_FamilySignature(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	sections := Chebyshev1HP(fc, 4, 1.0, sr)

	pass := measureBandSignature(sections, 1.2*fc, 0.4*sr, 100, sr)
	stop := measureBandSignature(sections, 10, 0.5*fc, 10, sr)

	if pass.spanDB < 1.0 {
		t.Fatalf("chebyshev1 HP passband should ripple: span=%.3f dB", pass.spanDB)
	}
	if pass.extrema < 1 {
		t.Fatalf("chebyshev1 HP passband should have interior extrema: extrema=%d", pass.extrema)
	}
	if stop.extrema != 0 {
		t.Fatalf("chebyshev1 HP stopband should be monotonic (no equiripple): extrema=%d", stop.extrema)
	}
}

func TestChebyshev2LP_FamilySignature(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	sections := Chebyshev2LP(fc, 4, 2.0, sr)

	pass := measureBandSignature(sections, 10, 0.8*fc, 10, sr)
	stop := measureBandSignature(sections, 2*fc, 0.45*sr, 100, sr)

	if pass.spanDB > 0.8 {
		t.Fatalf("chebyshev2 LP passband should be comparatively flat: span=%.3f dB", pass.spanDB)
	}
	if stop.extrema < 1 {
		t.Fatalf("chebyshev2 LP stopband should ripple/equiripple: extrema=%d", stop.extrema)
	}
	if stop.maxDB > -3.0 {
		t.Fatalf("chebyshev2 LP stopband should stay attenuated: max=%.3f dB", stop.maxDB)
	}
}

func TestChebyshev2HP_FamilySignature(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	sections := Chebyshev2HP(fc, 4, 2.0, sr)

	pass := measureBandSignature(sections, 1.2*fc, 0.4*sr, 100, sr)
	stop := measureBandSignature(sections, 10, 0.5*fc, 10, sr)

	if pass.spanDB > 0.8 {
		t.Fatalf("chebyshev2 HP passband should be comparatively flat: span=%.3f dB", pass.spanDB)
	}
	if stop.extrema < 1 {
		t.Fatalf("chebyshev2 HP stopband should ripple/equiripple: extrema=%d", stop.extrema)
	}
	if stop.maxDB > -3.0 {
		t.Fatalf("chebyshev2 HP stopband should stay attenuated: max=%.3f dB", stop.maxDB)
	}
}

func TestEllipticLP_FamilySignature(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	sections := EllipticLP(fc, 4, 0.5, 40.0, sr)

	pass := measureBandSignature(sections, 10, 0.9*fc, 10, sr)
	stop := measureBandSignature(sections, 2*fc, 0.45*sr, 100, sr)

	// Elliptic LP should exhibit passband ripple and stopband ripple.
	if pass.spanDB < 0.4 {
		t.Fatalf("elliptic LP passband should ripple: span=%.3f dB", pass.spanDB)
	}
	if pass.extrema < 1 {
		t.Fatalf("elliptic LP passband should have interior extrema: extrema=%d", pass.extrema)
	}
	if stop.extrema < 1 {
		t.Fatalf("elliptic LP stopband should ripple/equiripple: extrema=%d", stop.extrema)
	}
	if stop.maxDB > -20.0 {
		t.Fatalf("elliptic LP stopband should be attenuated: max=%.3f dB", stop.maxDB)
	}
}

func TestEllipticHP_FamilySignature(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	sections := EllipticHP(fc, 4, 0.5, 40.0, sr)

	pass := measureBandSignature(sections, 1.2*fc, 0.4*sr, 100, sr)
	stop := measureBandSignature(sections, 10, 0.5*fc, 10, sr)

	// Elliptic HP should exhibit passband ripple and stopband ripple.
	if pass.spanDB < 0.4 {
		t.Fatalf("elliptic HP passband should ripple: span=%.3f dB", pass.spanDB)
	}
	if pass.extrema < 1 {
		t.Fatalf("elliptic HP passband should have interior extrema: extrema=%d", pass.extrema)
	}
	if stop.extrema < 1 {
		t.Fatalf("elliptic HP stopband should ripple/equiripple: extrema=%d", stop.extrema)
	}
	if stop.maxDB > -20.0 {
		t.Fatalf("elliptic HP stopband should be attenuated: max=%.3f dB", stop.maxDB)
	}
}
