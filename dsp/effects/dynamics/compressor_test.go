package dynamics

import (
	"math"
	"testing"
)

// TestNewCompressor verifies constructor with valid and invalid sample rates.
func TestNewCompressor(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate float64
		wantErr    bool
	}{
		{"valid 44100", 44100, false},
		{"valid 48000", 48000, false},
		{"valid 96000", 96000, false},
		{"invalid zero", 0, true},
		{"invalid negative", -1, true},
		{"invalid NaN", math.NaN(), true},
		{"invalid +Inf", math.Inf(1), true},
		{"invalid -Inf", math.Inf(-1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewCompressor(tt.sampleRate)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCompressor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && c == nil {
				t.Error("NewCompressor() returned nil without error")
			}
		})
	}
}

// TestCompressorDefaults verifies default parameter values.
func TestCompressorDefaults(t *testing.T) {
	c, err := NewCompressor(48000)
	if err != nil {
		t.Fatalf("NewCompressor() error = %v", err)
	}

	tests := []struct {
		name string
		got  float64
		want float64
	}{
		{"Threshold", c.Threshold(), defaultCompressorThresholdDB},
		{"Ratio", c.Ratio(), defaultCompressorRatio},
		{"Knee", c.Knee(), defaultCompressorKneeDB},
		{"Attack", c.Attack(), defaultCompressorAttackMs},
		{"Release", c.Release(), defaultCompressorReleaseMs},
		{"SampleRate", c.SampleRate(), 48000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %f, want %f", tt.name, tt.got, tt.want)
			}
		})
	}

	if !c.AutoMakeup() {
		t.Error("AutoMakeup should be enabled by default")
	}
}

// TestSetThreshold verifies threshold setter with valid and invalid values.
func TestSetThreshold(t *testing.T) {
	c, _ := NewCompressor(48000)

	tests := []struct {
		name    string
		value   float64
		wantErr bool
	}{
		{"valid -20", -20, false},
		{"valid 0", 0, false},
		{"valid -60", -60, false},
		{"valid positive", 10, false}, // No hard limit
		{"invalid NaN", math.NaN(), true},
		{"invalid +Inf", math.Inf(1), true},
		{"invalid -Inf", math.Inf(-1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.SetThreshold(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetThreshold(%f) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}

			if !tt.wantErr && c.Threshold() != tt.value {
				t.Errorf("Threshold() = %f, want %f", c.Threshold(), tt.value)
			}
		})
	}
}

// TestSetRatio verifies ratio setter with valid and invalid values.
func TestSetRatio(t *testing.T) {
	c, _ := NewCompressor(48000)

	tests := []struct {
		name    string
		value   float64
		wantErr bool
	}{
		{"valid 1.0", 1.0, false},
		{"valid 4.0", 4.0, false},
		{"valid 100.0", 100.0, false},
		{"invalid 0.5", 0.5, true},
		{"invalid 101", 101, true},
		{"invalid NaN", math.NaN(), true},
		{"invalid +Inf", math.Inf(1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.SetRatio(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetRatio(%f) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}

			if !tt.wantErr && c.Ratio() != tt.value {
				t.Errorf("Ratio() = %f, want %f", c.Ratio(), tt.value)
			}
		})
	}
}

// TestSetKnee verifies knee setter with valid and invalid values.
func TestSetKnee(t *testing.T) {
	c, _ := NewCompressor(48000)

	tests := []struct {
		name    string
		value   float64
		wantErr bool
	}{
		{"valid 0", 0, false},
		{"valid 6", 6, false},
		{"valid 24", 24, false},
		{"invalid -1", -1, true},
		{"invalid 25", 25, true},
		{"invalid NaN", math.NaN(), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.SetKnee(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetKnee(%f) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}

			if !tt.wantErr && c.Knee() != tt.value {
				t.Errorf("Knee() = %f, want %f", c.Knee(), tt.value)
			}
		})
	}
}

// TestSetAttack verifies attack setter with valid and invalid values.
func TestSetAttack(t *testing.T) {
	c, _ := NewCompressor(48000)

	tests := []struct {
		name    string
		value   float64
		wantErr bool
	}{
		{"valid 0.1", 0.1, false},
		{"valid 10", 10, false},
		{"valid 1000", 1000, false},
		{"invalid 0.05", 0.05, true},
		{"invalid 1001", 1001, true},
		{"invalid NaN", math.NaN(), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.SetAttack(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetAttack(%f) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}

			if !tt.wantErr && c.Attack() != tt.value {
				t.Errorf("Attack() = %f, want %f", c.Attack(), tt.value)
			}
		})
	}
}

// TestSetRelease verifies release setter with valid and invalid values.
func TestSetRelease(t *testing.T) {
	c, _ := NewCompressor(48000)

	tests := []struct {
		name    string
		value   float64
		wantErr bool
	}{
		{"valid 1", 1, false},
		{"valid 100", 100, false},
		{"valid 5000", 5000, false},
		{"invalid 0.5", 0.5, true},
		{"invalid 5001", 5001, true},
		{"invalid NaN", math.NaN(), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.SetRelease(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetRelease(%f) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}

			if !tt.wantErr && c.Release() != tt.value {
				t.Errorf("Release() = %f, want %f", c.Release(), tt.value)
			}
		})
	}
}

// TestSetMakeupGain verifies makeup gain setter.
func TestSetMakeupGain(t *testing.T) {
	c, _ := NewCompressor(48000)

	tests := []struct {
		name    string
		value   float64
		wantErr bool
	}{
		{"valid 0", 0, false},
		{"valid 6", 6, false},
		{"valid -10", -10, false},
		{"invalid NaN", math.NaN(), true},
		{"invalid +Inf", math.Inf(1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.SetMakeupGain(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetMakeupGain(%f) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}

			if !tt.wantErr {
				if c.MakeupGain() != tt.value {
					t.Errorf("MakeupGain() = %f, want %f", c.MakeupGain(), tt.value)
				}

				if c.AutoMakeup() {
					t.Error("SetMakeupGain should disable AutoMakeup")
				}
			}
		})
	}
}

// TestSetAutoMakeup verifies auto makeup gain toggle.
func TestSetAutoMakeup(t *testing.T) {
	c, _ := NewCompressor(48000)

	// Disable auto makeup
	if err := c.SetAutoMakeup(false); err != nil {
		t.Fatalf("SetAutoMakeup(false) error = %v", err)
	}

	if c.AutoMakeup() {
		t.Error("AutoMakeup() should be false")
	}

	// Re-enable auto makeup
	if err := c.SetAutoMakeup(true); err != nil {
		t.Fatalf("SetAutoMakeup(true) error = %v", err)
	}

	if !c.AutoMakeup() {
		t.Error("AutoMakeup() should be true")
	}
}

// TestCoefficientCalculations verifies internal coefficient computations.
func TestCoefficientCalculations(t *testing.T) {
	c, _ := NewCompressor(48000)

	// Test threshold conversion to log2
	if err := c.SetThreshold(-20); err != nil {
		t.Fatal(err)
	}

	expectedThresholdLog2 := -20 * log2Of10Div20
	if math.Abs(c.thresholdLog2-expectedThresholdLog2) > 1e-10 {
		t.Errorf("thresholdLog2 = %f, want %f", c.thresholdLog2, expectedThresholdLog2)
	}

	// Test knee width calculation
	if err := c.SetKnee(6); err != nil {
		t.Fatal(err)
	}

	expectedKneeWidthLog2 := 6.0 * log2Of10Div20
	if math.Abs(c.kneeWidthLog2-expectedKneeWidthLog2) > 1e-10 {
		t.Errorf("kneeWidthLog2 = %f, want %f", c.kneeWidthLog2, expectedKneeWidthLog2)
	}

	// Test attack coefficient (should be between 0 and 1)
	if c.attackCoeff <= 0 || c.attackCoeff >= 1 {
		t.Errorf("attackCoeff = %f, want (0, 1)", c.attackCoeff)
	}

	// Test release coefficient (should be between 0 and 1)
	if c.releaseCoeff <= 0 || c.releaseCoeff >= 1 {
		t.Errorf("releaseCoeff = %f, want (0, 1)", c.releaseCoeff)
	}
}

// TestAutoMakeupGainCalculation verifies auto makeup gain formula.
func TestAutoMakeupGainCalculation(t *testing.T) {
	c, _ := NewCompressor(48000)

	// Set specific parameters
	if err := c.SetThreshold(-20); err != nil {
		t.Fatal(err)
	}

	if err := c.SetRatio(4); err != nil {
		t.Fatal(err)
	}

	if err := c.SetAutoMakeup(true); err != nil {
		t.Fatal(err)
	}

	// Expected: -threshold * (1 - 1/ratio) = -(-20) * (1 - 0.25) = 15
	expectedMakeup := -(-20) * (1.0 - 1.0/4.0)
	if math.Abs(c.MakeupGain()-expectedMakeup) > 1e-10 {
		t.Errorf("MakeupGain() = %f, want %f", c.MakeupGain(), expectedMakeup)
	}
}

// TestGainCalculationBelowThreshold verifies no compression below threshold.
func TestGainCalculationBelowThreshold(t *testing.T) {
	c, _ := NewCompressor(48000)
	if err := c.SetThreshold(-20); err != nil {
		t.Fatal(err)
	}

	// Test various levels below threshold
	levels := []float64{0.001, 0.01, 0.05} // All below -20dB
	for _, level := range levels {
		gain := c.calculateGain(level)
		if gain != 1.0 {
			t.Errorf("calculateGain(%f) = %f, want 1.0 (below threshold)", level, gain)
		}
	}
}

// TestGainCalculationAboveThreshold verifies compression above threshold.
func TestGainCalculationAboveThreshold(t *testing.T) {
	c, _ := NewCompressor(48000)
	if err := c.SetThreshold(-20); err != nil {
		t.Fatal(err)
	}

	if err := c.SetRatio(4); err != nil {
		t.Fatal(err)
	}

	if err := c.SetKnee(0); err != nil { // Hard knee for predictable testing
		t.Fatal(err)
	}

	// Test level above threshold (-20dB = 0.1 linear)
	level := 0.2 // Above threshold
	gain := c.calculateGain(level)

	if gain >= 1.0 {
		t.Errorf("calculateGain(%f) = %f, want < 1.0 (should compress)", level, gain)
	}

	if gain <= 0 {
		t.Errorf("calculateGain(%f) = %f, want > 0", level, gain)
	}
}

// TestGainCalculationRatios verifies different ratios.
func TestGainCalculationRatios(t *testing.T) {
	levels := []float64{0.2, 0.3} // Levels above threshold

	tests := []struct {
		ratio        float64
		wantLessGain bool // Higher ratio = more compression = less gain
	}{
		{1.0, false}, // No compression
		{2.0, false},
		{4.0, false},
		{10.0, false},
	}

	var prevGain float64

	for i, tt := range tests {
		c, _ := NewCompressor(48000)
		if err := c.SetThreshold(-20); err != nil {
			t.Fatal(err)
		}

		if err := c.SetRatio(tt.ratio); err != nil {
			t.Fatal(err)
		}

		if err := c.SetKnee(0); err != nil {
			t.Fatal(err)
		}

		gain := c.calculateGain(levels[0])

		if tt.ratio == 1.0 && gain != 1.0 {
			t.Errorf("Ratio 1.0 should produce unity gain, got %f", gain)
		}

		if i > 0 && gain >= prevGain {
			t.Errorf("Ratio %f produced gain %f >= previous %f (expected less)",
				tt.ratio, gain, prevGain)
		}

		prevGain = gain
	}
}

// TestProcessSampleZero verifies zero input produces zero output.
func TestProcessSampleZero(t *testing.T) {
	c, _ := NewCompressor(48000)
	c.Reset()

	for i := 0; i < 100; i++ {
		output := c.ProcessSample(0)
		if output != 0 {
			t.Errorf("ProcessSample(0) = %f, want 0", output)
			break
		}
	}
}

// TestProcessInPlaceMatchesSample verifies consistency between processing methods.
func TestProcessInPlaceMatchesSample(t *testing.T) {
	// Create two identical compressors
	c1, _ := NewCompressor(48000)
	c2, _ := NewCompressor(48000)

	// Generate test signal
	input := make([]float64, 256)
	for i := range input {
		input[i] = 0.1 * math.Sin(2*math.Pi*440*float64(i)/48000)
	}

	// Process with ProcessSample
	want := make([]float64, len(input))
	for i := range input {
		want[i] = c1.ProcessSample(input[i])
	}

	// Process with ProcessInPlace
	got := make([]float64, len(input))
	copy(got, input)
	c2.ProcessInPlace(got)

	// Compare results
	const tolerance = 1e-12

	for i := range got {
		diff := math.Abs(got[i] - want[i])
		if diff > tolerance {
			t.Errorf("sample %d: ProcessInPlace() = %f, ProcessSample() = %f, diff = %g",
				i, got[i], want[i], diff)

			break
		}
	}
}

// TestReset verifies reset clears state.
func TestReset(t *testing.T) {
	c, _ := NewCompressor(48000)

	// Process some samples to build up state
	for i := 0; i < 100; i++ {
		c.ProcessSample(0.5)
	}

	// Verify state was built up
	if c.peakLevel == 0 {
		t.Error("Peak level should be non-zero after processing")
	}

	// Reset
	c.Reset()

	// Verify state was cleared
	if c.peakLevel != 0 {
		t.Errorf("Peak level = %f after Reset(), want 0", c.peakLevel)
	}

	metrics := c.GetMetrics()
	if metrics.InputPeak != 0 || metrics.OutputPeak != 0 {
		t.Error("Metrics should be cleared after Reset()")
	}
}

// TestMetricsTracking verifies metrics are tracked correctly.
func TestMetricsTracking(t *testing.T) {
	c, _ := NewCompressor(48000)
	// Set low threshold and fast attack to ensure compression happens quickly
	if err := c.SetThreshold(-20); err != nil {
		t.Fatal(err)
	}

	if err := c.SetAttack(1); err != nil { // Very fast attack
		t.Fatal(err)
	}

	c.ResetMetrics()

	// Process many loud samples to build up envelope and trigger compression
	for i := 0; i < 500; i++ {
		c.ProcessSample(0.8)
	}

	metrics := c.GetMetrics()

	// Check input peak
	if metrics.InputPeak != 0.8 {
		t.Errorf("InputPeak = %f, want 0.8", metrics.InputPeak)
	}

	// Check that output peak and gain reduction were tracked
	if metrics.OutputPeak == 0 {
		t.Error("OutputPeak should be non-zero")
	}

	if metrics.GainReduction >= 1.0 {
		t.Errorf("GainReduction = %f, should be less than 1.0 for loud signals with low threshold", metrics.GainReduction)
	}
}

// TestEnvelopeFollowerAttack verifies attack phase behavior.
func TestEnvelopeFollowerAttack(t *testing.T) {
	c, _ := NewCompressor(48000)
	if err := c.SetAttack(1); err != nil { // Very fast 1ms attack
		t.Fatal(err)
	}

	c.Reset()

	// Apply step input
	const level = 0.5

	prevPeak := 0.0

	// Process enough samples - envelope followers approach asymptotically
	// With 1ms attack at 48kHz, we need several time constants (5-10ms) to approach target
	for i := 0; i < 1000; i++ {
		c.ProcessSample(level)
		// Peak should increase monotonically during attack (or stay constant when settled)
		if c.peakLevel < prevPeak-1e-10 { // Allow tiny numerical errors
			t.Errorf("Peak decreased during attack at sample %d: %f -> %f", i, prevPeak, c.peakLevel)
			break
		}

		prevPeak = c.peakLevel
	}

	// After 1000 samples (~20ms), should be reasonably close to target level
	// Envelope followers approach asymptotically, so we allow some tolerance
	if c.peakLevel < 0.45 {
		t.Errorf("Peak = %f after attack, expected >= 0.45 (approaching %f)", c.peakLevel, level)
	}
}

// TestEnvelopeFollowerRelease verifies release phase behavior.
func TestEnvelopeFollowerRelease(t *testing.T) {
	c, _ := NewCompressor(48000)
	if err := c.SetAttack(1); err != nil { // Fast attack to build up quickly
		t.Fatal(err)
	}

	if err := c.SetRelease(50); err != nil { // 50ms release
		t.Fatal(err)
	}

	c.Reset()

	// Build up peak - process for longer to ensure peak is settled
	for i := 0; i < 2000; i++ {
		c.ProcessSample(0.5)
	}

	peakAfterAttack := c.peakLevel

	// Verify peak was built up (envelope followers approach asymptotically)
	if peakAfterAttack < 0.4 {
		t.Fatalf("Peak not built up properly: %f (expected >= 0.4)", peakAfterAttack)
	}

	// Process silence for release (50ms * 48kHz = 2400 samples, so 5000 is more than sufficient)
	prevPeak := peakAfterAttack

	for i := 0; i < 5000; i++ {
		c.ProcessSample(0)
		// Peak should decrease monotonically during release (or stay at zero when settled)
		if c.peakLevel > prevPeak+1e-10 { // Allow tiny numerical errors
			t.Errorf("Peak increased during release at sample %d: %f -> %f", i, prevPeak, c.peakLevel)
			break
		}

		prevPeak = c.peakLevel
	}

	// After 5000 samples (~100ms, which is 2x the release time), should have decayed significantly
	// Allow 25% of original peak since envelope followers decay exponentially
	if c.peakLevel >= peakAfterAttack*0.25 {
		t.Errorf("Peak = %f after release, want < %f", c.peakLevel, peakAfterAttack*0.25)
	}
}
