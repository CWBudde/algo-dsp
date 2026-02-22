package resample

import (
	"math"
	"testing"
)

func TestNewForRatesCommon(t *testing.T) {
	r, err := NewForRates(44100, 48000)
	if err != nil {
		t.Fatalf("NewForRates() error = %v", err)
	}

	up, down := r.Ratio()
	if up != 160 || down != 147 {
		t.Fatalf("ratio = %d/%d, want 160/147", up, down)
	}
}

func TestQualityModes_PassbandAndStopband(t *testing.T) {
	tests := []struct {
		name          string
		quality       Quality
		maxPassbandDB float64
		minStopbandDB float64
	}{
		{name: "fast", quality: QualityFast, maxPassbandDB: 0.7, minStopbandDB: 20},
		{name: "balanced", quality: QualityBalanced, maxPassbandDB: 0.35, minStopbandDB: 35},
		{name: "best", quality: QualityBest, maxPassbandDB: 0.2, minStopbandDB: 50},
	}

	for _, tc := range tests {
		rPass, err := NewRational(1, 2, WithQuality(tc.quality))
		if err != nil {
			t.Fatalf("%s: NewRational passband error = %v", tc.name, err)
		}

		rStop, err := NewRational(1, 2, WithQuality(tc.quality))
		if err != nil {
			t.Fatalf("%s: NewRational stopband error = %v", tc.name, err)
		}

		inPass := sine(2000, 48000, 32768)
		inStop := sine(17000, 48000, 32768)

		outPass := rPass.Process(inPass)
		outStop := rStop.Process(inStop)

		inPassRMS := rms(inPass[4096:])
		outPassRMS := rms(outPass[2048:])

		passbandDB := math.Abs(dbRatio(outPassRMS, inPassRMS))
		if passbandDB > tc.maxPassbandDB {
			t.Fatalf("%s: passband droop %.2f dB > %.2f dB", tc.name, passbandDB, tc.maxPassbandDB)
		}

		inStopRMS := rms(inStop[4096:])
		outStopRMS := rms(outStop[2048:])

		stopAttenDB := -dbRatio(outStopRMS, inStopRMS)
		if stopAttenDB < tc.minStopbandDB {
			t.Fatalf("%s: stopband attenuation %.2f dB < %.2f dB", tc.name, stopAttenDB, tc.minStopbandDB)
		}
	}
}
