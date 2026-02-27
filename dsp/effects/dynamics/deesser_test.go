package dynamics

import (
	"math"
	"testing"
)

// TestNewDeEsser verifies constructor with valid and invalid sample rates.
func TestNewDeEsser(t *testing.T) {
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
			d, err := NewDeEsser(tt.sampleRate)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDeEsser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && d == nil {
				t.Error("NewDeEsser() returned nil without error")
			}
		})
	}
}

// TestDeEsserDefaults verifies default parameter values.
func TestDeEsserDefaults(t *testing.T) {
	d, err := NewDeEsser(48000)
	if err != nil {
		t.Fatalf("NewDeEsser() error = %v", err)
	}

	tests := []struct {
		name string
		got  float64
		want float64
	}{
		{"Frequency", d.Frequency(), defaultDeEsserFreqHz},
		{"Q", d.Q(), defaultDeEsserQ},
		{"Threshold", d.Threshold(), defaultDeEsserThreshDB},
		{"Ratio", d.Ratio(), defaultDeEsserRatio},
		{"Knee", d.Knee(), defaultDeEsserKneeDB},
		{"Attack", d.Attack(), defaultDeEsserAttackMs},
		{"Release", d.Release(), defaultDeEsserReleaseMs},
		{"Range", d.Range(), defaultDeEsserRangeDB},
		{"SampleRate", d.SampleRate(), 48000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %g, want %g", tt.got, tt.want)
			}
		})
	}

	if d.Mode() != defaultDeEsserMode {
		t.Errorf("Mode() = %d, want %d", d.Mode(), defaultDeEsserMode)
	}

	if d.Detector() != defaultDeEsserDetector {
		t.Errorf("Detector() = %d, want %d", d.Detector(), defaultDeEsserDetector)
	}

	if d.Listen() != defaultDeEsserListen {
		t.Errorf("Listen() = %v, want %v", d.Listen(), defaultDeEsserListen)
	}

	if d.FilterOrder() != defaultDeEsserFilterOrder {
		t.Errorf("FilterOrder() = %d, want %d", d.FilterOrder(), defaultDeEsserFilterOrder)
	}
}

// TestDeEsserSetterValidation tests parameter validation for all setters.
func TestDeEsserSetterValidation(t *testing.T) {
	d, err := NewDeEsser(48000)
	if err != nil {
		t.Fatalf("NewDeEsser() error = %v", err)
	}

	tests := []struct {
		name string
		fn   func() error
	}{
		// Frequency.
		{"freq below min", func() error { return d.SetFrequency(999) }},
		{"freq above max", func() error { return d.SetFrequency(20001) }},
		{"freq NaN", func() error { return d.SetFrequency(math.NaN()) }},
		{"freq Inf", func() error { return d.SetFrequency(math.Inf(1)) }},
		{"freq above Nyquist", func() error { return d.SetFrequency(24001) }},

		// Q.
		{"Q below min", func() error { return d.SetQ(0.09) }},
		{"Q above max", func() error { return d.SetQ(10.1) }},
		{"Q NaN", func() error { return d.SetQ(math.NaN()) }},
		{"Q zero", func() error { return d.SetQ(0) }},

		// Threshold.
		{"threshold NaN", func() error { return d.SetThreshold(math.NaN()) }},
		{"threshold Inf", func() error { return d.SetThreshold(math.Inf(1)) }},

		// Ratio.
		{"ratio below min", func() error { return d.SetRatio(0.5) }},
		{"ratio above max", func() error { return d.SetRatio(101) }},
		{"ratio NaN", func() error { return d.SetRatio(math.NaN()) }},

		// Knee.
		{"knee below min", func() error { return d.SetKnee(-1) }},
		{"knee above max", func() error { return d.SetKnee(13) }},
		{"knee NaN", func() error { return d.SetKnee(math.NaN()) }},

		// Attack.
		{"attack below min", func() error { return d.SetAttack(0.009) }},
		{"attack above max", func() error { return d.SetAttack(51) }},
		{"attack NaN", func() error { return d.SetAttack(math.NaN()) }},

		// Release.
		{"release below min", func() error { return d.SetRelease(0.5) }},
		{"release above max", func() error { return d.SetRelease(501) }},
		{"release NaN", func() error { return d.SetRelease(math.NaN()) }},

		// Range.
		{"range below min", func() error { return d.SetRange(-61) }},
		{"range above max", func() error { return d.SetRange(1) }},
		{"range NaN", func() error { return d.SetRange(math.NaN()) }},

		// Mode.
		{"mode invalid negative", func() error { return d.SetMode(DeEsserMode(-1)) }},
		{"mode invalid high", func() error { return d.SetMode(DeEsserMode(5)) }},

		// Detector.
		{"detector invalid negative", func() error { return d.SetDetector(DeEsserDetector(-1)) }},
		{"detector invalid high", func() error { return d.SetDetector(DeEsserDetector(5)) }},

		// Filter order.
		{"order below min", func() error { return d.SetFilterOrder(0) }},
		{"order above max", func() error { return d.SetFilterOrder(5) }},

		// Sample rate.
		{"sample rate zero", func() error { return d.SetSampleRate(0) }},
		{"sample rate negative", func() error { return d.SetSampleRate(-1) }},
		{"sample rate NaN", func() error { return d.SetSampleRate(math.NaN()) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}

// TestDeEsserSettersUpdate verifies that valid setter calls update state.
func TestDeEsserSettersUpdate(t *testing.T) {
	d, err := NewDeEsser(48000)
	if err != nil {
		t.Fatalf("NewDeEsser() error = %v", err)
	}

	if err := d.SetFrequency(8000); err != nil {
		t.Fatalf("SetFrequency() error = %v", err)
	}

	if d.Frequency() != 8000 {
		t.Errorf("Frequency() = %g, want 8000", d.Frequency())
	}

	if err := d.SetQ(2.0); err != nil {
		t.Fatalf("SetQ() error = %v", err)
	}

	if d.Q() != 2.0 {
		t.Errorf("Q() = %g, want 2", d.Q())
	}

	if err := d.SetThreshold(-30); err != nil {
		t.Fatalf("SetThreshold() error = %v", err)
	}

	if d.Threshold() != -30 {
		t.Errorf("Threshold() = %g, want -30", d.Threshold())
	}

	if err := d.SetRatio(8); err != nil {
		t.Fatalf("SetRatio() error = %v", err)
	}

	if d.Ratio() != 8 {
		t.Errorf("Ratio() = %g, want 8", d.Ratio())
	}

	if err := d.SetKnee(6); err != nil {
		t.Fatalf("SetKnee() error = %v", err)
	}

	if d.Knee() != 6 {
		t.Errorf("Knee() = %g, want 6", d.Knee())
	}

	if err := d.SetAttack(1); err != nil {
		t.Fatalf("SetAttack() error = %v", err)
	}

	if d.Attack() != 1 {
		t.Errorf("Attack() = %g, want 1", d.Attack())
	}

	if err := d.SetRelease(50); err != nil {
		t.Fatalf("SetRelease() error = %v", err)
	}

	if d.Release() != 50 {
		t.Errorf("Release() = %g, want 50", d.Release())
	}

	if err := d.SetRange(-40); err != nil {
		t.Fatalf("SetRange() error = %v", err)
	}

	if d.Range() != -40 {
		t.Errorf("Range() = %g, want -40", d.Range())
	}

	if err := d.SetMode(DeEsserWideband); err != nil {
		t.Fatalf("SetMode() error = %v", err)
	}

	if d.Mode() != DeEsserWideband {
		t.Errorf("Mode() = %d, want %d", d.Mode(), DeEsserWideband)
	}

	if err := d.SetDetector(DeEsserDetectHighpass); err != nil {
		t.Fatalf("SetDetector() error = %v", err)
	}

	if d.Detector() != DeEsserDetectHighpass {
		t.Errorf("Detector() = %d, want %d", d.Detector(), DeEsserDetectHighpass)
	}

	d.SetListen(true)

	if !d.Listen() {
		t.Error("Listen() = false, want true")
	}

	if err := d.SetFilterOrder(3); err != nil {
		t.Fatalf("SetFilterOrder() error = %v", err)
	}

	if d.FilterOrder() != 3 {
		t.Errorf("FilterOrder() = %d, want 3", d.FilterOrder())
	}

	if err := d.SetSampleRate(96000); err != nil {
		t.Fatalf("SetSampleRate() error = %v", err)
	}

	if d.SampleRate() != 96000 {
		t.Errorf("SampleRate() = %g, want 96000", d.SampleRate())
	}
}

// TestDeEsserSilenceProducesSilence verifies that silent input produces silent output.
func TestDeEsserSilenceProducesSilence(t *testing.T) {
	d, err := NewDeEsser(48000)
	if err != nil {
		t.Fatalf("NewDeEsser() error = %v", err)
	}

	for i := range 1024 {
		out := d.ProcessSample(0)
		if out != 0 {
			t.Fatalf("sample %d: silent input should produce 0, got %g", i, out)
		}
	}
}

// TestDeEsserLowFrequencyTransparent verifies that a low-frequency signal
// well below the detection band passes through without significant alteration.
func TestDeEsserLowFrequencyTransparent(t *testing.T) {
	d, err := NewDeEsser(48000)
	if err != nil {
		t.Fatalf("NewDeEsser() error = %v", err)
	}
	// Set threshold very low to be sensitive — but the low-frequency signal
	// should still not trigger reduction because the detection filter
	// rejects it.
	if err := d.SetThreshold(-60); err != nil {
		t.Fatalf("SetThreshold() error = %v", err)
	}

	// Generate a 200 Hz signal (well below 6kHz detection).
	const (
		freq = 200.0
		sr   = 48000.0
		n    = 4096
	)

	// Let the filter settle.

	for i := range n {
		sample := 0.5 * math.Sin(2*math.Pi*freq*float64(i)/sr)
		d.ProcessSample(sample)
	}

	// Now measure — output should be close to input.
	maxDiff := 0.0

	for i := range n {
		sample := 0.5 * math.Sin(2*math.Pi*freq*float64(n+i)/sr)
		out := d.ProcessSample(sample)

		diff := math.Abs(out - sample)
		if diff > maxDiff {
			maxDiff = diff
		}
	}

	// Allow small tolerance for filter transients.
	if maxDiff > 0.05 {
		t.Errorf("low-frequency signal altered too much: max diff = %g", maxDiff)
	}
}

// TestDeEsserReducesSibilance verifies that a high-frequency signal
// in the sibilance band is attenuated.
func TestDeEsserReducesSibilance(t *testing.T) {
	d, err := NewDeEsser(48000)
	if err != nil {
		t.Fatalf("NewDeEsser() error = %v", err)
	}
	// Very sensitive threshold, aggressive ratio.
	if err := d.SetThreshold(-40); err != nil {
		t.Fatalf("SetThreshold() error = %v", err)
	}

	if err := d.SetRatio(20); err != nil {
		t.Fatalf("SetRatio() error = %v", err)
	}

	// Generate a 6000 Hz sine (right at detection center).
	const (
		freq      = 6000.0
		sr        = 48000.0
		n         = 2048
		amplitude = 0.5
	)

	// Let the detector envelope settle.

	for i := range n {
		sample := amplitude * math.Sin(2*math.Pi*freq*float64(i)/sr)
		d.ProcessSample(sample)
	}

	// Measure output level.
	peakOut := 0.0

	for i := range n {
		sample := amplitude * math.Sin(2*math.Pi*freq*float64(n+i)/sr)

		out := d.ProcessSample(sample)
		if math.Abs(out) > peakOut {
			peakOut = math.Abs(out)
		}
	}

	// Output peak should be noticeably reduced.
	if peakOut >= amplitude*0.9 {
		t.Errorf("sibilant signal not sufficiently reduced: peak out = %g (input amplitude = %g)",
			peakOut, amplitude)
	}
}

// TestDeEsserWidebandMode verifies wideband mode reduces the full signal.
func TestDeEsserWidebandMode(t *testing.T) {
	d, err := NewDeEsser(48000)
	if err != nil {
		t.Fatalf("NewDeEsser() error = %v", err)
	}

	if err := d.SetMode(DeEsserWideband); err != nil {
		t.Fatalf("SetMode() error = %v", err)
	}

	if err := d.SetThreshold(-40); err != nil {
		t.Fatalf("SetThreshold() error = %v", err)
	}

	if err := d.SetRatio(20); err != nil {
		t.Fatalf("SetRatio() error = %v", err)
	}

	// Feed sibilant signal to trigger detection.
	const (
		freq = 6000.0
		sr   = 48000.0
		n    = 2048
		amp  = 0.5
	)

	for i := range n {
		sample := amp * math.Sin(2*math.Pi*freq*float64(i)/sr)
		d.ProcessSample(sample)
	}

	// Now feed a lower frequency and check it's also reduced (wideband effect).
	peakOut := 0.0

	for i := range n {
		// Mix of low and high frequency — wideband should reduce both.
		t := float64(n + i)
		sample := amp*math.Sin(2*math.Pi*freq*t/sr) + 0.3*math.Sin(2*math.Pi*300*t/sr)

		out := d.ProcessSample(sample)
		if math.Abs(out) > peakOut {
			peakOut = math.Abs(out)
		}
	}

	// The combined signal should be reduced.
	inputPeak := amp + 0.3
	if peakOut >= inputPeak*0.9 {
		t.Errorf("wideband de-esser did not reduce signal: peak = %g (input peak = %g)",
			peakOut, inputPeak)
	}
}

// TestDeEsserListenMode verifies listen mode outputs the detection band.
func TestDeEsserListenMode(t *testing.T) {
	d, err := NewDeEsser(48000)
	if err != nil {
		t.Fatalf("NewDeEsser() error = %v", err)
	}

	d.SetListen(true)

	// Feed a low-frequency signal — listen output should be near zero
	// since bandpass at 6kHz rejects 200Hz.
	const (
		sr = 48000.0
		n  = 2048
	)

	for i := range n {
		d.ProcessSample(0.5 * math.Sin(2*math.Pi*200*float64(i)/sr))
	}

	peakLow := 0.0

	for i := range n {
		out := d.ProcessSample(0.5 * math.Sin(2*math.Pi*200*float64(n+i)/sr))
		if math.Abs(out) > peakLow {
			peakLow = math.Abs(out)
		}
	}

	// Feed a signal at the detection frequency — listen output should be significant.
	d.Reset()

	for i := range n {
		d.ProcessSample(0.5 * math.Sin(2*math.Pi*6000*float64(i)/sr))
	}

	peakHigh := 0.0

	for i := range n {
		out := d.ProcessSample(0.5 * math.Sin(2*math.Pi*6000*float64(n+i)/sr))
		if math.Abs(out) > peakHigh {
			peakHigh = math.Abs(out)
		}
	}

	if peakLow >= peakHigh {
		t.Errorf("listen mode should pass sibilance band: low peak = %g, high peak = %g",
			peakLow, peakHigh)
	}

	if peakHigh < 0.01 {
		t.Errorf("listen mode produced near-zero output for detection band signal: %g", peakHigh)
	}
}

// TestDeEsserHighpassDetector verifies highpass detection mode works.
func TestDeEsserHighpassDetector(t *testing.T) {
	d, err := NewDeEsser(48000)
	if err != nil {
		t.Fatalf("NewDeEsser() error = %v", err)
	}

	if err := d.SetDetector(DeEsserDetectHighpass); err != nil {
		t.Fatalf("SetDetector() error = %v", err)
	}

	if err := d.SetThreshold(-40); err != nil {
		t.Fatalf("SetThreshold() error = %v", err)
	}

	if err := d.SetRatio(10); err != nil {
		t.Fatalf("SetRatio() error = %v", err)
	}

	// A signal above the detection frequency should trigger reduction.
	const (
		freq = 8000.0
		sr   = 48000.0
		n    = 2048
		amp  = 0.5
	)

	for i := range n {
		d.ProcessSample(amp * math.Sin(2*math.Pi*freq*float64(i)/sr))
	}

	peakOut := 0.0

	for i := range n {
		out := d.ProcessSample(amp * math.Sin(2*math.Pi*freq*float64(n+i)/sr))
		if math.Abs(out) > peakOut {
			peakOut = math.Abs(out)
		}
	}

	if peakOut >= amp*0.9 {
		t.Errorf("highpass detector did not reduce sibilance: peak = %g", peakOut)
	}
}

// TestDeEsserProcessInPlaceMatchesSample verifies buffer and sample processing agree.
func TestDeEsserProcessInPlaceMatchesSample(t *testing.T) {
	d1, err := NewDeEsser(48000)
	if err != nil {
		t.Fatalf("NewDeEsser() error = %v", err)
	}

	d2, err := NewDeEsser(48000)
	if err != nil {
		t.Fatalf("NewDeEsser() error = %v", err)
	}

	input := make([]float64, 256)
	for i := range input {
		input[i] = 0.4 * math.Sin(2*math.Pi*6000*float64(i)/48000)
	}

	want := make([]float64, len(input))
	copy(want, input)

	for i := range want {
		want[i] = d1.ProcessSample(want[i])
	}

	got := make([]float64, len(input))
	copy(got, input)
	d2.ProcessInPlace(got)

	for i := range got {
		if diff := math.Abs(got[i] - want[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g diff=%g", i, got[i], want[i], diff)
		}
	}
}

// TestDeEsserResetRestoresState verifies that Reset produces deterministic output.
func TestDeEsserResetRestoresState(t *testing.T) {
	d, err := NewDeEsser(48000)
	if err != nil {
		t.Fatalf("NewDeEsser() error = %v", err)
	}

	input := make([]float64, 512)
	for i := range input {
		input[i] = 0.5 * math.Sin(2*math.Pi*6000*float64(i)/48000)
	}

	out1 := make([]float64, len(input))
	for i, v := range input {
		out1[i] = d.ProcessSample(v)
	}

	d.Reset()

	out2 := make([]float64, len(input))
	for i, v := range input {
		out2[i] = d.ProcessSample(v)
	}

	for i := range out1 {
		if diff := math.Abs(out1[i] - out2[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch after reset: got=%g want=%g diff=%g",
				i, out2[i], out1[i], diff)
		}
	}
}

// TestDeEsserMetrics verifies metering captures expected data.
func TestDeEsserMetrics(t *testing.T) {
	d, err := NewDeEsser(48000)
	if err != nil {
		t.Fatalf("NewDeEsser() error = %v", err)
	}

	if err := d.SetThreshold(-30); err != nil {
		t.Fatalf("SetThreshold() error = %v", err)
	}

	// Process a loud sibilant signal.
	for i := range 2048 {
		d.ProcessSample(0.8 * math.Sin(2*math.Pi*6000*float64(i)/48000))
	}

	m := d.GetMetrics()
	if m.DetectionLevel <= 0 {
		t.Error("DetectionLevel should be > 0 after processing sibilance")
	}

	if m.GainReduction >= 1.0 {
		t.Error("GainReduction should be < 1.0 after processing sibilance above threshold")
	}

	d.ResetMetrics()

	m2 := d.GetMetrics()
	if m2.DetectionLevel != 0 {
		t.Error("DetectionLevel should be 0 after ResetMetrics")
	}

	if m2.GainReduction != 1.0 {
		t.Errorf("GainReduction should be 1.0 after ResetMetrics, got %g", m2.GainReduction)
	}
}

// TestDeEsserHardKnee verifies behavior with 0 dB knee.
func TestDeEsserHardKnee(t *testing.T) {
	d, err := NewDeEsser(48000)
	if err != nil {
		t.Fatalf("NewDeEsser() error = %v", err)
	}

	if err := d.SetKnee(0); err != nil {
		t.Fatalf("SetKnee() error = %v", err)
	}

	if err := d.SetThreshold(-30); err != nil {
		t.Fatalf("SetThreshold() error = %v", err)
	}

	// Process should still work correctly.
	for i := range 1024 {
		d.ProcessSample(0.5 * math.Sin(2*math.Pi*6000*float64(i)/48000))
	}

	m := d.GetMetrics()
	if m.GainReduction >= 1.0 {
		t.Error("expected gain reduction with hard knee on sibilant signal")
	}
}

// TestDeEsserFilterOrderEffect verifies that higher filter orders produce
// steeper detection slopes.
func TestDeEsserFilterOrderEffect(t *testing.T) {
	// With order 1, a signal somewhat off-center should still have some
	// detection energy. With order 4, it should have much less.
	const (
		sr = 48000.0
		n  = 4096
	)

	const offFreq = 3000.0 // Significantly below 6kHz center

	measureDetection := func(order int) float64 {
		d, err := NewDeEsser(sr)
		if err != nil {
			t.Fatalf("NewDeEsser() error = %v", err)
		}

		if err := d.SetFilterOrder(order); err != nil {
			t.Fatalf("SetFilterOrder() error = %v", err)
		}

		d.SetListen(true)

		// Let filter settle.
		for i := range n {
			d.ProcessSample(0.5 * math.Sin(2*math.Pi*offFreq*float64(i)/sr))
		}

		peak := 0.0

		for i := range n {
			out := d.ProcessSample(0.5 * math.Sin(2*math.Pi*offFreq*float64(n+i)/sr))
			if math.Abs(out) > peak {
				peak = math.Abs(out)
			}
		}

		return peak
	}

	peak1 := measureDetection(1)
	peak4 := measureDetection(4)

	if peak4 >= peak1 {
		t.Errorf("higher filter order should reduce off-band detection: order1=%g order4=%g",
			peak1, peak4)
	}
}

// TestDeEsserRangeLimit verifies that gain reduction is bounded by the range parameter.
func TestDeEsserRangeLimit(t *testing.T) {
	d, err := NewDeEsser(48000)
	if err != nil {
		t.Fatalf("NewDeEsser() error = %v", err)
	}

	// Very aggressive settings.
	if err := d.SetThreshold(-60); err != nil {
		t.Fatalf("SetThreshold() error = %v", err)
	}

	if err := d.SetRatio(100); err != nil {
		t.Fatalf("SetRatio() error = %v", err)
	}

	if err := d.SetRange(-12); err != nil {
		t.Fatalf("SetRange() error = %v", err)
	}

	if err := d.SetMode(DeEsserWideband); err != nil {
		t.Fatalf("SetMode() error = %v", err)
	}

	if err := d.SetKnee(0); err != nil {
		t.Fatalf("SetKnee() error = %v", err)
	}

	// Process a loud sibilant signal until envelope settles.
	const (
		freq = 6000.0
		sr   = 48000.0
		amp  = 0.9
	)

	for i := range 8192 {
		d.ProcessSample(amp * math.Sin(2*math.Pi*freq*float64(i)/sr))
	}

	m := d.GetMetrics()
	// Range is -12 dB, which is linear ~0.251.
	rangeLin := math.Pow(10, -12.0/20.0)
	if m.GainReduction < rangeLin*0.95 {
		t.Errorf("gain reduction %.4f exceeded range limit %.4f", m.GainReduction, rangeLin)
	}
}

// TestDeEsserRatioOneIsTransparent verifies that ratio 1:1 means no reduction.
func TestDeEsserRatioOneIsTransparent(t *testing.T) {
	d, err := NewDeEsser(48000)
	if err != nil {
		t.Fatalf("NewDeEsser() error = %v", err)
	}

	if err := d.SetRatio(1); err != nil {
		t.Fatalf("SetRatio() error = %v", err)
	}

	if err := d.SetMode(DeEsserWideband); err != nil {
		t.Fatalf("SetMode() error = %v", err)
	}

	input := make([]float64, 1024)
	for i := range input {
		input[i] = 0.5 * math.Sin(2*math.Pi*6000*float64(i)/48000)
	}

	for i, v := range input {
		out := d.ProcessSample(v)
		if diff := math.Abs(out - v); diff > 1e-10 {
			t.Fatalf("sample %d: ratio 1 should be transparent, diff=%g", i, diff)
		}
	}
}

// BenchmarkDeEsserProcessSample benchmarks single-sample processing.
func BenchmarkDeEsserProcessSample(b *testing.B) {
	d, err := NewDeEsser(48000)
	if err != nil {
		b.Fatalf("NewDeEsser() error = %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := range b.N {
		d.ProcessSample(0.5 * math.Sin(2*math.Pi*6000*float64(i)/48000))
	}
}

// BenchmarkDeEsserProcessInPlace benchmarks buffer processing.
func BenchmarkDeEsserProcessInPlace(b *testing.B) {
	d, err := NewDeEsser(48000)
	if err != nil {
		b.Fatalf("NewDeEsser() error = %v", err)
	}

	buf := make([]float64, 1024)
	for i := range buf {
		buf[i] = 0.5 * math.Sin(2*math.Pi*6000*float64(i)/48000)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		d.ProcessInPlace(buf)
	}
}
