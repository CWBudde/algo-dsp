package effects

import (
	"math"
	"testing"
)

// TestDelay_SetTargetTime_RampsGradually verifies that calling SetTargetTime
// causes the effective delay to change incrementally, not in a single jump.
// A jump would manifest as a sudden large change in the read-pointer position,
// which is audible as a click.
func TestDelay_SetTargetTime_RampsGradually(t *testing.T) {
	const sampleRate = 1000.0

	d, err := NewDelay(sampleRate)
	if err != nil {
		t.Fatalf("NewDelay: %v", err)
	}

	// Start with a long delay (250ms = 250 samples) and snap it.
	if err := d.SetTime(0.25); err != nil {
		t.Fatalf("SetTime: %v", err)
	}

	startSamples := d.CurrentDelaySamples()

	// Now request a short delay (10ms = 10 samples) via the smooth setter.
	if err := d.SetTargetTime(0.01); err != nil {
		t.Fatalf("SetTargetTime: %v", err)
	}

	// Process a few samples to trigger the ramp.
	buf := make([]float64, 10)
	d.ProcessInPlace(buf)

	current := d.CurrentDelaySamples()

	// The current delay must have moved toward the target but not reached it.
	if current >= startSamples {
		t.Errorf("delay did not ramp: current=%v, start=%v", current, startSamples)
	}

	targetSamples := 0.01 * sampleRate // 10 samples
	if current <= targetSamples {
		t.Errorf("delay overshot or reached target too fast: current=%v, target=%v", current, targetSamples)
	}
}

// TestDelay_SetTargetTime_ConvergesToTarget verifies that after processing
// enough samples the effective delay eventually equals the requested target.
func TestDelay_SetTargetTime_ConvergesToTarget(t *testing.T) {
	const sampleRate = 1000.0

	d, err := NewDelay(sampleRate)
	if err != nil {
		t.Fatalf("NewDelay: %v", err)
	}

	if err := d.SetTime(0.25); err != nil {
		t.Fatalf("SetTime: %v", err)
	}

	targetSeconds := 0.01
	if err := d.SetTargetTime(targetSeconds); err != nil {
		t.Fatalf("SetTargetTime: %v", err)
	}

	// Process enough samples for the smoother to converge (5τ with 10ms τ).
	buf := make([]float64, 500)
	d.ProcessInPlace(buf)

	want := math.Round(targetSeconds * sampleRate)
	got := d.CurrentDelaySamples()

	if math.Abs(got-want) > 0.5 {
		t.Errorf("did not converge: got=%v, want=%v", got, want)
	}
}

// TestDelay_SetTime_SnapsImmediately verifies that SetTime (the existing API)
// still snaps the current delay to the target without ramping — preserving
// backward compatibility for static configuration before playback starts.
func TestDelay_SetTime_SnapsImmediately(t *testing.T) {
	const sampleRate = 1000.0

	d, err := NewDelay(sampleRate)
	if err != nil {
		t.Fatalf("NewDelay: %v", err)
	}

	if err := d.SetTime(0.25); err != nil {
		t.Fatalf("SetTime: %v", err)
	}

	if err := d.SetTime(0.01); err != nil {
		t.Fatalf("SetTime 0.01: %v", err)
	}

	want := math.Round(0.01 * sampleRate)
	got := d.CurrentDelaySamples()

	if math.Abs(got-want) > 1e-9 {
		t.Errorf("SetTime did not snap: got=%v, want=%v", got, want)
	}
}

func TestDelayProcessInPlaceMatchesSample(t *testing.T) {
	d1, err := NewDelay(48000)
	if err != nil {
		t.Fatalf("NewDelay() error = %v", err)
	}

	d2, err := NewDelay(48000)
	if err != nil {
		t.Fatalf("NewDelay() error = %v", err)
	}

	input := make([]float64, 128)
	for i := range input {
		input[i] = math.Sin(2 * math.Pi * float64(i) / 29)
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

func TestDelayResetRestoresState(t *testing.T) {
	d, err := NewDelay(48000)
	if err != nil {
		t.Fatalf("NewDelay() error = %v", err)
	}

	in := make([]float64, 96)
	in[0] = 1

	out1 := make([]float64, len(in))
	for i := range in {
		out1[i] = d.ProcessSample(in[i])
	}

	d.Reset()

	out2 := make([]float64, len(in))
	for i := range in {
		out2[i] = d.ProcessSample(in[i])
	}

	for i := range out1 {
		if diff := math.Abs(out1[i] - out2[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch after reset: got=%g want=%g diff=%g", i, out2[i], out1[i], diff)
		}
	}
}

func TestDelayImpulseAtConfiguredTime(t *testing.T) {
	const sampleRate = 1000.0

	d, err := NewDelay(sampleRate)
	if err != nil {
		t.Fatalf("NewDelay() error = %v", err)
	}

	if err := d.SetTime(0.01); err != nil {
		t.Fatalf("SetTime() error = %v", err)
	}

	if err := d.SetMix(1); err != nil {
		t.Fatalf("SetMix() error = %v", err)
	}

	if err := d.SetFeedback(0); err != nil {
		t.Fatalf("SetFeedback() error = %v", err)
	}

	in := make([]float64, 20)
	in[0] = 1

	out := make([]float64, len(in))
	for i := range in {
		out[i] = d.ProcessSample(in[i])
	}

	for i := range out {
		want := 0.0
		if i == 10 {
			want = 1
		}

		if diff := math.Abs(out[i] - want); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g", i, out[i], want)
		}
	}
}
