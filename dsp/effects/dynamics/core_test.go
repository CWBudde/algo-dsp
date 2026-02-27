package dynamics

import "testing"

func TestCompressorTopologyAndDetectorModes(t *testing.T) {
	c, err := NewCompressor(48000)
	if err != nil {
		t.Fatalf("NewCompressor() error = %v", err)
	}

	err = c.SetAutoMakeup(false)
	if err != nil {
		t.Fatal(err)
	}

	err = c.SetMakeupGain(0)
	if err != nil {
		t.Fatal(err)
	}

	err = c.SetThreshold(-18)
	if err != nil {
		t.Fatal(err)
	}

	err = c.SetRatio(6)
	if err != nil {
		t.Fatal(err)
	}

	err = c.SetAttack(2)
	if err != nil {
		t.Fatal(err)
	}

	err = c.SetRelease(100)
	if err != nil {
		t.Fatal(err)
	}

	err = c.SetDetectorMode(DetectorModeRMS)
	if err != nil {
		t.Fatal(err)
	}

	err = c.SetRMSWindow(20)
	if err != nil {
		t.Fatal(err)
	}

	err = c.SetTopology(DynamicsTopologyFeedback)
	if err != nil {
		t.Fatal(err)
	}

	var outFeedback float64
	for range 512 {
		outFeedback = c.ProcessSample(0.8)
	}

	c.Reset()

	err = c.SetTopology(DynamicsTopologyFeedforward)
	if err != nil {
		t.Fatal(err)
	}

	var outFeedforward float64
	for range 512 {
		outFeedforward = c.ProcessSample(0.8)
	}

	if outFeedback == outFeedforward {
		t.Fatalf("expected topology-dependent output difference, got equal value %f", outFeedback)
	}
}

func TestCompressorSidechainFilterValidation(t *testing.T) {
	c, err := NewCompressor(48000)
	if err != nil {
		t.Fatalf("NewCompressor() error = %v", err)
	}

	err = c.SetSidechainLowCut(24000)
	if err == nil {
		t.Fatal("expected error for low-cut >= nyquist")
	}

	err = c.SetSidechainHighCut(24000)
	if err == nil {
		t.Fatal("expected error for high-cut >= nyquist")
	}

	err = c.SetSidechainHighCut(1000)
	if err != nil {
		t.Fatalf("unexpected high-cut error: %v", err)
	}

	err = c.SetSidechainLowCut(2000)
	if err == nil {
		t.Fatal("expected error for low-cut >= high-cut")
	}
}

func TestCompressorSidechainProcessingPath(t *testing.T) {
	c, err := NewCompressor(48000)
	if err != nil {
		t.Fatalf("NewCompressor() error = %v", err)
	}

	err = c.SetAutoMakeup(false)
	if err != nil {
		t.Fatal(err)
	}

	err = c.SetMakeupGain(0)
	if err != nil {
		t.Fatal(err)
	}

	err = c.SetThreshold(-30)
	if err != nil {
		t.Fatal(err)
	}

	err = c.SetRatio(8)
	if err != nil {
		t.Fatal(err)
	}

	err = c.SetSidechainLowCut(300)
	if err != nil {
		t.Fatal(err)
	}

	err = c.SetSidechainHighCut(6000)
	if err != nil {
		t.Fatal(err)
	}

	// Silent program material should still be attenuated if sidechain drives detection.
	var out float64
	for range 1024 {
		out = c.ProcessSampleSidechain(0.2, 1.0)
	}

	if out >= 0.2 {
		t.Fatalf("expected sidechain-driven gain reduction, got %f", out)
	}
}

func TestGateTopologyAndDetectorModes(t *testing.T) {
	g, err := NewGate(48000)
	if err != nil {
		t.Fatalf("NewGate() error = %v", err)
	}

	err = g.SetThreshold(-20)
	if err != nil {
		t.Fatal(err)
	}

	err = g.SetRatio(8)
	if err != nil {
		t.Fatal(err)
	}

	err = g.SetDetectorMode(DetectorModeRMS)
	if err != nil {
		t.Fatal(err)
	}

	err = g.SetRMSWindow(25)
	if err != nil {
		t.Fatal(err)
	}

	err = g.SetTopology(DynamicsTopologyFeedback)
	if err != nil {
		t.Fatal(err)
	}

	var fb float64
	for range 512 {
		fb = g.ProcessSample(0.05)
	}

	g.Reset()

	err = g.SetTopology(DynamicsTopologyFeedforward)
	if err != nil {
		t.Fatal(err)
	}

	var ff float64
	for range 512 {
		ff = g.ProcessSample(0.05)
	}

	if fb == ff {
		t.Fatalf("expected topology-dependent output difference, got equal value %f", ff)
	}
}

func TestGateSidechainProcessingPath(t *testing.T) {
	g, err := NewGate(48000)
	if err != nil {
		t.Fatalf("NewGate() error = %v", err)
	}

	err = g.SetThreshold(-30)
	if err != nil {
		t.Fatal(err)
	}

	err = g.SetRange(-60)
	if err != nil {
		t.Fatal(err)
	}

	err = g.SetSidechainLowCut(400)
	if err != nil {
		t.Fatal(err)
	}

	err = g.SetSidechainHighCut(5000)
	if err != nil {
		t.Fatal(err)
	}

	var out float64
	for range 512 {
		out = g.ProcessSampleSidechain(0.2, 0.001)
	}

	if out >= 0.2 {
		t.Fatalf("expected sidechain-driven attenuation, got %f", out)
	}
}
