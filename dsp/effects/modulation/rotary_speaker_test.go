package modulation

import (
	"math"
	"testing"
)

func TestNewRotarySpeaker(t *testing.T) {
	r, err := NewRotarySpeaker()
	if err != nil {
		t.Fatalf("NewRotarySpeaker() error = %v", err)
	}

	if r == nil {
		t.Fatal("NewRotarySpeaker() returned nil")
	}
}

func TestRotarySpeakerSettersValidation(t *testing.T) {
	r, err := NewRotarySpeaker()
	if err != nil {
		t.Fatalf("NewRotarySpeaker() error = %v", err)
	}

	tests := []struct {
		name string
		fn   func() error
	}{
		{"sample rate too low", func() error { return r.SetSampleRate(4000) }},
		{"mix below zero", func() error { return r.SetMix(-0.1) }},
		{"mix above one", func() error { return r.SetMix(1.1) }},
		{"stereo width below zero", func() error { return r.SetStereoWidth(-0.1) }},
		{"stereo width above one", func() error { return r.SetStereoWidth(1.1) }},
		{"drive zero", func() error { return r.SetDrive(0) }},
		{"crossover too low", func() error { return r.SetCrossoverHz(10) }},
		{"horn chorale speed negative", func() error { return r.SetHornSpeed(-1, 5) }},
		{"drum tremolo speed too high", func() error { return r.SetDrumSpeed(0.5, 30) }},
		{"horn accel tau zero", func() error { return r.SetHornTimeConstants(0, 1) }},
		{"drum decel tau too high", func() error { return r.SetDrumTimeConstants(0.5, 100) }},
		{"horn radius negative", func() error { return r.SetGeometry(-1, 0.1, 0.3) }},
		{"invalid speed mode", func() error { return r.SetSpeedMode(SpeedMode(123)) }},
	}

	for _, tc := range tests {
		err := tc.fn()
		if err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}

func TestRotarySpeakerResetRestoresState(t *testing.T) {
	r, err := NewRotarySpeaker()
	if err != nil {
		t.Fatalf("NewRotarySpeaker() error = %v", err)
	}

	// Warm up the rotor state.
	for i := range 128 {
		r.ProcessSample(math.Sin(2 * math.Pi * 440 * float64(i) / r.sampleRate))
	}

	l1, r1 := r.ProcessSample(0.25)

	r.Reset()
	l2, r2 := r.ProcessSample(0.25)

	r.Reset()
	l3, r3 := r.ProcessSample(0.25)

	if almostEqualRotary(l1, l2) && almostEqualRotary(r1, r2) {
		t.Fatal("expected Reset() to change evolved state")
	}

	if !almostEqualRotary(l2, l3) || !almostEqualRotary(r2, r3) {
		t.Fatal("Reset() did not restore deterministic initial state")
	}
}

func TestRotarySpeakerDeterministic(t *testing.T) {
	r1, err := NewRotarySpeaker()
	if err != nil {
		t.Fatalf("NewRotarySpeaker() error = %v", err)
	}

	r2, err := NewRotarySpeaker()
	if err != nil {
		t.Fatalf("NewRotarySpeaker() error = %v", err)
	}

	for i := range 2048 {
		x := math.Sin(2 * math.Pi * 220 * float64(i) / 44100.0)
		l1, rr1 := r1.ProcessSample(x)

		l2, rr2 := r2.ProcessSample(x)
		if !almostEqualRotary(l1, l2) || !almostEqualRotary(rr1, rr2) {
			t.Fatalf("non-deterministic output at sample %d", i)
		}
	}
}

func TestRotarySpeakerStereoWidth(t *testing.T) {
	r, err := NewRotarySpeaker()
	if err != nil {
		t.Fatalf("NewRotarySpeaker() error = %v", err)
	}

	err = r.SetStereoWidth(0)
	if err != nil {
		t.Fatalf("SetStereoWidth(0) error = %v", err)
	}

	var maxDiffMono float64

	for i := range 2048 {
		x := math.Sin(2 * math.Pi * 330 * float64(i) / 44100.0)
		l, rr := r.ProcessSample(x)

		diff := math.Abs(l - rr)
		if diff > maxDiffMono {
			maxDiffMono = diff
		}
	}

	r.Reset()

	err = r.SetStereoWidth(1)
	if err != nil {
		t.Fatalf("SetStereoWidth(1) error = %v", err)
	}

	var maxDiffStereo float64

	for i := range 2048 {
		x := math.Sin(2 * math.Pi * 330 * float64(i) / 44100.0)
		l, rr := r.ProcessSample(x)

		diff := math.Abs(l - rr)
		if diff > maxDiffStereo {
			maxDiffStereo = diff
		}
	}

	if maxDiffStereo <= maxDiffMono {
		t.Fatalf("expected stereo width to increase L/R difference: mono=%g stereo=%g", maxDiffMono, maxDiffStereo)
	}
}

func TestRotarySpeakerOutputIsFinite(t *testing.T) {
	r, err := NewRotarySpeaker()
	if err != nil {
		t.Fatalf("NewRotarySpeaker() error = %v", err)
	}

	for i := range 4096 {
		x := 0.9 * math.Sin(2*math.Pi*110*float64(i)/44100.0)

		l, rr := r.ProcessSample(x)
		if math.IsNaN(l) || math.IsInf(l, 0) || math.IsNaN(rr) || math.IsInf(rr, 0) {
			t.Fatalf("non-finite output at sample %d: L=%v R=%v", i, l, rr)
		}
	}
}

func TestRotarySpeakerSpeedTransitionChangesRotorVelocity(t *testing.T) {
	r, err := NewRotarySpeaker()
	if err != nil {
		t.Fatalf("NewRotarySpeaker() error = %v", err)
	}

	err = r.SetSpeedMode(SpeedModeChorale)
	if err != nil {
		t.Fatalf("SetSpeedMode(Chorale) error = %v", err)
	}

	for range 512 {
		r.ProcessSample(0)
	}

	omegaSlow := math.Abs(r.horn.omega)

	err = r.SetSpeedMode(SpeedModeTremolo)
	if err != nil {
		t.Fatalf("SetSpeedMode(Tremolo) error = %v", err)
	}

	for range 4096 {
		r.ProcessSample(0)
	}

	omegaFast := math.Abs(r.horn.omega)

	if omegaFast <= omegaSlow {
		t.Fatalf("expected faster rotor after tremolo transition: slow=%g fast=%g", omegaSlow, omegaFast)
	}
}

func TestRotarySpeakerProcessStereoInPlace(t *testing.T) {
	r, err := NewRotarySpeaker()
	if err != nil {
		t.Fatalf("NewRotarySpeaker() error = %v", err)
	}

	left := make([]float64, 256)
	right := make([]float64, 256)

	for i := range left {
		left[i] = math.Sin(2 * math.Pi * 440 * float64(i) / 44100.0)
		right[i] = math.Sin(2 * math.Pi * 441 * float64(i) / 44100.0)
	}

	r.ProcessStereoInPlace(left, right)

	var changed bool

	for i := range left {
		if left[i] != 0 || right[i] != 0 {
			changed = true
			break
		}
	}

	if !changed {
		t.Fatal("ProcessStereoInPlace() produced all zeros unexpectedly")
	}
}

func TestRotarySpeakerAccessors(t *testing.T) {
	r, err := NewRotarySpeaker()
	if err != nil {
		t.Fatalf("NewRotarySpeaker() error = %v", err)
	}

	if r.SampleRate() != defaultRotarySampleRate {
		t.Errorf("SampleRate() = %g, want %g", r.SampleRate(), defaultRotarySampleRate)
	}

	if r.Mix() != 1.0 {
		t.Errorf("Mix() = %g, want 1.0", r.Mix())
	}

	if r.StereoWidth() != 1.0 {
		t.Errorf("StereoWidth() = %g, want 1.0", r.StereoWidth())
	}

	if r.Drive() != 1.0 {
		t.Errorf("Drive() = %g, want 1.0", r.Drive())
	}
}

func almostEqualRotary(a, b float64) bool {
	return math.Abs(a-b) <= 1e-12
}
