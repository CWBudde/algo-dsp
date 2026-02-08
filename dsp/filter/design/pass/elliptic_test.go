package pass

import (
	"math"
	"testing"
)

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
	for freq := 10.0; freq < fc*0.8; freq += 10 {
		magDB := cascadeMagDB(sections, freq, sr)
		maxRipple = math.Max(maxRipple, math.Abs(magDB))
		minGain = math.Min(minGain, magDB)
	}

	// Passband ripple should be within the specified dB range.
	if maxRipple > rippleDB+0.1 {
		t.Errorf("passband ripple %.3f dB exceeds spec %.3f dB", maxRipple, rippleDB)
	}
	if minGain < -rippleDB-0.1 {
		t.Errorf("passband minimum %.3f dB exceeds ripple spec %.3f dB", minGain, rippleDB)
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
	for freq := fc * 2; freq < sr*0.45; freq += 100 {
		magDB := cascadeMagDB(sections, freq, sr)
		maxStopband = math.Max(maxStopband, magDB)
	}

	// Stopband should be at least as attenuated as specified.
	if maxStopband > -stopbandDB+1.0 {
		t.Errorf("stopband peak %.1f dB, expected below %.1f dB", maxStopband, -stopbandDB)
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
	for freq := fc * 1.2; freq < sr*0.4; freq += 100 {
		magDB := cascadeMagDB(sections, freq, sr)
		maxRipple = math.Max(maxRipple, math.Abs(magDB))
		minGain = math.Min(minGain, magDB)
	}

	// Passband ripple should be within spec.
	if maxRipple > rippleDB+0.2 {
		t.Errorf("HP passband ripple %.3f dB exceeds spec %.3f dB", maxRipple, rippleDB)
	}
	if minGain < -rippleDB-0.2 {
		t.Errorf("HP passband minimum %.3f dB exceeds ripple spec %.3f dB", minGain, rippleDB)
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
	for freq := 10.0; freq < fc*0.5; freq += 10 {
		magDB := cascadeMagDB(sections, freq, sr)
		maxStopband = math.Max(maxStopband, magDB)
	}

	// Stopband should meet spec.
	if maxStopband > -stopbandDB+1.0 {
		t.Errorf("HP stopband peak %.1f dB, expected below %.1f dB", maxStopband, -stopbandDB)
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
		t.Errorf("Nyquist gain %.3f dB, expected ~0 dB", nyqDB)
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
