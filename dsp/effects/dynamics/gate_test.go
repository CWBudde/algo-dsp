package dynamics

import (
	"math"
	"testing"
)

// TestNewGate verifies constructor with valid and invalid sample rates.
func TestNewGate(t *testing.T) {
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
			g, err := NewGate(tt.sampleRate)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewGate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && g == nil {
				t.Error("NewGate() returned nil without error")
			}
		})
	}
}

// TestGateDefaults verifies default parameter values.
func TestGateDefaults(t *testing.T) {
	g, err := NewGate(48000)
	if err != nil {
		t.Fatalf("NewGate() error = %v", err)
	}

	tests := []struct {
		name string
		got  float64
		want float64
	}{
		{"Threshold", g.Threshold(), defaultGateThresholdDB},
		{"Ratio", g.Ratio(), defaultGateRatio},
		{"Knee", g.Knee(), defaultGateKneeDB},
		{"Attack", g.Attack(), defaultGateAttackMs},
		{"Hold", g.Hold(), defaultGateHoldMs},
		{"Release", g.Release(), defaultGateReleaseMs},
		{"Range", g.Range(), defaultGateRangeDB},
		{"SampleRate", g.SampleRate(), 48000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %f, want %f", tt.name, tt.got, tt.want)
			}
		})
	}
}

// TestGateSetThreshold verifies threshold setter with valid and invalid values.
func TestGateSetThreshold(t *testing.T) {
	g, _ := NewGate(48000)

	tests := []struct {
		name    string
		value   float64
		wantErr bool
	}{
		{"valid -40", -40, false},
		{"valid -20", -20, false},
		{"valid 0", 0, false},
		{"valid -60", -60, false},
		{"invalid NaN", math.NaN(), true},
		{"invalid +Inf", math.Inf(1), true},
		{"invalid -Inf", math.Inf(-1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := g.SetThreshold(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetThreshold(%f) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}

			if !tt.wantErr && g.Threshold() != tt.value {
				t.Errorf("Threshold() = %f, want %f", g.Threshold(), tt.value)
			}
		})
	}
}

// TestGateSetRatio verifies ratio setter with valid and invalid values.
func TestGateSetRatio(t *testing.T) {
	g, _ := NewGate(48000)

	tests := []struct {
		name    string
		value   float64
		wantErr bool
	}{
		{"valid 1.0", 1.0, false},
		{"valid 2.0", 2.0, false},
		{"valid 10.0", 10.0, false},
		{"valid 100.0", 100.0, false},
		{"invalid 0.5", 0.5, true},
		{"invalid 101", 101, true},
		{"invalid NaN", math.NaN(), true},
		{"invalid +Inf", math.Inf(1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := g.SetRatio(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetRatio(%f) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}

			if !tt.wantErr && g.Ratio() != tt.value {
				t.Errorf("Ratio() = %f, want %f", g.Ratio(), tt.value)
			}
		})
	}
}

// TestGateSetKnee verifies knee setter with valid and invalid values.
func TestGateSetKnee(t *testing.T) {
	g, _ := NewGate(48000)

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
			err := g.SetKnee(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetKnee(%f) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}

			if !tt.wantErr && g.Knee() != tt.value {
				t.Errorf("Knee() = %f, want %f", g.Knee(), tt.value)
			}
		})
	}
}

// TestGateSetAttack verifies attack setter with valid and invalid values.
func TestGateSetAttack(t *testing.T) {
	g, _ := NewGate(48000)

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
			err := g.SetAttack(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetAttack(%f) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}

			if !tt.wantErr && g.Attack() != tt.value {
				t.Errorf("Attack() = %f, want %f", g.Attack(), tt.value)
			}
		})
	}
}

// TestGateSetHold verifies hold setter with valid and invalid values.
func TestGateSetHold(t *testing.T) {
	g, _ := NewGate(48000)

	tests := []struct {
		name    string
		value   float64
		wantErr bool
	}{
		{"valid 0", 0, false},
		{"valid 50", 50, false},
		{"valid 5000", 5000, false},
		{"invalid -1", -1, true},
		{"invalid 5001", 5001, true},
		{"invalid NaN", math.NaN(), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := g.SetHold(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetHold(%f) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}

			if !tt.wantErr && g.Hold() != tt.value {
				t.Errorf("Hold() = %f, want %f", g.Hold(), tt.value)
			}
		})
	}
}

// TestGateSetRelease verifies release setter with valid and invalid values.
func TestGateSetRelease(t *testing.T) {
	g, _ := NewGate(48000)

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
			err := g.SetRelease(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetRelease(%f) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}

			if !tt.wantErr && g.Release() != tt.value {
				t.Errorf("Release() = %f, want %f", g.Release(), tt.value)
			}
		})
	}
}

// TestGateSetRange verifies range setter with valid and invalid values.
func TestGateSetRange(t *testing.T) {
	g, _ := NewGate(48000)

	tests := []struct {
		name    string
		value   float64
		wantErr bool
	}{
		{"valid -80", -80, false},
		{"valid -20", -20, false},
		{"valid 0", 0, false},
		{"valid -120", -120, false},
		{"invalid -121", -121, true},
		{"invalid 1", 1, true},
		{"invalid NaN", math.NaN(), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := g.SetRange(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetRange(%f) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}

			if !tt.wantErr && g.Range() != tt.value {
				t.Errorf("Range() = %f, want %f", g.Range(), tt.value)
			}
		})
	}
}

// TestGateCoefficientCalculations verifies internal coefficient computations.
func TestGateCoefficientCalculations(t *testing.T) {
	g, _ := NewGate(48000)

	// Test threshold conversion to log2
	if err := g.SetThreshold(-40); err != nil {
		t.Fatal(err)
	}

	expectedThresholdLog2 := -40 * log2Of10Div20
	if math.Abs(g.thresholdLog2-expectedThresholdLog2) > 1e-10 {
		t.Errorf("thresholdLog2 = %f, want %f", g.thresholdLog2, expectedThresholdLog2)
	}

	// Test knee width calculation
	if err := g.SetKnee(6); err != nil {
		t.Fatal(err)
	}

	expectedKneeWidthLog2 := 6.0 * log2Of10Div20
	if math.Abs(g.kneeWidthLog2-expectedKneeWidthLog2) > 1e-10 {
		t.Errorf("kneeWidthLog2 = %f, want %f", g.kneeWidthLog2, expectedKneeWidthLog2)
	}

	// Test attack coefficient (should be between 0 and 1)
	if g.attackCoeff <= 0 || g.attackCoeff >= 1 {
		t.Errorf("attackCoeff = %f, want (0, 1)", g.attackCoeff)
	}

	// Test release coefficient (should be between 0 and 1)
	if g.releaseCoeff <= 0 || g.releaseCoeff >= 1 {
		t.Errorf("releaseCoeff = %f, want (0, 1)", g.releaseCoeff)
	}

	// Test hold samples calculation
	if err := g.SetHold(50); err != nil {
		t.Fatal(err)
	}

	expectedHoldSamples := int(50.0 * 0.001 * 48000)
	if g.holdSamples != expectedHoldSamples {
		t.Errorf("holdSamples = %d, want %d", g.holdSamples, expectedHoldSamples)
	}

	// Test range conversion to linear
	if err := g.SetRange(-80); err != nil {
		t.Fatal(err)
	}

	expectedRangeLin := math.Pow(10, -80.0/20.0)
	if math.Abs(g.rangeLin-expectedRangeLin) > 1e-10 {
		t.Errorf("rangeLin = %e, want %e", g.rangeLin, expectedRangeLin)
	}
}

// TestGateGainAboveThreshold verifies no attenuation above threshold.
func TestGateGainAboveThreshold(t *testing.T) {
	g, _ := NewGate(48000)
	if err := g.SetThreshold(-40); err != nil {
		t.Fatal(err)
	}

	// Test various levels above threshold (-40 dB ≈ 0.01 linear)
	levels := []float64{0.1, 0.5, 1.0} // All well above -40 dB
	for _, level := range levels {
		gain := g.calculateGain(level)
		if gain != 1.0 {
			t.Errorf("calculateGain(%f) = %f, want 1.0 (above threshold)", level, gain)
		}
	}
}

// TestGateGainBelowThreshold verifies attenuation below threshold.
func TestGateGainBelowThreshold(t *testing.T) {
	g, _ := NewGate(48000)
	if err := g.SetThreshold(-20); err != nil {
		t.Fatal(err)
	}

	if err := g.SetRatio(10); err != nil {
		t.Fatal(err)
	}

	if err := g.SetKnee(0); err != nil { // Hard knee for predictable testing
		t.Fatal(err)
	}

	// Test level below threshold (-20 dB = 0.1 linear)
	level := 0.05 // Below threshold
	gain := g.calculateGain(level)

	if gain >= 1.0 {
		t.Errorf("calculateGain(%f) = %f, want < 1.0 (should gate)", level, gain)
	}

	if gain <= 0 {
		t.Errorf("calculateGain(%f) = %f, want > 0", level, gain)
	}
}

// TestGateGainRatios verifies different ratios produce progressively more gating.
func TestGateGainRatios(t *testing.T) {
	// Level below threshold for testing
	level := 0.01 // Well below -20 dB threshold

	var prevGain float64

	ratios := []float64{1.0, 2.0, 4.0, 10.0}

	for i, ratio := range ratios {
		g, _ := NewGate(48000)
		if err := g.SetThreshold(-20); err != nil {
			t.Fatal(err)
		}

		if err := g.SetRatio(ratio); err != nil {
			t.Fatal(err)
		}

		if err := g.SetKnee(0); err != nil {
			t.Fatal(err)
		}

		if err := g.SetRange(-120); err != nil { // Wide range so it doesn't clamp
			t.Fatal(err)
		}

		gain := g.calculateGain(level)

		if ratio == 1.0 && gain != 1.0 {
			t.Errorf("Ratio 1.0 should produce unity gain, got %f", gain)
		}

		if i > 0 && gain >= prevGain {
			t.Errorf("Ratio %f produced gain %f >= previous %f (expected less)",
				ratio, gain, prevGain)
		}

		prevGain = gain
	}
}

// TestGateGainSilence verifies that silence produces minimum gain.
func TestGateGainSilence(t *testing.T) {
	g, _ := NewGate(48000)

	gain := g.calculateGain(0)
	if gain != g.rangeLin {
		t.Errorf("calculateGain(0) = %e, want %e (rangeLin)", gain, g.rangeLin)
	}
}

// TestGateRangeClamp verifies that gain never drops below rangeLin.
func TestGateRangeClamp(t *testing.T) {
	g, _ := NewGate(48000)
	if err := g.SetThreshold(-10); err != nil {
		t.Fatal(err)
	}

	if err := g.SetRatio(100); err != nil {
		t.Fatal(err)
	}

	if err := g.SetRange(-40); err != nil {
		t.Fatal(err)
	}

	if err := g.SetKnee(0); err != nil {
		t.Fatal(err)
	}

	// Very low level should be clamped to range
	gain := g.calculateGain(0.0001)

	expectedMin := math.Pow(10, -40.0/20.0)
	if gain < expectedMin-1e-10 {
		t.Errorf("calculateGain(0.0001) = %e, want >= %e (range clamp)", gain, expectedMin)
	}
}

// TestGateSoftKneeTransition verifies smooth transition in the knee region.
func TestGateSoftKneeTransition(t *testing.T) {
	g, _ := NewGate(48000)
	if err := g.SetThreshold(-20); err != nil {
		t.Fatal(err)
	}

	if err := g.SetRatio(10); err != nil {
		t.Fatal(err)
	}

	if err := g.SetKnee(12); err != nil {
		t.Fatal(err)
	}

	// Sample levels around the threshold region
	// -20 dB = 0.1 linear. Knee of 12 dB means transition from -14 to -26 dB
	levels := []float64{0.25, 0.15, 0.1, 0.07, 0.05, 0.03}
	prevGain := 2.0 // Start above any possible gain

	for _, level := range levels {
		gain := g.calculateGain(level)

		// Gains should be monotonically decreasing as level drops
		if gain > prevGain+1e-10 {
			t.Errorf("Gain increased at level %f: %f > previous %f", level, gain, prevGain)
		}

		// Gain should always be in [rangeLin, 1.0]
		if gain > 1.0+1e-10 || gain < g.rangeLin-1e-10 {
			t.Errorf("Gain %f out of range [%e, 1.0] at level %f", gain, g.rangeLin, level)
		}

		prevGain = gain
	}
}

// TestGateProcessSampleZero verifies zero input produces zero output.
func TestGateProcessSampleZero(t *testing.T) {
	g, _ := NewGate(48000)
	g.Reset()

	for i := 0; i < 100; i++ {
		output := g.ProcessSample(0)
		if output != 0 {
			t.Errorf("ProcessSample(0) = %f, want 0", output)
			break
		}
	}
}

// TestGateProcessInPlaceMatchesSample verifies consistency between processing methods.
func TestGateProcessInPlaceMatchesSample(t *testing.T) {
	// Create two identical gates
	g1, _ := NewGate(48000)
	g2, _ := NewGate(48000)

	// Generate test signal with varying levels
	input := make([]float64, 256)
	for i := range input {
		// Mix of loud and quiet sections
		amplitude := 0.5
		if i >= 100 && i < 200 {
			amplitude = 0.001 // Below threshold
		}

		input[i] = amplitude * math.Sin(2*math.Pi*440*float64(i)/48000)
	}

	// Process with ProcessSample
	want := make([]float64, len(input))
	for i := range input {
		want[i] = g1.ProcessSample(input[i])
	}

	// Process with ProcessInPlace
	got := make([]float64, len(input))
	copy(got, input)
	g2.ProcessInPlace(got)

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

// TestGateReset verifies reset clears state.
func TestGateReset(t *testing.T) {
	g, _ := NewGate(48000)

	// Process some samples to build up state
	for i := 0; i < 100; i++ {
		g.ProcessSample(0.5)
	}

	// Verify state was built up
	if g.peakLevel == 0 {
		t.Error("Peak level should be non-zero after processing")
	}

	// Reset
	g.Reset()

	// Verify state was cleared
	if g.peakLevel != 0 {
		t.Errorf("Peak level = %f after Reset(), want 0", g.peakLevel)
	}

	if g.holdCounter != 0 {
		t.Errorf("holdCounter = %d after Reset(), want 0", g.holdCounter)
	}

	metrics := g.GetMetrics()
	if metrics.InputPeak != 0 || metrics.OutputPeak != 0 {
		t.Error("Metrics should be cleared after Reset()")
	}
}

// TestGateMetricsTracking verifies metrics are tracked correctly.
func TestGateMetricsTracking(t *testing.T) {
	g, _ := NewGate(48000)
	if err := g.SetThreshold(-10); err != nil {
		t.Fatal(err)
	}

	if err := g.SetHold(0); err != nil { // Disable hold for this test
		t.Fatal(err)
	}

	g.ResetMetrics()

	// Process quiet samples (below threshold) to trigger gating
	for i := 0; i < 500; i++ {
		g.ProcessSample(0.01)
	}

	metrics := g.GetMetrics()

	// Check input peak
	if math.Abs(metrics.InputPeak-0.01) > 1e-10 {
		t.Errorf("InputPeak = %f, want 0.01", metrics.InputPeak)
	}

	// GainReduction should be less than 1.0 (gating is active)
	if metrics.GainReduction >= 1.0 {
		t.Errorf("GainReduction = %f, should be less than 1.0 for quiet signals", metrics.GainReduction)
	}
}

// TestGateHoldBehavior verifies that the hold mechanism prevents premature closing.
func TestGateHoldBehavior(t *testing.T) {
	g, _ := NewGate(48000)
	if err := g.SetThreshold(-20); err != nil {
		t.Fatal(err)
	}

	if err := g.SetAttack(0.1); err != nil { // Very fast attack
		t.Fatal(err)
	}

	if err := g.SetRelease(1); err != nil { // Very fast release so envelope decays quickly
		t.Fatal(err)
	}

	if err := g.SetHold(10); err != nil { // 10 ms hold = 480 samples
		t.Fatal(err)
	}

	if err := g.SetKnee(0); err != nil {
		t.Fatal(err)
	}

	g.Reset()

	// Build up envelope above threshold
	for i := 0; i < 2000; i++ {
		g.ProcessSample(0.5) // Well above threshold
	}

	// Confirm hold counter is set
	if g.holdCounter != g.holdSamples {
		t.Errorf("holdCounter = %d, want %d after loud signal", g.holdCounter, g.holdSamples)
	}

	// Feed silence — the envelope decays via the fast release (1ms).
	// Once the envelope drops below threshold, the hold counter starts decrementing.
	// We process enough samples for the envelope to decay AND the hold to expire.
	// With 1ms release at 48kHz, the envelope drops very quickly (< 100 samples),
	// then 480 hold samples need to count down.
	totalSamples := 1000 + g.holdSamples // Generous margin for envelope decay + hold
	for i := 0; i < totalSamples; i++ {
		g.ProcessSample(0)
	}

	// After envelope decay + hold period, hold counter should be 0
	if g.holdCounter != 0 {
		t.Errorf("holdCounter = %d after decay+hold period, want 0", g.holdCounter)
	}
}

// TestGateHoldCounterResets verifies hold counter resets when signal returns above threshold.
func TestGateHoldCounterResets(t *testing.T) {
	g, _ := NewGate(48000)
	if err := g.SetThreshold(-20); err != nil {
		t.Fatal(err)
	}

	if err := g.SetHold(100); err != nil { // Long hold
		t.Fatal(err)
	}

	if err := g.SetAttack(0.1); err != nil {
		t.Fatal(err)
	}

	g.Reset()

	// Build up above threshold
	for i := 0; i < 2000; i++ {
		g.ProcessSample(0.5)
	}

	initialHold := g.holdCounter

	// Process a few silence samples to start decrementing hold
	for i := 0; i < 10; i++ {
		g.ProcessSample(0)
	}

	// Hold should have decremented (envelope may still be above threshold
	// due to release, but once it drops, hold starts counting down)

	// Return to loud signal
	for i := 0; i < 2000; i++ {
		g.ProcessSample(0.5)
	}

	// Hold counter should be reset to full
	if g.holdCounter != initialHold {
		t.Errorf("holdCounter = %d, want %d (should reset on loud signal)", g.holdCounter, initialHold)
	}
}

// TestGateEnvelopeFollowerAttack verifies gate opening behavior.
func TestGateEnvelopeFollowerAttack(t *testing.T) {
	g, _ := NewGate(48000)
	if err := g.SetAttack(1); err != nil { // Fast 1ms attack
		t.Fatal(err)
	}

	g.Reset()

	// Apply step input
	const level = 0.5

	prevPeak := 0.0

	for i := 0; i < 1000; i++ {
		g.ProcessSample(level)

		if g.peakLevel < prevPeak-1e-10 {
			t.Errorf("Peak decreased during attack at sample %d: %f -> %f", i, prevPeak, g.peakLevel)
			break
		}

		prevPeak = g.peakLevel
	}

	// After 1000 samples, should be close to target
	if g.peakLevel < 0.45 {
		t.Errorf("Peak = %f after attack, expected >= 0.45 (approaching %f)", g.peakLevel, level)
	}
}

// TestGateEnvelopeFollowerRelease verifies gate closing behavior.
func TestGateEnvelopeFollowerRelease(t *testing.T) {
	g, _ := NewGate(48000)
	if err := g.SetAttack(1); err != nil {
		t.Fatal(err)
	}

	if err := g.SetRelease(50); err != nil {
		t.Fatal(err)
	}

	if err := g.SetHold(0); err != nil { // Disable hold for this test
		t.Fatal(err)
	}

	g.Reset()

	// Build up peak
	for i := 0; i < 2000; i++ {
		g.ProcessSample(0.5)
	}

	peakAfterAttack := g.peakLevel

	if peakAfterAttack < 0.4 {
		t.Fatalf("Peak not built up properly: %f (expected >= 0.4)", peakAfterAttack)
	}

	// Process silence for release
	prevPeak := peakAfterAttack

	for i := 0; i < 5000; i++ {
		g.ProcessSample(0)

		if g.peakLevel > prevPeak+1e-10 {
			t.Errorf("Peak increased during release at sample %d: %f -> %f", i, prevPeak, g.peakLevel)
			break
		}

		prevPeak = g.peakLevel
	}

	// After ~100ms, should have decayed significantly
	if g.peakLevel >= peakAfterAttack*0.25 {
		t.Errorf("Peak = %f after release, want < %f", g.peakLevel, peakAfterAttack*0.25)
	}
}

// TestGateCalculateOutputLevel verifies static curve visualization helper.
func TestGateCalculateOutputLevel(t *testing.T) {
	g, _ := NewGate(48000)
	if err := g.SetThreshold(-20); err != nil {
		t.Fatal(err)
	}

	if err := g.SetRatio(10); err != nil {
		t.Fatal(err)
	}

	if err := g.SetKnee(0); err != nil {
		t.Fatal(err)
	}

	// Above threshold: output = input
	out := g.CalculateOutputLevel(0.5)
	if math.Abs(out-0.5) > 1e-10 {
		t.Errorf("CalculateOutputLevel(0.5) = %f, want 0.5 (above threshold)", out)
	}

	// Below threshold: output < input
	out = g.CalculateOutputLevel(0.05)
	if out >= 0.05 {
		t.Errorf("CalculateOutputLevel(0.05) = %f, want < 0.05 (below threshold)", out)
	}

	if out <= 0 {
		t.Errorf("CalculateOutputLevel(0.05) = %f, want > 0", out)
	}
}

// TestGateSetSampleRate verifies sample rate updates.
func TestGateSetSampleRate(t *testing.T) {
	g, _ := NewGate(48000)

	tests := []struct {
		name    string
		value   float64
		wantErr bool
	}{
		{"valid 44100", 44100, false},
		{"valid 96000", 96000, false},
		{"invalid zero", 0, true},
		{"invalid negative", -1, true},
		{"invalid NaN", math.NaN(), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := g.SetSampleRate(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetSampleRate(%f) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}

			if !tt.wantErr && g.SampleRate() != tt.value {
				t.Errorf("SampleRate() = %f, want %f", g.SampleRate(), tt.value)
			}
		})
	}
}

// TestGateRatioOnePassthrough verifies ratio 1:1 produces no gating.
func TestGateRatioOnePassthrough(t *testing.T) {
	g, _ := NewGate(48000)
	if err := g.SetRatio(1.0); err != nil {
		t.Fatal(err)
	}

	if err := g.SetKnee(0); err != nil {
		t.Fatal(err)
	}

	// Even well below threshold, ratio 1:1 should produce unity gain
	levels := []float64{0.001, 0.01, 0.05, 0.1, 0.5}
	for _, level := range levels {
		gain := g.calculateGain(level)
		if gain != 1.0 {
			t.Errorf("calculateGain(%f) = %f with ratio 1.0, want 1.0", level, gain)
		}
	}
}

// TestGateNegativeInput verifies gate handles negative samples correctly.
func TestGateNegativeInput(t *testing.T) {
	g, _ := NewGate(48000)
	g.Reset()

	// Process negative sample — should behave same as positive
	out := g.ProcessSample(-0.5)
	if out > 0 {
		t.Errorf("ProcessSample(-0.5) = %f, want negative output", out)
	}
}
