package spatial

import (
	"errors"
	"math"
	"testing"
)

type fixedHRTFProvider struct {
	set HRTFImpulseResponseSet
	err error
}

func (p fixedHRTFProvider) ImpulseResponses(_ float64) (HRTFImpulseResponseSet, error) {
	if p.err != nil {
		return HRTFImpulseResponseSet{}, p.err
	}

	return p.set, nil
}

func TestHRTFCrosstalkSimulatorValidation(t *testing.T) {
	_, err := NewHRTFCrosstalkSimulator(0)
	if err == nil {
		t.Fatal("expected error for invalid sample rate")
	}

	_, err = NewHRTFCrosstalkSimulator(48000)
	if err == nil {
		t.Fatal("expected error for missing provider")
	}

	provider := fixedHRTFProvider{set: HRTFImpulseResponseSet{
		LeftCross:  []float64{0.1},
		RightCross: []float64{0.1},
	}}

	_, err = NewHRTFCrosstalkSimulator(48000,
		WithHRTFProvider(provider),
		WithHRTFMode(HRTFModeComplete))
	if err == nil {
		t.Fatal("expected error for complete mode without direct IR")
	}
}

func TestHRTFCrosstalkSimulatorCrossfeedOnlyRouting(t *testing.T) {
	provider := fixedHRTFProvider{set: HRTFImpulseResponseSet{
		LeftCross:  []float64{0.25},
		RightCross: []float64{0.5},
	}}

	s, err := NewHRTFCrosstalkSimulator(48000,
		WithHRTFProvider(provider),
		WithHRTFMode(HRTFModeCrossfeedOnly),
	)
	if err != nil {
		t.Fatalf("NewHRTFCrosstalkSimulator() error = %v", err)
	}

	outL, outR := s.ProcessStereo(1, 2)
	if math.Abs(outL-(1+2*0.25)) > 1e-12 {
		t.Fatalf("crossfeed-only left mismatch: got=%g", outL)
	}

	if math.Abs(outR-(2+1*0.5)) > 1e-12 {
		t.Fatalf("crossfeed-only right mismatch: got=%g", outR)
	}
}

func TestHRTFCrosstalkSimulatorCompleteRouting(t *testing.T) {
	provider := fixedHRTFProvider{set: HRTFImpulseResponseSet{
		LeftDirect:  []float64{0.8},
		LeftCross:   []float64{0.1},
		RightDirect: []float64{0.9},
		RightCross:  []float64{0.2},
	}}

	s, err := NewHRTFCrosstalkSimulator(48000,
		WithHRTFProvider(provider),
		WithHRTFMode(HRTFModeComplete),
	)
	if err != nil {
		t.Fatalf("NewHRTFCrosstalkSimulator() error = %v", err)
	}

	outL, outR := s.ProcessStereo(1, 2)
	if math.Abs(outL-(1*0.8+2*0.1)) > 1e-12 {
		t.Fatalf("complete left mismatch: got=%g", outL)
	}

	if math.Abs(outR-(2*0.9+1*0.2)) > 1e-12 {
		t.Fatalf("complete right mismatch: got=%g", outR)
	}
}

func TestHRTFCrosstalkSimulatorResetDeterministic(t *testing.T) {
	provider := fixedHRTFProvider{set: HRTFImpulseResponseSet{
		LeftDirect:  []float64{0.8, 0.1},
		LeftCross:   []float64{0.1, 0.05},
		RightDirect: []float64{0.8, -0.1},
		RightCross:  []float64{-0.1, 0.05},
	}}

	s, err := NewHRTFCrosstalkSimulator(48000,
		WithHRTFProvider(provider),
		WithHRTFMode(HRTFModeComplete),
	)
	if err != nil {
		t.Fatalf("NewHRTFCrosstalkSimulator() error = %v", err)
	}

	inL := []float64{1, 0, 0, 0, 0}
	inR := []float64{0, 1, 0, 0, 0}
	outL1 := make([]float64, len(inL))
	outR1 := make([]float64, len(inR))

	for i := range inL {
		outL1[i], outR1[i] = s.ProcessStereo(inL[i], inR[i])
	}

	s.Reset()

	for i := range inL {
		outL2, outR2 := s.ProcessStereo(inL[i], inR[i])
		if math.Abs(outL1[i]-outL2) > 1e-12 {
			t.Fatalf("left mismatch at %d after reset", i)
		}

		if math.Abs(outR1[i]-outR2) > 1e-12 {
			t.Fatalf("right mismatch at %d after reset", i)
		}
	}
}

func TestHRTFCrosstalkSimulatorProviderReload(t *testing.T) {
	providerA := fixedHRTFProvider{set: HRTFImpulseResponseSet{
		LeftCross:  []float64{0.2},
		RightCross: []float64{0.2},
	}}
	providerB := fixedHRTFProvider{set: HRTFImpulseResponseSet{
		LeftCross:  []float64{0.5},
		RightCross: []float64{0.5},
	}}

	s, err := NewHRTFCrosstalkSimulator(48000,
		WithHRTFProvider(providerA),
		WithHRTFMode(HRTFModeCrossfeedOnly),
	)
	if err != nil {
		t.Fatalf("NewHRTFCrosstalkSimulator() error = %v", err)
	}

	outL1, _ := s.ProcessStereo(0, 1)

	err = s.SetProvider(providerB)
	if err != nil {
		t.Fatalf("SetProvider() error = %v", err)
	}

	outL2, _ := s.ProcessStereo(0, 1)
	if outL2 <= outL1 {
		t.Fatalf("expected stronger crossfeed after provider reload: before=%g after=%g", outL1, outL2)
	}
}

func TestHRTFCrosstalkSimulatorProviderError(t *testing.T) {
	provider := fixedHRTFProvider{err: errors.New("load failed")}

	if _, err := NewHRTFCrosstalkSimulator(48000,
		WithHRTFProvider(provider)); err == nil {
		t.Fatal("expected provider error")
	}
}
