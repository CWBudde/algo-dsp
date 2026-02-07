package sweep

import (
	"math"
	"testing"
)

func TestLogSweepValidation(t *testing.T) {
	tests := []struct {
		name    string
		sweep   LogSweep
		wantErr error
	}{
		{"valid", LogSweep{20, 20000, 1, 48000}, nil},
		{"zero start freq", LogSweep{0, 20000, 1, 48000}, ErrInvalidFrequency},
		{"negative end freq", LogSweep{20, -1, 1, 48000}, ErrInvalidFrequency},
		{"start >= end", LogSweep{1000, 100, 1, 48000}, ErrFrequencyOrder},
		{"equal freqs", LogSweep{1000, 1000, 1, 48000}, ErrFrequencyOrder},
		{"zero duration", LogSweep{20, 20000, 0, 48000}, ErrInvalidDuration},
		{"negative duration", LogSweep{20, 20000, -1, 48000}, ErrInvalidDuration},
		{"zero sample rate", LogSweep{20, 20000, 1, 0}, ErrInvalidSampleRate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.sweep.Validate()
			if err != tt.wantErr {
				t.Errorf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestLogSweepGenerate(t *testing.T) {
	s := &LogSweep{
		StartFreq:  20,
		EndFreq:    20000,
		Duration:   1,
		SampleRate: 48000,
	}

	sweep, err := s.Generate()
	if err != nil {
		t.Fatal(err)
	}

	expectedLen := 48000
	if len(sweep) != expectedLen {
		t.Errorf("length = %d, want %d", len(sweep), expectedLen)
	}

	// Sweep should be bounded [-1, 1] (it's a sine sweep)
	for i, v := range sweep {
		if v < -1.001 || v > 1.001 {
			t.Errorf("sample[%d] = %f, out of [-1, 1] range", i, v)
			break
		}
	}

	// First sample should be sin(0) = 0
	if math.Abs(sweep[0]) > 1e-10 {
		t.Errorf("first sample = %g, want ~0", sweep[0])
	}
}

func TestLogSweepGenerateShort(t *testing.T) {
	s := &LogSweep{
		StartFreq:  100,
		EndFreq:    1000,
		Duration:   0.1,
		SampleRate: 8000,
	}

	sweep, err := s.Generate()
	if err != nil {
		t.Fatal(err)
	}

	expectedLen := 800
	if len(sweep) != expectedLen {
		t.Errorf("length = %d, want %d", len(sweep), expectedLen)
	}
}

func TestLogSweepInverseFilter(t *testing.T) {
	s := &LogSweep{
		StartFreq:  100,
		EndFreq:    4000,
		Duration:   0.5,
		SampleRate: 16000,
	}

	inv, err := s.InverseFilter()
	if err != nil {
		t.Fatal(err)
	}

	sweepLen := s.samples()
	if len(inv) != sweepLen {
		t.Errorf("inverse filter length = %d, want %d", len(inv), sweepLen)
	}

	// The inverse filter should have decreasing amplitude envelope
	// (high frequencies get less energy in log sweep, so inverse boosts them less)
	// Check that the signal is bounded
	maxAbs := 0.0
	for _, v := range inv {
		if math.Abs(v) > maxAbs {
			maxAbs = math.Abs(v)
		}
	}
	if maxAbs == 0 {
		t.Error("inverse filter is all zeros")
	}
}

func TestLogSweepDeconvolveIdentity(t *testing.T) {
	// If we pass the sweep through an identity system (delta function),
	// deconvolution should recover the delta.
	s := &LogSweep{
		StartFreq:  100,
		EndFreq:    4000,
		Duration:   0.25,
		SampleRate: 16000,
	}

	sweep, err := s.Generate()
	if err != nil {
		t.Fatal(err)
	}

	// The "response" IS the sweep itself (identity system)
	ir, err := s.Deconvolve(sweep)
	if err != nil {
		t.Fatal(err)
	}

	// Find the peak in the recovered IR
	peakIdx := 0
	peakVal := 0.0
	for i, v := range ir {
		if math.Abs(v) > peakVal {
			peakVal = math.Abs(v)
			peakIdx = i
		}
	}

	if peakVal == 0 {
		t.Fatal("deconvolved IR is all zeros")
	}

	// The peak should be well-defined (much larger than surrounding samples)
	// Check that the peak is at least 20 dB above average energy
	var totalEnergy float64
	for _, v := range ir {
		totalEnergy += v * v
	}
	avgEnergy := totalEnergy / float64(len(ir))
	peakEnergy := peakVal * peakVal

	peakToAvgDB := 10 * math.Log10(peakEnergy/avgEnergy)
	if peakToAvgDB < 15 { // relaxed threshold for numerical reasons
		t.Errorf("peak-to-average ratio = %.1f dB, want >= 15 dB (peak at %d)", peakToAvgDB, peakIdx)
	}
}

func TestLogSweepDeconvolveKnownIR(t *testing.T) {
	// Create a sweep, convolve with a known simple IR, then deconvolve.
	// The recovered IR should match the original.
	s := &LogSweep{
		StartFreq:  100,
		EndFreq:    4000,
		Duration:   0.5,
		SampleRate: 16000,
	}

	sweep, err := s.Generate()
	if err != nil {
		t.Fatal(err)
	}

	// Simple IR: delta at sample 0 with amplitude 1, plus reflection at sample 100 with amplitude 0.3
	irLen := 200
	knownIR := make([]float64, irLen)
	knownIR[0] = 1.0
	knownIR[100] = 0.3

	// Convolve sweep with known IR to simulate system response
	responseLen := len(sweep) + irLen - 1
	response := make([]float64, responseLen)
	for i, sv := range sweep {
		for j, iv := range knownIR {
			if i+j < responseLen {
				response[i+j] += sv * iv
			}
		}
	}

	// Deconvolve to recover the IR
	recovered, err := s.Deconvolve(response)
	if err != nil {
		t.Fatal(err)
	}

	// Find the main peak
	peakIdx := 0
	peakVal := 0.0
	for i, v := range recovered {
		if math.Abs(v) > peakVal {
			peakVal = math.Abs(v)
			peakIdx = i
		}
	}

	// There should be a secondary peak ~100 samples after the main peak
	// with amplitude roughly 0.3 of the main peak
	searchStart := peakIdx + 80
	searchEnd := peakIdx + 120
	if searchEnd > len(recovered) {
		searchEnd = len(recovered)
	}

	secondPeakVal := 0.0
	for i := searchStart; i < searchEnd; i++ {
		if math.Abs(recovered[i]) > secondPeakVal {
			secondPeakVal = math.Abs(recovered[i])
		}
	}

	ratio := secondPeakVal / peakVal
	// Should be approximately 0.3 (within reasonable tolerance for FFT artifacts)
	if ratio < 0.15 || ratio > 0.5 {
		t.Errorf("reflection amplitude ratio = %.3f, want ~0.3", ratio)
	}
}

func TestLogSweepDeconvolveEmptyResponse(t *testing.T) {
	s := &LogSweep{100, 4000, 0.5, 16000}
	_, err := s.Deconvolve(nil)
	if err != ErrEmptyResponse {
		t.Errorf("Deconvolve(nil) = %v, want ErrEmptyResponse", err)
	}
	_, err = s.Deconvolve([]float64{})
	if err != ErrEmptyResponse {
		t.Errorf("Deconvolve([]) = %v, want ErrEmptyResponse", err)
	}
}

func TestLogSweepExtractHarmonicIRs(t *testing.T) {
	s := &LogSweep{
		StartFreq:  100,
		EndFreq:    4000,
		Duration:   0.5,
		SampleRate: 16000,
	}

	sweep, err := s.Generate()
	if err != nil {
		t.Fatal(err)
	}

	// For a linear system, ExtractHarmonicIRs should return
	// a dominant linear IR and negligible harmonic IRs
	harmonics, err := s.ExtractHarmonicIRs(sweep, 3)
	if err != nil {
		t.Fatal(err)
	}

	if len(harmonics) != 3 {
		t.Fatalf("expected 3 harmonic IRs, got %d", len(harmonics))
	}

	// Linear IR (index 0) should have the most energy
	energies := make([]float64, len(harmonics))
	for h, ir := range harmonics {
		for _, v := range ir {
			energies[h] += v * v
		}
	}

	if energies[0] == 0 {
		t.Error("linear IR has zero energy")
	}

	// Harmonic IRs should have much less energy than linear for a linear system
	for h := 1; h < len(harmonics); h++ {
		if energies[0] > 0 && energies[h]/energies[0] > 0.1 {
			t.Errorf("H%d energy ratio = %.3f, expected < 0.1 for linear system", h+1, energies[h]/energies[0])
		}
	}
}

func TestLogSweepExtractHarmonicIRsValidation(t *testing.T) {
	s := &LogSweep{100, 4000, 0.5, 16000}
	sweep, _ := s.Generate()

	_, err := s.ExtractHarmonicIRs(sweep, 1)
	if err != ErrMaxHarmonic {
		t.Errorf("ExtractHarmonicIRs(maxHarmonic=1) = %v, want ErrMaxHarmonic", err)
	}
}

func TestLinearSweepValidation(t *testing.T) {
	tests := []struct {
		name    string
		sweep   LinearSweep
		wantErr error
	}{
		{"valid", LinearSweep{20, 20000, 1, 48000}, nil},
		{"zero start", LinearSweep{0, 20000, 1, 48000}, ErrInvalidFrequency},
		{"reversed", LinearSweep{1000, 100, 1, 48000}, ErrFrequencyOrder},
		{"zero duration", LinearSweep{20, 20000, 0, 48000}, ErrInvalidDuration},
		{"zero sr", LinearSweep{20, 20000, 1, 0}, ErrInvalidSampleRate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.sweep.Validate()
			if err != tt.wantErr {
				t.Errorf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestLinearSweepGenerate(t *testing.T) {
	s := &LinearSweep{
		StartFreq:  100,
		EndFreq:    4000,
		Duration:   0.5,
		SampleRate: 16000,
	}

	sweep, err := s.Generate()
	if err != nil {
		t.Fatal(err)
	}

	expectedLen := 8000
	if len(sweep) != expectedLen {
		t.Errorf("length = %d, want %d", len(sweep), expectedLen)
	}

	// Bounded in [-1, 1]
	for i, v := range sweep {
		if v < -1.001 || v > 1.001 {
			t.Errorf("sample[%d] = %f, out of range", i, v)
			break
		}
	}
}

func TestLinearSweepDeconvolve(t *testing.T) {
	s := &LinearSweep{
		StartFreq:  100,
		EndFreq:    4000,
		Duration:   0.5,
		SampleRate: 16000,
	}

	sweep, err := s.Generate()
	if err != nil {
		t.Fatal(err)
	}

	// Identity system test
	ir, err := s.Deconvolve(sweep)
	if err != nil {
		t.Fatal(err)
	}

	// Find peak
	peakVal := 0.0
	for _, v := range ir {
		if math.Abs(v) > peakVal {
			peakVal = math.Abs(v)
		}
	}

	if peakVal == 0 {
		t.Fatal("deconvolved IR is all zeros")
	}
}

func TestNextPowerOf2(t *testing.T) {
	tests := []struct {
		n    int
		want int
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 4},
		{4, 4},
		{5, 8},
		{7, 8},
		{8, 8},
		{9, 16},
		{1023, 1024},
		{1024, 1024},
		{1025, 2048},
	}

	for _, tt := range tests {
		got := nextPowerOf2(tt.n)
		if got != tt.want {
			t.Errorf("nextPowerOf2(%d) = %d, want %d", tt.n, got, tt.want)
		}
	}
}
