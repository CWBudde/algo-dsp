package dynamics

import (
	"math"
	"testing"
)

func TestNewExpander(t *testing.T) {
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
			e, err := NewExpander(tt.sampleRate)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewExpander() error=%v wantErr=%v", err, tt.wantErr)
			}
			if !tt.wantErr && e == nil {
				t.Fatal("NewExpander() returned nil without error")
			}
		})
	}
}

func TestExpanderDefaults(t *testing.T) {
	e, err := NewExpander(48000)
	if err != nil {
		t.Fatal(err)
	}

	if e.Threshold() != defaultExpanderThresholdDB {
		t.Fatalf("threshold=%f", e.Threshold())
	}
	if e.Ratio() != defaultExpanderRatio {
		t.Fatalf("ratio=%f", e.Ratio())
	}
	if e.Knee() != defaultExpanderKneeDB {
		t.Fatalf("knee=%f", e.Knee())
	}
	if e.Attack() != defaultExpanderAttackMs {
		t.Fatalf("attack=%f", e.Attack())
	}
	if e.Release() != defaultExpanderReleaseMs {
		t.Fatalf("release=%f", e.Release())
	}
	if e.Range() != defaultExpanderRangeDB {
		t.Fatalf("range=%f", e.Range())
	}
}

func TestExpanderParameterValidation(t *testing.T) {
	e, _ := NewExpander(48000)

	if err := e.SetRatio(0.5); err == nil {
		t.Fatal("expected ratio error")
	}
	if err := e.SetKnee(25); err == nil {
		t.Fatal("expected knee error")
	}
	if err := e.SetAttack(0.05); err == nil {
		t.Fatal("expected attack error")
	}
	if err := e.SetRelease(0.5); err == nil {
		t.Fatal("expected release error")
	}
	if err := e.SetRange(-121); err == nil {
		t.Fatal("expected range error")
	}
}

func TestExpanderGainBehavior(t *testing.T) {
	e, _ := NewExpander(48000)
	_ = e.SetThreshold(-20)
	_ = e.SetRatio(6)
	_ = e.SetKnee(0)
	_ = e.SetRange(-80)

	// Above threshold should pass mostly unchanged after settling.
	var above float64
	for i := 0; i < 1024; i++ {
		above = e.ProcessSample(0.5)
	}
	if above < 0.49 {
		t.Fatalf("expected above-threshold pass-through, got %f", above)
	}

	e.Reset()
	// Below threshold should be attenuated.
	var below float64
	for i := 0; i < 1024; i++ {
		below = e.ProcessSample(0.02)
	}
	if below >= 0.02 {
		t.Fatalf("expected attenuation below threshold, got %f", below)
	}
}

func TestExpanderTopologyDetectorAndSidechain(t *testing.T) {
	e, _ := NewExpander(48000)
	_ = e.SetThreshold(-25)
	_ = e.SetRatio(4)
	_ = e.SetDetectorMode(DetectorModeRMS)
	_ = e.SetRMSWindow(20)
	_ = e.SetTopology(DynamicsTopologyFeedback)

	var fb float64
	for i := 0; i < 512; i++ {
		fb = e.ProcessSample(0.05)
	}
	e.Reset()
	_ = e.SetTopology(DynamicsTopologyFeedforward)
	var ff float64
	for i := 0; i < 512; i++ {
		ff = e.ProcessSample(0.05)
	}
	if fb == ff {
		t.Fatalf("expected topology difference, got %f", ff)
	}

	_ = e.SetSidechainLowCut(300)
	_ = e.SetSidechainHighCut(5000)
	var out float64
	for i := 0; i < 512; i++ {
		out = e.ProcessSampleSidechain(0.2, 0.001)
	}
	if out >= 0.2 {
		t.Fatalf("expected sidechain-driven attenuation, got %f", out)
	}
}
