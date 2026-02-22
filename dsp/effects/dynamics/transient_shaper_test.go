package dynamics

import (
	"math"
	"testing"
)

func TestNewTransientShaper(t *testing.T) {
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
			ts, err := NewTransientShaper(tt.sampleRate)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewTransientShaper() err=%v wantErr=%v", err, tt.wantErr)
			}

			if !tt.wantErr && ts == nil {
				t.Fatal("NewTransientShaper() returned nil without error")
			}
		})
	}
}

func TestTransientShaperDefaults(t *testing.T) {
	ts, err := NewTransientShaper(48000)
	if err != nil {
		t.Fatal(err)
	}

	if ts.AttackAmount() != defaultTransientShaperAttackAmount {
		t.Fatalf("attackAmount=%f", ts.AttackAmount())
	}

	if ts.SustainAmount() != defaultTransientShaperSustainAmount {
		t.Fatalf("sustainAmount=%f", ts.SustainAmount())
	}

	if ts.Attack() != defaultTransientShaperAttackMs {
		t.Fatalf("attackMs=%f", ts.Attack())
	}

	if ts.Release() != defaultTransientShaperReleaseMs {
		t.Fatalf("releaseMs=%f", ts.Release())
	}
}

func TestTransientShaperParameterValidation(t *testing.T) {
	ts, _ := NewTransientShaper(48000)

	if err := ts.SetAttackAmount(-1.1); err == nil {
		t.Fatal("expected attack amount error")
	}

	if err := ts.SetSustainAmount(1.1); err == nil {
		t.Fatal("expected sustain amount error")
	}

	if err := ts.SetAttack(0.05); err == nil {
		t.Fatal("expected attack time error")
	}

	if err := ts.SetRelease(0.5); err == nil {
		t.Fatal("expected release time error")
	}
}

func TestTransientShaperProcessInPlaceMatchesSamplePath(t *testing.T) {
	a, _ := NewTransientShaper(48000)
	b, _ := NewTransientShaper(48000)
	_ = a.SetAttackAmount(0.6)
	_ = a.SetSustainAmount(-0.4)
	_ = b.SetAttackAmount(0.6)
	_ = b.SetSustainAmount(-0.4)

	input := make([]float64, 512)
	for i := 0; i < len(input); i++ {
		input[i] = 0.5 * math.Sin(2*math.Pi*220*float64(i)/48000)
	}

	want := make([]float64, len(input))
	copy(want, input)
	for i := range want {
		want[i] = a.ProcessSample(want[i])
	}

	got := make([]float64, len(input))
	copy(got, input)
	b.ProcessInPlace(got)

	for i := range got {
		if diff := math.Abs(got[i] - want[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g diff=%g", i, got[i], want[i], diff)
		}
	}
}

func TestTransientShaperAttackBoostsRisingEdge(t *testing.T) {
	neutral, _ := NewTransientShaper(48000)
	boosted, _ := NewTransientShaper(48000)
	_ = boosted.SetAttackAmount(1.0)

	// Precondition the detector with silence.
	for i := 0; i < 128; i++ {
		neutral.ProcessSample(0)
		boosted.ProcessSample(0)
	}

	in := 1.0
	outNeutral := neutral.ProcessSample(in)
	outBoosted := boosted.ProcessSample(in)

	if outBoosted <= outNeutral {
		t.Fatalf("expected attack boost on rising edge: neutral=%f boosted=%f", outNeutral, outBoosted)
	}
}

func TestTransientShaperNegativeSustainReducesDecayTail(t *testing.T) {
	neutral, _ := NewTransientShaper(48000)
	shaped, _ := NewTransientShaper(48000)
	_ = shaped.SetSustainAmount(-1.0)
	_ = shaped.SetRelease(40)

	var neutralTail, shapedTail float64
	count := 0

	for i := 0; i < 600; i++ {
		in := 0.8
		if i >= 300 {
			in = 0.15
		}

		n := neutral.ProcessSample(in)
		s := shaped.ProcessSample(in)

		if i >= 300 {
			neutralTail += math.Abs(n)
			shapedTail += math.Abs(s)
			count++
		}
	}

	if count == 0 {
		t.Fatal("internal test error: zero tail samples")
	}

	neutralTail /= float64(count)
	shapedTail /= float64(count)

	if shapedTail >= neutralTail {
		t.Fatalf("expected reduced sustain tail: neutral=%f shaped=%f", neutralTail, shapedTail)
	}
}

func TestTransientShaperResetRestoresDeterministicState(t *testing.T) {
	ts, _ := NewTransientShaper(48000)
	_ = ts.SetAttackAmount(0.7)
	_ = ts.SetSustainAmount(-0.2)

	in := make([]float64, 256)
	for i := range in {
		in[i] = 0.6 * math.Sin(2*math.Pi*330*float64(i)/48000)
	}

	out1 := make([]float64, len(in))
	for i := range in {
		out1[i] = ts.ProcessSample(in[i])
	}

	ts.Reset()

	out2 := make([]float64, len(in))
	for i := range in {
		out2[i] = ts.ProcessSample(in[i])
	}

	for i := range out1 {
		if diff := math.Abs(out1[i] - out2[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch after reset: got=%g want=%g diff=%g", i, out2[i], out1[i], diff)
		}
	}
}
