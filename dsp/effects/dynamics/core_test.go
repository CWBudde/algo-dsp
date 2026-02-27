package dynamics

import "testing"

func TestCompressorTopologyAndDetectorModes(t *testing.T) {
	c, err := NewCompressor(48000)
	if err != nil {
		t.Fatalf("NewCompressor() error = %v", err)
	}

	if err := c.SetAutoMakeup(false); err != nil {
		t.Fatal(err)
	}

	if err := c.SetMakeupGain(0); err != nil {
		t.Fatal(err)
	}

	if err := c.SetThreshold(-18); err != nil {
		t.Fatal(err)
	}

	if err := c.SetRatio(6); err != nil {
		t.Fatal(err)
	}

	if err := c.SetAttack(2); err != nil {
		t.Fatal(err)
	}

	if err := c.SetRelease(100); err != nil {
		t.Fatal(err)
	}

	if err := c.SetDetectorMode(DetectorModeRMS); err != nil {
		t.Fatal(err)
	}

	if err := c.SetRMSWindow(20); err != nil {
		t.Fatal(err)
	}

	if err := c.SetTopology(DynamicsTopologyFeedback); err != nil {
		t.Fatal(err)
	}

	var outFeedback float64
	for range 512 {
		outFeedback = c.ProcessSample(0.8)
	}

	c.Reset()

	if err := c.SetTopology(DynamicsTopologyFeedforward); err != nil {
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

	if err := c.SetSidechainLowCut(24000); err == nil {
		t.Fatal("expected error for low-cut >= nyquist")
	}

	if err := c.SetSidechainHighCut(24000); err == nil {
		t.Fatal("expected error for high-cut >= nyquist")
	}

	if err := c.SetSidechainHighCut(1000); err != nil {
		t.Fatalf("unexpected high-cut error: %v", err)
	}

	if err := c.SetSidechainLowCut(2000); err == nil {
		t.Fatal("expected error for low-cut >= high-cut")
	}
}

func TestCompressorSidechainProcessingPath(t *testing.T) {
	c, err := NewCompressor(48000)
	if err != nil {
		t.Fatalf("NewCompressor() error = %v", err)
	}

	if err := c.SetAutoMakeup(false); err != nil {
		t.Fatal(err)
	}

	if err := c.SetMakeupGain(0); err != nil {
		t.Fatal(err)
	}

	if err := c.SetThreshold(-30); err != nil {
		t.Fatal(err)
	}

	if err := c.SetRatio(8); err != nil {
		t.Fatal(err)
	}

	if err := c.SetSidechainLowCut(300); err != nil {
		t.Fatal(err)
	}

	if err := c.SetSidechainHighCut(6000); err != nil {
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

	if err := g.SetThreshold(-20); err != nil {
		t.Fatal(err)
	}

	if err := g.SetRatio(8); err != nil {
		t.Fatal(err)
	}

	if err := g.SetDetectorMode(DetectorModeRMS); err != nil {
		t.Fatal(err)
	}

	if err := g.SetRMSWindow(25); err != nil {
		t.Fatal(err)
	}

	if err := g.SetTopology(DynamicsTopologyFeedback); err != nil {
		t.Fatal(err)
	}

	var fb float64
	for range 512 {
		fb = g.ProcessSample(0.05)
	}

	g.Reset()

	if err := g.SetTopology(DynamicsTopologyFeedforward); err != nil {
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

	if err := g.SetThreshold(-30); err != nil {
		t.Fatal(err)
	}

	if err := g.SetRange(-60); err != nil {
		t.Fatal(err)
	}

	if err := g.SetSidechainLowCut(400); err != nil {
		t.Fatal(err)
	}

	if err := g.SetSidechainHighCut(5000); err != nil {
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
