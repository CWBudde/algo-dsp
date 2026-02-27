package dynamics

import (
	"math"
	"testing"
)

func TestNewLookaheadLimiter(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate float64
		wantErr    bool
	}{
		{"valid", 48000, false},
		{"zero", 0, true},
		{"negative", -1, true},
		{"nan", math.NaN(), true},
		{"inf", math.Inf(1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := NewLookaheadLimiter(tt.sampleRate)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewLookaheadLimiter() err=%v wantErr=%v", err, tt.wantErr)
			}

			if !tt.wantErr && l == nil {
				t.Fatal("NewLookaheadLimiter() returned nil without error")
			}
		})
	}
}

func TestLookaheadLimiterParameterValidation(t *testing.T) {
	l, _ := NewLookaheadLimiter(48000)

	err := l.SetThreshold(-30)
	if err == nil {
		t.Fatal("expected threshold validation error")
	}

	err = l.SetRelease(0.5)
	if err == nil {
		t.Fatal("expected release validation error")
	}

	err = l.SetLookahead(-1)
	if err == nil {
		t.Fatal("expected lookahead validation error")
	}
}

func TestLookaheadLimiterDelayBehavior(t *testing.T) {
	l, err := NewLookaheadLimiter(1000)
	if err != nil {
		t.Fatal(err)
	}

	_ = l.SetThreshold(0)
	_ = l.SetRelease(50)
	_ = l.SetLookahead(2) // 2 samples at 1 kHz.

	in := make([]float64, 8)
	in[0] = 1.0

	out := make([]float64, len(in))
	for i := range in {
		out[i] = l.ProcessSample(in[i])
	}

	if math.Abs(out[0]) > 1e-12 || math.Abs(out[1]) > 1e-12 {
		t.Fatalf("expected initial silence from lookahead delay, got [%f %f]", out[0], out[1])
	}

	if out[2] <= 0 {
		t.Fatalf("expected delayed impulse at index 2, got %f", out[2])
	}
}

func TestLookaheadLimiterProcessInPlaceMatchesSamplePath(t *testing.T) {
	l1, _ := NewLookaheadLimiter(48000)
	l2, _ := NewLookaheadLimiter(48000)

	_ = l1.SetThreshold(-3)
	_ = l1.SetRelease(80)
	_ = l1.SetLookahead(3)
	_ = l2.SetThreshold(-3)
	_ = l2.SetRelease(80)
	_ = l2.SetLookahead(3)

	in := make([]float64, 256)
	for i := range in {
		in[i] = 0.9 * math.Sin(2*math.Pi*440*float64(i)/48000)
	}

	want := make([]float64, len(in))
	for i := range in {
		want[i] = l1.ProcessSample(in[i])
	}

	got := append([]float64(nil), in...)
	l2.ProcessInPlace(got)

	for i := range got {
		if diff := math.Abs(got[i] - want[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g diff=%g", i, got[i], want[i], diff)
		}
	}
}

func TestLookaheadLimiterSidechainDrivesGain(t *testing.T) {
	lNeutral, _ := NewLookaheadLimiter(48000)
	lSide, _ := NewLookaheadLimiter(48000)
	_ = lNeutral.SetThreshold(-12)
	_ = lSide.SetThreshold(-12)
	_ = lNeutral.SetLookahead(0)
	_ = lSide.SetLookahead(0)

	// Program is low level (would not be limited on its own).
	program := 0.08
	sidechainHot := 1.0

	// Warm up detector.
	for range 200 {
		lNeutral.ProcessSample(program)
		lSide.ProcessSampleSidechain(program, sidechainHot)
	}

	noSide := lNeutral.ProcessSample(program)
	withSide := lSide.ProcessSampleSidechain(program, sidechainHot)

	if withSide >= noSide {
		t.Fatalf("expected sidechain to reduce output: noSide=%f withSide=%f", noSide, withSide)
	}
}

func TestLookaheadLimiterResetRestoresDeterministicState(t *testing.T) {
	l, _ := NewLookaheadLimiter(48000)
	_ = l.SetThreshold(-6)
	_ = l.SetLookahead(4)

	in := make([]float64, 128)
	for i := range in {
		in[i] = 0.7 * math.Sin(2*math.Pi*220*float64(i)/48000)
	}

	out1 := make([]float64, len(in))
	for i := range in {
		out1[i] = l.ProcessSample(in[i])
	}

	l.Reset()

	out2 := make([]float64, len(in))
	for i := range in {
		out2[i] = l.ProcessSample(in[i])
	}

	for i := range out1 {
		if diff := math.Abs(out1[i] - out2[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch after reset: got=%g want=%g diff=%g", i, out2[i], out1[i], diff)
		}
	}
}
