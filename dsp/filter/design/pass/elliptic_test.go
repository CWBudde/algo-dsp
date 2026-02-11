package pass

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

func bandMaxMin(sections []biquad.Coefficients, fStart, fEnd, step, sr float64) (float64, float64) {
	maxDB := -math.MaxFloat64
	minDB := math.MaxFloat64
	for f := fStart; f <= fEnd; f += step {
		d := cascadeMagDB(sections, f, sr)
		if d > maxDB {
			maxDB = d
		}
		if d < minDB {
			minDB = d
		}
	}
	return maxDB, minDB
}

func interiorExtremaCount(sections []biquad.Coefficients, fStart, fEnd, step, sr float64) int {
	var vals []float64
	for f := fStart; f <= fEnd; f += step {
		vals = append(vals, cascadeMagDB(sections, f, sr))
	}
	if len(vals) < 3 {
		return 0
	}
	count := 0
	for i := 1; i < len(vals)-1; i++ {
		if (vals[i] > vals[i-1] && vals[i] > vals[i+1]) || (vals[i] < vals[i-1] && vals[i] < vals[i+1]) {
			count++
		}
	}
	return count
}

// TestEllipticLP_ValidOrders tests that elliptic lowpass produces correct section counts.
func TestEllipticLP_ValidOrders(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	ripple := 0.5
	stopband := 40.0

	tests := []struct {
		order    int
		sections int
	}{
		{1, 1},
		{2, 1},
		{3, 2},
		{4, 2},
		{5, 3},
		{6, 3},
	}

	for _, tt := range tests {
		sections := EllipticLP(fc, tt.order, ripple, stopband, sr)
		if len(sections) != tt.sections {
			t.Errorf("order %d: got %d sections, want %d", tt.order, len(sections), tt.sections)
		}
		for _, s := range sections {
			assertFiniteCoefficients(t, s)
			assertStableSection(t, s)
		}
	}
}

// TestEllipticHP_ValidOrders tests that elliptic highpass produces correct section counts.
func TestEllipticHP_ValidOrders(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	ripple := 0.5
	stopband := 40.0

	tests := []struct {
		order    int
		sections int
	}{
		{1, 1},
		{2, 1},
		{3, 2},
		{4, 2},
		{5, 3},
		{6, 3},
	}

	for _, tt := range tests {
		sections := EllipticHP(fc, tt.order, ripple, stopband, sr)
		if len(sections) != tt.sections {
			t.Errorf("order %d: got %d sections, want %d", tt.order, len(sections), tt.sections)
		}
		for _, s := range sections {
			assertFiniteCoefficients(t, s)
			assertStableSection(t, s)
		}
	}
}

// TestEllipticLP_PassbandRipple tests that lowpass passband ripple is within spec.
func TestEllipticLP_PassbandRipple(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	rippleDB := 0.5
	stopbandDB := 40.0

	sections := EllipticLP(fc, 4, rippleDB, stopbandDB, sr)

	// Test passband up to 80% of cutoff frequency.
	maxRipple := 0.0
	minGain := 100.0
	maxRippleFreq := 0.0
	minGainFreq := 0.0
	for freq := 10.0; freq < fc*0.8; freq += 10 {
		magDB := cascadeMagDB(sections, freq, sr)
		if absDB := math.Abs(magDB); absDB > maxRipple {
			maxRipple = absDB
			maxRippleFreq = freq
		}
		if magDB < minGain {
			minGain = magDB
			minGainFreq = freq
		}
	}

	// Passband ripple should be within the specified dB range.
	if maxRipple > rippleDB+0.1 {
		t.Errorf("passband ripple %.3f dB at %.1f Hz exceeds spec %.3f dB", maxRipple, maxRippleFreq, rippleDB)
	}
	if minGain < -rippleDB-0.1 {
		t.Errorf("passband minimum %.3f dB at %.1f Hz exceeds ripple spec %.3f dB", minGain, minGainFreq, rippleDB)
	}
}

// TestEllipticLP_StopbandAttenuation tests that stopband meets minimum attenuation.
func TestEllipticLP_StopbandAttenuation(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	rippleDB := 0.5
	stopbandDB := 40.0

	sections := EllipticLP(fc, 4, rippleDB, stopbandDB, sr)

	// Test stopband from 2x cutoff to Nyquist.
	maxStopband := -200.0
	maxStopbandFreq := 0.0
	for freq := fc * 2; freq < sr*0.45; freq += 100 {
		magDB := cascadeMagDB(sections, freq, sr)
		if magDB > maxStopband {
			maxStopband = magDB
			maxStopbandFreq = freq
		}
	}

	// Stopband should be at least as attenuated as specified.
	if maxStopband > -stopbandDB+1.0 {
		t.Errorf("stopband peak %.1f dB at %.1f Hz, expected below %.1f dB", maxStopband, maxStopbandFreq, -stopbandDB)
	}
}

// TestEllipticHP_PassbandRipple tests highpass passband ripple.
func TestEllipticHP_PassbandRipple(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	rippleDB := 0.5
	stopbandDB := 40.0

	sections := EllipticHP(fc, 4, rippleDB, stopbandDB, sr)

	// Test passband from 1.2x cutoff to 80% Nyquist.
	maxRipple := 0.0
	minGain := 100.0
	maxRippleFreq := 0.0
	minGainFreq := 0.0
	for freq := fc * 1.2; freq < sr*0.4; freq += 100 {
		magDB := cascadeMagDB(sections, freq, sr)
		if absDB := math.Abs(magDB); absDB > maxRipple {
			maxRipple = absDB
			maxRippleFreq = freq
		}
		if magDB < minGain {
			minGain = magDB
			minGainFreq = freq
		}
	}

	// Passband ripple should be within spec.
	if maxRipple > rippleDB+0.2 {
		t.Errorf("HP passband ripple %.3f dB at %.1f Hz exceeds spec %.3f dB", maxRipple, maxRippleFreq, rippleDB)
	}
	if minGain < -rippleDB-0.2 {
		t.Errorf("HP passband minimum %.3f dB at %.1f Hz exceeds ripple spec %.3f dB", minGain, minGainFreq, rippleDB)
	}
}

// TestEllipticHP_StopbandAttenuation tests highpass stopband.
func TestEllipticHP_StopbandAttenuation(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	rippleDB := 0.5
	stopbandDB := 40.0

	sections := EllipticHP(fc, 4, rippleDB, stopbandDB, sr)

	// Test stopband from 10 Hz to 0.5x cutoff.
	maxStopband := -200.0
	maxStopbandFreq := 0.0
	for freq := 10.0; freq < fc*0.5; freq += 10 {
		magDB := cascadeMagDB(sections, freq, sr)
		if magDB > maxStopband {
			maxStopband = magDB
			maxStopbandFreq = freq
		}
	}

	// Stopband should meet spec.
	if maxStopband > -stopbandDB+1.0 {
		t.Errorf("HP stopband peak %.1f dB at %.1f Hz, expected below %.1f dB", maxStopband, maxStopbandFreq, -stopbandDB)
	}
}

// TestEllipticLP_SharperTransition tests that elliptic has sharper transition than Butterworth.
func TestEllipticLP_SharperTransition(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	order := 4

	elliptic := EllipticLP(fc, order, 0.5, 40, sr)
	butterworth := ButterworthLP(fc, order, sr)

	// At 1.5x cutoff, elliptic should have more attenuation than Butterworth.
	freqTest := fc * 1.5
	ellipticDB := cascadeMagDB(elliptic, freqTest, sr)
	butterworthDB := cascadeMagDB(butterworth, freqTest, sr)

	if ellipticDB >= butterworthDB {
		t.Errorf("elliptic transition not sharper: elliptic %.1f dB, butterworth %.1f dB at %.0f Hz",
			ellipticDB, butterworthDB, freqTest)
	}
}

// TestEllipticLP_HigherStopbandGivesMoreAttenuation tests stopband parameter effect.
func TestEllipticLP_HigherStopbandGivesMoreAttenuation(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	order := 4
	ripple := 0.5

	low := EllipticLP(fc, order, ripple, 40, sr)
	high := EllipticLP(fc, order, ripple, 60, sr)

	// At 3x cutoff, higher stopband spec should give more attenuation.
	freqTest := fc * 3
	lowDB := cascadeMagDB(low, freqTest, sr)
	highDB := cascadeMagDB(high, freqTest, sr)

	if highDB >= lowDB {
		t.Errorf("higher stopband didn't increase attenuation: 40dB=%.1f, 60dB=%.1f at %.0f Hz",
			lowDB, highDB, freqTest)
	}
}

// TestEllipticLP_InvalidParams tests parameter validation.
func TestEllipticLP_InvalidParams(t *testing.T) {
	tests := []struct {
		name string
		freq float64
		sr   float64
	}{
		{"negative freq", -100, 48000},
		{"zero freq", 0, 48000},
		{"freq >= Nyquist", 24000, 48000},
		{"negative sr", 1000, -48000},
		{"zero sr", 1000, 0},
	}

	for _, tt := range tests {
		sections := EllipticLP(tt.freq, 4, 0.5, 40, tt.sr)
		if sections != nil {
			t.Errorf("%s: expected nil, got %d sections", tt.name, len(sections))
		}
	}
}

// TestEllipticHP_InvalidParams tests highpass parameter validation.
func TestEllipticHP_InvalidParams(t *testing.T) {
	tests := []struct {
		name string
		freq float64
		sr   float64
	}{
		{"negative freq", -100, 48000},
		{"zero freq", 0, 48000},
		{"freq >= Nyquist", 24000, 48000},
		{"negative sr", 1000, -48000},
		{"zero sr", 1000, 0},
	}

	for _, tt := range tests {
		sections := EllipticHP(tt.freq, 4, 0.5, 40, tt.sr)
		if sections != nil {
			t.Errorf("%s: expected nil, got %d sections", tt.name, len(sections))
		}
	}
}

// TestEllipticLP_ZeroOrder tests zero-order rejection.
func TestEllipticLP_ZeroOrder(t *testing.T) {
	sections := EllipticLP(1000, 0, 0.5, 40, 48000)
	if sections != nil {
		t.Errorf("order 0: expected nil, got %d sections", len(sections))
	}
}

// TestEllipticHP_ZeroOrder tests highpass zero-order rejection.
func TestEllipticHP_ZeroOrder(t *testing.T) {
	sections := EllipticHP(1000, 0, 0.5, 40, 48000)
	if sections != nil {
		t.Errorf("order 0: expected nil, got %d sections", len(sections))
	}
}

// TestEllipticLP_DCGain tests that DC gain is unity (0 dB).
func TestEllipticLP_DCGain(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	sections := EllipticLP(fc, 4, 0.5, 40, sr)

	dcDB := cascadeMagDB(sections, 1, sr) // Test at 1 Hz (near DC)
	if math.Abs(dcDB) > 0.01 {
		t.Errorf("DC gain %.3f dB, expected 0 dB", dcDB)
	}
}

// TestEllipticHP_NyquistGain tests that Nyquist gain is unity (0 dB).
func TestEllipticHP_NyquistGain(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	sections := EllipticHP(fc, 4, 0.5, 40, sr)

	nyqDB := cascadeMagDB(sections, sr*0.49, sr) // Test near Nyquist
	if math.Abs(nyqDB) > 0.1 {
		t.Errorf("Nyquist gain %.3f dB at %.1f Hz, expected ~0 dB", nyqDB, sr*0.49)
	}
}

// TestEllipticLP_FiniteResponses tests various configurations produce finite responses.
func TestEllipticLP_FiniteResponses(t *testing.T) {
	sampleRates := []float64{44100, 48000, 96000}
	orders := []int{2, 3, 4, 5, 6}
	testFreqs := []float64{10, 100, 1000, 5000, 10000}

	for _, sr := range sampleRates {
		for _, order := range orders {
			sections := EllipticLP(1000, order, 0.5, 40, sr)
			for _, freq := range testFreqs {
				if freq >= sr/2 {
					continue
				}
				magDB := cascadeMagDB(sections, freq, sr)
				if math.IsNaN(magDB) || math.IsInf(magDB, 0) {
					t.Errorf("LP order %d, sr %.0f, freq %.0f: invalid response %.3f",
						order, sr, freq, magDB)
				}
			}
		}
	}
}

// TestEllipticHP_FiniteResponses tests various highpass configurations.
func TestEllipticHP_FiniteResponses(t *testing.T) {
	sampleRates := []float64{44100, 48000, 96000}
	orders := []int{2, 3, 4, 5, 6}
	testFreqs := []float64{10, 100, 1000, 5000, 10000}

	for _, sr := range sampleRates {
		for _, order := range orders {
			sections := EllipticHP(1000, order, 0.5, 40, sr)
			for _, freq := range testFreqs {
				if freq >= sr/2 {
					continue
				}
				magDB := cascadeMagDB(sections, freq, sr)
				if math.IsNaN(magDB) || math.IsInf(magDB, 0) {
					t.Errorf("HP order %d, sr %.0f, freq %.0f: invalid response %.3f",
						order, sr, freq, magDB)
				}
			}
		}
	}
}

// TestEllipticLP_RippleParameterAffectsPassband enforces a core elliptic property:
// increasing rippleDB must increase passband ripple extent for fixed order/stopband.
func TestEllipticLP_RippleParameterAffectsPassband(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	order := 4
	stopbandDB := 40.0

	lowRipple := EllipticLP(fc, order, 0.1, stopbandDB, sr)
	highRipple := EllipticLP(fc, order, 1.0, stopbandDB, sr)

	lowMax, lowMin := bandMaxMin(lowRipple, 10, 0.9*fc, 10, sr)
	highMax, highMin := bandMaxMin(highRipple, 10, 0.9*fc, 10, sr)

	lowSpan := lowMax - lowMin
	highSpan := highMax - highMin

	if highSpan <= lowSpan+0.2 {
		t.Fatalf("rippleDB does not affect LP passband as expected: span@0.1dB=%.3f, span@1.0dB=%.3f", lowSpan, highSpan)
	}
}

// TestEllipticHP_RippleParameterAffectsPassband enforces the same for highpass.
func TestEllipticHP_RippleParameterAffectsPassband(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	order := 4
	stopbandDB := 40.0

	lowRipple := EllipticHP(fc, order, 0.1, stopbandDB, sr)
	highRipple := EllipticHP(fc, order, 1.0, stopbandDB, sr)

	lowMax, lowMin := bandMaxMin(lowRipple, 1.2*fc, 0.4*sr, 100, sr)
	highMax, highMin := bandMaxMin(highRipple, 1.2*fc, 0.4*sr, 100, sr)

	lowSpan := lowMax - lowMin
	highSpan := highMax - highMin

	if highSpan <= lowSpan+0.2 {
		t.Fatalf("rippleDB does not affect HP passband as expected: span@0.1dB=%.3f, span@1.0dB=%.3f", lowSpan, highSpan)
	}
}

// TestEllipticLP_PassbandHasInteriorRippleExtrema checks the passband shape is not
// a purely monotonic roll-off for order>=4; elliptic passbands should be equiripple.
func TestEllipticLP_PassbandHasInteriorRippleExtrema(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	sections := EllipticLP(fc, 4, 0.5, 40, sr)

	extrema := interiorExtremaCount(sections, 50, 0.95*fc, 10, sr)
	if extrema < 1 {
		t.Fatalf("LP passband appears monotonic (no interior extrema), expected equiripple behavior")
	}
}

// TestEllipticLPHP_EdgeCasesNearLimits validates stable, finite behavior near
// frequency and ripple extremes without relying on diagnostic logging tests.
func TestEllipticLPHP_EdgeCasesNearLimits(t *testing.T) {
	const sr = 48000.0

	cases := []struct {
		name       string
		fc         float64
		order      int
		rippleDB   float64
		stopbandDB float64
	}{
		{name: "tiny cutoff", fc: 5.0, order: 4, rippleDB: 0.1, stopbandDB: 40},
		{name: "low cutoff", fc: 20.0, order: 6, rippleDB: 0.5, stopbandDB: 60},
		{name: "near nyquist", fc: sr * 0.499, order: 4, rippleDB: 0.5, stopbandDB: 40},
		{name: "very small ripple", fc: 1000.0, order: 4, rippleDB: 0.01, stopbandDB: 40},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lp := EllipticLP(tc.fc, tc.order, tc.rippleDB, tc.stopbandDB, sr)
			hp := EllipticHP(tc.fc, tc.order, tc.rippleDB, tc.stopbandDB, sr)
			if len(lp) == 0 || len(hp) == 0 {
				t.Fatalf("expected non-empty sections for %+v", tc)
			}
			for _, s := range lp {
				assertFiniteCoefficients(t, s)
				assertStableSection(t, s)
			}
			for _, s := range hp {
				assertFiniteCoefficients(t, s)
				assertStableSection(t, s)
			}
		})
	}
}
