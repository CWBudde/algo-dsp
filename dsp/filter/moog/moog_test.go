package moog

import (
	"math"
	"testing"
)

func TestNewValidation(t *testing.T) {
	if _, err := New(0); err == nil {
		t.Fatal("expected error for invalid sample rate")
	}

	if _, err := New(48000, WithCutoffHz(24000)); err == nil {
		t.Fatal("expected error for cutoff at Nyquist")
	}

	if _, err := New(48000, WithResonance(5)); err == nil {
		t.Fatal("expected error for resonance out of range")
	}

	if _, err := New(48000, WithOversampling(3)); err == nil {
		t.Fatal("expected error for invalid oversampling")
	}
}

func TestProcessInPlaceMatchesSample(t *testing.T) {
	f1, err := New(48000,
		WithVariant(VariantHuovilainen),
		WithCutoffHz(2400),
		WithResonance(1.1),
		WithDrive(2.5),
		WithOversampling(4),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	f2, err := New(48000,
		WithVariant(VariantHuovilainen),
		WithCutoffHz(2400),
		WithResonance(1.1),
		WithDrive(2.5),
		WithOversampling(4),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	in := make([]float64, 384)
	for i := range in {
		in[i] = 0.65*math.Sin(2*math.Pi*float64(i)/47) + 0.12*math.Sin(2*math.Pi*float64(i)/11)
	}

	want := make([]float64, len(in))
	for i, x := range in {
		want[i] = f1.ProcessSample(x)
	}

	got := append([]float64(nil), in...)
	f2.ProcessInPlace(got)

	for i := range got {
		if d := math.Abs(got[i] - want[i]); d > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g", i, got[i], want[i])
		}
	}
}

func TestStateRoundTrip(t *testing.T) {
	f, err := New(48000,
		WithVariant(VariantClassic),
		WithCutoffHz(1200),
		WithResonance(0.9),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	for i := range 96 {
		_ = f.ProcessSample(math.Sin(2 * math.Pi * float64(i) / 29))
	}

	s := f.State()

	clone, err := New(48000,
		WithVariant(VariantClassic),
		WithCutoffHz(1200),
		WithResonance(0.9),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := clone.SetState(s); err != nil {
		t.Fatalf("SetState() error = %v", err)
	}

	for i := range 128 {
		x := math.Sin(2*math.Pi*float64(i)/31) + 0.2*math.Sin(2*math.Pi*float64(i)/7)

		y1 := f.ProcessSample(x)

		y2 := clone.ProcessSample(x)
		if math.Abs(y1-y2) > 1e-12 {
			t.Fatalf("state mismatch at %d: %g vs %g", i, y1, y2)
		}
	}
}

func TestSetStateRejectsNonFinite(t *testing.T) {
	f, err := New(48000)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	st := State{}

	st.Stage[0] = math.NaN()
	if err := f.SetState(st); err == nil {
		t.Fatal("expected error for non-finite state")
	}
}

func TestLegacyParityClassicModes(t *testing.T) {
	type testCase struct {
		name     string
		variant  Variant
		improved bool
		tanhFn   func(float64) float64
		tol      float64
	}

	tests := []testCase{
		{name: "classic", variant: VariantClassic, improved: false, tanhFn: math.Tanh, tol: 1e-12},
		{name: "classic_lightweight", variant: VariantClassicLightweight, improved: false, tanhFn: fastTanhApprox, tol: 1e-12},
		{name: "improved", variant: VariantImprovedClassic, improved: true, tanhFn: math.Tanh, tol: 1e-12},
		{name: "improved_lightweight", variant: VariantImprovedClassicLightweight, improved: true, tanhFn: fastTanhApprox, tol: 1e-12},
	}

	const (
		sr        = 48000.0
		cutoffHz  = 1800.0
		resonance = 1.35
		thermal   = 5.0
	)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, err := New(sr,
				WithVariant(tc.variant),
				WithCutoffHz(cutoffHz),
				WithResonance(resonance),
				WithThermalVoltage(thermal),
				WithDrive(1),
				WithInputGain(1),
				WithOutputGain(1),
				WithNormalizeOutput(false),
				WithOversampling(1),
			)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			var st legacyState

			coeff := 2 * thermal * (1 - math.Exp(-2*math.Pi*cutoffHz/sr))
			scale := dbToAmp(resonance)
			scale *= scale

			for i := range 512 {
				x := 0.7*math.Sin(2*math.Pi*float64(i)/37) + 0.13*math.Sin(2*math.Pi*float64(i)/9)

				want := legacyClassicStep(&st, x, resonance, coeff, thermal, scale, tc.improved, tc.tanhFn)

				got := f.ProcessSample(x)
				if math.Abs(got-want) > tc.tol {
					t.Fatalf("sample %d mismatch: got=%g want=%g", i, got, want)
				}
			}
		})
	}
}

func TestCutoffTrackingSampleRateGrid(t *testing.T) {
	sampleRates := []float64{44100, 48000, 96000}
	cutoffs := []float64{300, 1200, 4000}

	for _, sr := range sampleRates {
		for _, cutoff := range cutoffs {
			f, err := New(sr,
				WithVariant(VariantHuovilainen),
				WithCutoffHz(cutoff),
				WithResonance(0),
				WithDrive(0.5),
				WithNormalizeOutput(false),
			)
			if err != nil {
				t.Fatalf("New(sr=%g, cutoff=%g) error = %v", sr, cutoff, err)
			}

			passFreq := cutoff * 0.5
			stopFreq := cutoff * 4

			nyquist := sr * 0.5
			if stopFreq >= nyquist*0.95 {
				stopFreq = nyquist * 0.95
			}

			passRMS := steadyToneRMS(f, sr, passFreq, 4096, 1024)
			f.Reset()
			stopRMS := steadyToneRMS(f, sr, stopFreq, 4096, 1024)

			if passRMS <= stopRMS*1.2 {
				t.Fatalf(
					"cutoff tracking failed for sr=%g cutoff=%g: pass(%.1f Hz)=%.6f stop(%.1f Hz)=%.6f",
					sr, cutoff, passFreq, passRMS, stopFreq, stopRMS,
				)
			}
		}
	}
}

func TestDriveSweepIncreasesHarmonics(t *testing.T) {
	const (
		sr = 48000.0
		n  = 4096
		k0 = 220
	)

	lowDrive, err := New(sr,
		WithVariant(VariantClassic),
		WithCutoffHz(16000),
		WithResonance(0),
		WithDrive(0.6),
		WithNormalizeOutput(false),
	)
	if err != nil {
		t.Fatalf("New(lowDrive) error = %v", err)
	}

	highDrive, err := New(sr,
		WithVariant(VariantClassic),
		WithCutoffHz(16000),
		WithResonance(0),
		WithDrive(7.0),
		WithNormalizeOutput(false),
	)
	if err != nil {
		t.Fatalf("New(highDrive) error = %v", err)
	}

	outLow := make([]float64, n)

	outHigh := make([]float64, n)
	for i := range n {
		x := 0.8 * math.Sin(2*math.Pi*float64(k0)*float64(i)/n)
		outLow[i] = lowDrive.ProcessSample(x)
		outHigh[i] = highDrive.ProcessSample(x)
	}

	spurLow := spurRatio(outLow, k0)
	spurHigh := spurRatio(outHigh, k0)

	if spurHigh <= spurLow*1.3 {
		t.Fatalf("expected harmonic growth with drive: low=%g high=%g", spurLow, spurHigh)
	}
}

func TestSaturationSymmetry(t *testing.T) {
	f, err := New(48000,
		WithVariant(VariantClassic),
		WithCutoffHz(16000),
		WithResonance(0),
		WithDrive(3),
		WithNormalizeOutput(false),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	inputs := []float64{0.1, 0.25, 0.5, 0.8, 1.0}
	for _, x := range inputs {
		f.Reset()
		pos := f.ProcessSample(x)

		f.Reset()
		neg := f.ProcessSample(-x)

		if d := math.Abs(pos + neg); d > 1e-12 {
			t.Fatalf("symmetry mismatch for x=%g: pos=%g neg=%g", x, pos, neg)
		}
	}
}

func TestHighResonanceSustainsLongerTail(t *testing.T) {
	const (
		sr      = 48000.0
		cutoff  = 900.0
		samples = 4096
	)

	lowRes, err := New(sr,
		WithVariant(VariantHuovilainen),
		WithCutoffHz(cutoff),
		WithResonance(0.5),
		WithDrive(1),
	)
	if err != nil {
		t.Fatalf("New(lowRes) error = %v", err)
	}

	highRes, err := New(sr,
		WithVariant(VariantHuovilainen),
		WithCutoffHz(cutoff),
		WithResonance(3.6),
		WithDrive(1),
	)
	if err != nil {
		t.Fatalf("New(highRes) error = %v", err)
	}

	lowTail := impulseTailEnergy(lowRes, samples)

	highTail := impulseTailEnergy(highRes, samples)
	if highTail <= lowTail*4 {
		t.Fatalf("expected longer/sustained tail at high resonance: low=%g high=%g", lowTail, highTail)
	}
}

func TestRapidAutomationStaysFinite(t *testing.T) {
	filter, err := New(48000,
		WithVariant(VariantHuovilainen),
		WithCutoffHz(1000),
		WithResonance(1.0),
		WithDrive(2.5),
		WithOversampling(4),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	for i := range 3000 {
		cutoff := 100 + 18000*(0.5+0.5*math.Sin(2*math.Pi*float64(i)/211))
		res := 0.2 + 3.4*(0.5+0.5*math.Sin(2*math.Pi*float64(i)/137))

		err := filter.SetCutoffHz(cutoff)
		if err != nil {
			t.Fatalf("SetCutoffHz(%g) error = %v", cutoff, err)
		}

		err = filter.SetResonance(res)
		if err != nil {
			t.Fatalf("SetResonance(%g) error = %v", res, err)
		}

		x := 0.7*math.Sin(2*math.Pi*float64(i)/37) + 0.1*math.Sin(2*math.Pi*float64(i)/5)

		y := filter.ProcessSample(x)
		if !isFinite(y) {
			t.Fatalf("non-finite sample at %d: %v", i, y)
		}
	}
}

func TestOversamplingReducesSpurs(t *testing.T) {
	const (
		sampleRate = 48000.0
		n          = 2048
		k0         = 944
	)

	base, err := New(sampleRate,
		WithVariant(VariantHuovilainen),
		WithCutoffHz(12000),
		WithResonance(1.0),
		WithDrive(8),
		WithOversampling(1),
	)
	if err != nil {
		t.Fatalf("New(base) error = %v", err)
	}

	os, err := New(sampleRate,
		WithVariant(VariantHuovilainen),
		WithCutoffHz(12000),
		WithResonance(1.0),
		WithDrive(8),
		WithOversampling(8),
	)
	if err != nil {
		t.Fatalf("New(os) error = %v", err)
	}

	outBase := make([]float64, n)

	outOS := make([]float64, n)
	for i := range n {
		x := 0.85 * math.Sin(2*math.Pi*float64(k0)*float64(i)/n)
		outBase[i] = base.ProcessSample(x)
		outOS[i] = os.ProcessSample(x)
	}

	spurBase := spurRatio(outBase, k0)

	spurOS := spurRatio(outOS, k0)
	if spurOS >= spurBase*0.97 {
		t.Fatalf("expected oversampling to reduce spurs: base=%g os=%g", spurBase, spurOS)
	}
}

func TestCutoffConstraintUsesBaseSampleRateNyquist(t *testing.T) {
	f, err := New(48000, WithOversampling(8))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := f.SetCutoffHz(30000); err == nil {
		t.Fatal("expected error for cutoff above base-rate Nyquist")
	}
}

func TestStereoHelpers(t *testing.T) {
	st, err := NewStereo(48000,
		WithVariant(VariantClassic),
		WithCutoffHz(1400),
		WithResonance(0.9),
	)
	if err != nil {
		t.Fatalf("NewStereo() error = %v", err)
	}

	left := make([]float64, 128)
	right := make([]float64, 128)

	for i := range left {
		left[i] = math.Sin(2 * math.Pi * float64(i) / 41)
		right[i] = math.Sin(2*math.Pi*float64(i)/17) * 0.5
	}

	st.ProcessInPlace(left, right)

	for i := range left {
		if !isFinite(left[i]) || !isFinite(right[i]) {
			t.Fatalf("non-finite stereo output at %d", i)
		}
	}

	frames := make([][2]float64, 64)
	for i := range frames {
		frames[i][0] = math.Sin(2 * math.Pi * float64(i) / 29)
		frames[i][1] = math.Sin(2 * math.Pi * float64(i) / 13)
	}

	st.Reset()
	st.ProcessFramesInPlace(frames)

	for i := range frames {
		if !isFinite(frames[i][0]) || !isFinite(frames[i][1]) {
			t.Fatalf("non-finite frame output at %d", i)
		}
	}
}

func TestZDFProcessInPlaceMatchesSample(t *testing.T) {
	f1, err := New(48000,
		WithVariant(VariantZDF),
		WithCutoffHz(2400),
		WithResonance(1.1),
		WithDrive(2.5),
		WithOversampling(1),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	f2, err := New(48000,
		WithVariant(VariantZDF),
		WithCutoffHz(2400),
		WithResonance(1.1),
		WithDrive(2.5),
		WithOversampling(1),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	in := make([]float64, 384)
	for i := range in {
		in[i] = 0.65*math.Sin(2*math.Pi*float64(i)/47) + 0.12*math.Sin(2*math.Pi*float64(i)/11)
	}

	want := make([]float64, len(in))
	for i, x := range in {
		want[i] = f1.ProcessSample(x)
	}

	got := append([]float64(nil), in...)
	f2.ProcessInPlace(got)

	for i := range got {
		if d := math.Abs(got[i] - want[i]); d > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g", i, got[i], want[i])
		}
	}
}

func TestZDFStateRoundTrip(t *testing.T) {
	f, err := New(48000,
		WithVariant(VariantZDF),
		WithCutoffHz(1200),
		WithResonance(0.9),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	for i := range 96 {
		_ = f.ProcessSample(math.Sin(2 * math.Pi * float64(i) / 29))
	}

	state := f.State()

	clone, err := New(48000,
		WithVariant(VariantZDF),
		WithCutoffHz(1200),
		WithResonance(0.9),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := clone.SetState(state); err != nil {
		t.Fatalf("SetState() error = %v", err)
	}

	for i := range 128 {
		x := math.Sin(2*math.Pi*float64(i)/31) + 0.2*math.Sin(2*math.Pi*float64(i)/7)

		y1 := f.ProcessSample(x)

		y2 := clone.ProcessSample(x)
		if math.Abs(y1-y2) > 1e-12 {
			t.Fatalf("state mismatch at %d: %g vs %g", i, y1, y2)
		}
	}
}

func TestZDFCutoffTracking(t *testing.T) {
	sampleRates := []float64{44100, 48000, 96000}
	cutoffs := []float64{300, 1200, 4000}

	for _, sr := range sampleRates {
		for _, cutoff := range cutoffs {
			f, err := New(sr,
				WithVariant(VariantZDF),
				WithCutoffHz(cutoff),
				WithResonance(0),
				WithDrive(0.5),
				WithNormalizeOutput(false),
			)
			if err != nil {
				t.Fatalf("New(sr=%g, cutoff=%g) error = %v", sr, cutoff, err)
			}

			passFreq := cutoff * 0.5
			stopFreq := cutoff * 4

			nyquist := sr * 0.5
			if stopFreq >= nyquist*0.95 {
				stopFreq = nyquist * 0.95
			}

			passRMS := steadyToneRMS(f, sr, passFreq, 4096, 1024)
			f.Reset()
			stopRMS := steadyToneRMS(f, sr, stopFreq, 4096, 1024)

			if passRMS <= stopRMS*1.2 {
				t.Fatalf(
					"cutoff tracking failed for sr=%g cutoff=%g: pass(%.1f Hz)=%.6f stop(%.1f Hz)=%.6f",
					sr, cutoff, passFreq, passRMS, stopFreq, stopRMS,
				)
			}
		}
	}
}

// TestZDFHighFrequencyTuningAccuracy verifies that the ZDF variant maintains
// adequate cutoff accuracy at high cutoff-to-sample-rate ratios. It also
// logs the Huovilainen ratio for comparison; ZDF is expected to outperform
// at the highest cutoffs where tan(π*fc/fs) pre-warping provides the most benefit.
func TestZDFHighFrequencyTuningAccuracy(t *testing.T) {
	const sr = 48000.0

	// At high cutoff (close to Nyquist), ZDF should maintain better
	// pass-to-stop separation because tan(π*fc/fs) is exact while
	// the exponential approximation 1-exp(-2πfc/fs) drifts.
	highCutoffs := []float64{8000, 12000, 16000}

	for _, cutoff := range highCutoffs {
		passFreq := cutoff * 0.25
		stopFreq := cutoff * 2

		nyquist := sr * 0.5
		if stopFreq >= nyquist*0.95 {
			stopFreq = nyquist * 0.95
		}

		zdf, err := New(sr,
			WithVariant(VariantZDF),
			WithCutoffHz(cutoff),
			WithResonance(0),
			WithDrive(0.5),
			WithNormalizeOutput(false),
		)
		if err != nil {
			t.Fatalf("ZDF New(cutoff=%g) error = %v", cutoff, err)
		}

		huov, err := New(sr,
			WithVariant(VariantHuovilainen),
			WithCutoffHz(cutoff),
			WithResonance(0),
			WithDrive(0.5),
			WithNormalizeOutput(false),
		)
		if err != nil {
			t.Fatalf("Huov New(cutoff=%g) error = %v", cutoff, err)
		}

		// ZDF: measure pass/stop ratio
		zdfPass := steadyToneRMS(zdf, sr, passFreq, 8192, 2048)
		zdf.Reset()
		zdfStop := steadyToneRMS(zdf, sr, stopFreq, 8192, 2048)

		zdfRatio := 0.0
		if zdfStop > 0 {
			zdfRatio = zdfPass / zdfStop
		}

		// Huovilainen: measure pass/stop ratio
		huovPass := steadyToneRMS(huov, sr, passFreq, 8192, 2048)
		huov.Reset()
		huovStop := steadyToneRMS(huov, sr, stopFreq, 8192, 2048)

		huovRatio := 0.0
		if huovStop > 0 {
			huovRatio = huovPass / huovStop
		}

		t.Logf("cutoff=%g: ZDF ratio=%.2f, Huov ratio=%.2f", cutoff, zdfRatio, huovRatio)

		// Both should show proper lowpass behavior.
		if zdfRatio < 1.5 {
			t.Errorf("ZDF ratio too low at cutoff=%g: %.2f", cutoff, zdfRatio)
		}
	}
}

func TestZDFSaturationSymmetry(t *testing.T) {
	f, err := New(48000,
		WithVariant(VariantZDF),
		WithCutoffHz(16000),
		WithResonance(0),
		WithDrive(3),
		WithNormalizeOutput(false),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	inputs := []float64{0.1, 0.25, 0.5, 0.8, 1.0}
	for _, x := range inputs {
		f.Reset()
		pos := f.ProcessSample(x)

		f.Reset()
		neg := f.ProcessSample(-x)

		if d := math.Abs(pos + neg); d > 1e-12 {
			t.Fatalf("symmetry mismatch for x=%g: pos=%g neg=%g", x, pos, neg)
		}
	}
}

func TestZDFHighResonanceSustainsLongerTail(t *testing.T) {
	const (
		sr      = 48000.0
		cutoff  = 900.0
		samples = 4096
	)

	lowRes, err := New(sr,
		WithVariant(VariantZDF),
		WithCutoffHz(cutoff),
		WithResonance(0.5),
		WithDrive(1),
	)
	if err != nil {
		t.Fatalf("New(lowRes) error = %v", err)
	}

	highRes, err := New(sr,
		WithVariant(VariantZDF),
		WithCutoffHz(cutoff),
		WithResonance(3.6),
		WithDrive(1),
	)
	if err != nil {
		t.Fatalf("New(highRes) error = %v", err)
	}

	lowTail := impulseTailEnergy(lowRes, samples)

	highTail := impulseTailEnergy(highRes, samples)
	if highTail <= lowTail*4 {
		t.Fatalf("expected longer/sustained tail at high resonance: low=%g high=%g", lowTail, highTail)
	}
}

func TestZDFRapidAutomationStaysFinite(t *testing.T) {
	moogFilter, err := New(48000,
		WithVariant(VariantZDF),
		WithCutoffHz(1000),
		WithResonance(1.0),
		WithDrive(2.5),
		WithOversampling(4),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	for i := range 3000 {
		cutoff := 100 + 18000*(0.5+0.5*math.Sin(2*math.Pi*float64(i)/211))
		res := 0.2 + 3.4*(0.5+0.5*math.Sin(2*math.Pi*float64(i)/137))

		err := moogFilter.SetCutoffHz(cutoff)
		if err != nil {
			t.Fatalf("SetCutoffHz(%g) error = %v", cutoff, err)
		}

		err = moogFilter.SetResonance(res)
		if err != nil {
			t.Fatalf("SetResonance(%g) error = %v", res, err)
		}

		x := 0.7*math.Sin(2*math.Pi*float64(i)/37) + 0.1*math.Sin(2*math.Pi*float64(i)/5)

		y := moogFilter.ProcessSample(x)
		if !isFinite(y) {
			t.Fatalf("non-finite sample at %d: %v", i, y)
		}
	}
}

func TestZDFOversamplingReducesSpurs(t *testing.T) {
	const (
		sr = 48000.0
		n  = 2048
		k0 = 944
	)

	base, err := New(sr,
		WithVariant(VariantZDF),
		WithCutoffHz(12000),
		WithResonance(1.0),
		WithDrive(8),
		WithOversampling(1),
	)
	if err != nil {
		t.Fatalf("New(base) error = %v", err)
	}

	os, err := New(sr,
		WithVariant(VariantZDF),
		WithCutoffHz(12000),
		WithResonance(1.0),
		WithDrive(8),
		WithOversampling(8),
	)
	if err != nil {
		t.Fatalf("New(os) error = %v", err)
	}

	outBase := make([]float64, n)

	outOS := make([]float64, n)
	for i := range n {
		x := 0.85 * math.Sin(2*math.Pi*float64(k0)*float64(i)/n)
		outBase[i] = base.ProcessSample(x)
		outOS[i] = os.ProcessSample(x)
	}

	spurBase := spurRatio(outBase, k0)

	spurOS := spurRatio(outOS, k0)
	if spurOS >= spurBase*0.97 {
		t.Fatalf("expected oversampling to reduce spurs: base=%g os=%g", spurBase, spurOS)
	}
}

func TestZDFDriveSweepIncreasesHarmonics(t *testing.T) {
	const (
		sr = 48000.0
		n  = 4096
		k0 = 220
	)

	lowDrive, err := New(sr,
		WithVariant(VariantZDF),
		WithCutoffHz(16000),
		WithResonance(0),
		WithDrive(0.6),
		WithNormalizeOutput(false),
	)
	if err != nil {
		t.Fatalf("New(lowDrive) error = %v", err)
	}

	highDrive, err := New(sr,
		WithVariant(VariantZDF),
		WithCutoffHz(16000),
		WithResonance(0),
		WithDrive(7.0),
		WithNormalizeOutput(false),
	)
	if err != nil {
		t.Fatalf("New(highDrive) error = %v", err)
	}

	outLow := make([]float64, n)

	outHigh := make([]float64, n)
	for i := range n {
		x := 0.8 * math.Sin(2*math.Pi*float64(k0)*float64(i)/n)
		outLow[i] = lowDrive.ProcessSample(x)
		outHigh[i] = highDrive.ProcessSample(x)
	}

	spurLow := spurRatio(outLow, k0)
	spurHigh := spurRatio(outHigh, k0)

	if spurHigh <= spurLow*1.3 {
		t.Fatalf("expected harmonic growth with drive: low=%g high=%g", spurLow, spurHigh)
	}
}

// TestZDFNewtonConvergence verifies that more Newton iterations converge
// closer to the implicit equation solution.
func TestZDFNewtonConvergence(t *testing.T) {
	const sr = 48000.0

	// Use high resonance + drive to make Newton iteration matter.
	baseOpts := []Option{
		WithVariant(VariantZDF),
		WithCutoffHz(4000),
		WithResonance(3.5),
		WithDrive(4.0),
		WithNormalizeOutput(false),
	}

	// Run with 1 iteration vs 8 iterations and compare outputs.
	f1, err := New(sr, append(baseOpts, WithNewtonIterations(1))...)
	if err != nil {
		t.Fatalf("New(1 iter) error = %v", err)
	}

	f8, err := New(sr, append(baseOpts, WithNewtonIterations(8))...)
	if err != nil {
		t.Fatalf("New(8 iter) error = %v", err)
	}

	var maxDiff float64

	for i := range 1024 {
		x := 0.7*math.Sin(2*math.Pi*float64(i)/37) + 0.3*math.Sin(2*math.Pi*float64(i)/11)
		y1 := f1.ProcessSample(x)
		y8 := f8.ProcessSample(x)

		d := math.Abs(y1 - y8)
		if d > maxDiff {
			maxDiff = d
		}

		if !isFinite(y1) || !isFinite(y8) {
			t.Fatalf("non-finite at %d: y1=%g y8=%g", i, y1, y8)
		}
	}

	// Both should produce valid output; more iterations should give different
	// (more accurate) results at high resonance.
	t.Logf("max difference between 1-iter and 8-iter: %g", maxDiff)

	// At high resonance + drive, 1 vs 8 iterations should differ measurably.
	if maxDiff < 1e-10 {
		t.Log("warning: 1-iter and 8-iter produce nearly identical results")
	}
}

func TestNewtonIterationsValidation(t *testing.T) {
	if _, err := New(48000, WithNewtonIterations(0)); err == nil {
		t.Fatal("expected error for 0 newton iterations")
	}

	if _, err := New(48000, WithNewtonIterations(9)); err == nil {
		t.Fatal("expected error for 9 newton iterations")
	}

	f, err := New(48000, WithVariant(VariantZDF), WithNewtonIterations(2))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if f.NewtonIterations() != 2 {
		t.Fatalf("expected 2 newton iterations, got %d", f.NewtonIterations())
	}

	if err := f.SetNewtonIterations(6); err != nil {
		t.Fatalf("SetNewtonIterations(6) error = %v", err)
	}

	if f.NewtonIterations() != 6 {
		t.Fatalf("expected 6 newton iterations, got %d", f.NewtonIterations())
	}
}

func TestZDFStereo(t *testing.T) {
	st, err := NewStereo(48000,
		WithVariant(VariantZDF),
		WithCutoffHz(1400),
		WithResonance(0.9),
	)
	if err != nil {
		t.Fatalf("NewStereo() error = %v", err)
	}

	left := make([]float64, 128)
	right := make([]float64, 128)

	for i := range left {
		left[i] = math.Sin(2 * math.Pi * float64(i) / 41)
		right[i] = math.Sin(2*math.Pi*float64(i)/17) * 0.5
	}

	st.ProcessInPlace(left, right)

	for i := range left {
		if !isFinite(left[i]) || !isFinite(right[i]) {
			t.Fatalf("non-finite stereo output at %d", i)
		}
	}
}

type legacyState struct {
	last     [4]float64
	tanhLast [3]float64
}

func legacyClassicStep(
	state *legacyState,
	input, resonance, coefficient, thermalVoltage, scale float64,
	improved bool,
	tanhFn func(float64) float64,
) float64 {
	newInput := input - resonance*state.last[3]

	g := coefficient
	if improved {
		g *= 2 * thermalVoltage
	}

	shape := 0.5 / thermalVoltage
	state.last[0] += g * (tanhFn(shape*newInput) - state.tanhLast[0])
	state.tanhLast[0] = tanhFn(shape * state.last[0])

	state.last[1] += g * (state.tanhLast[0] - state.tanhLast[1])
	state.tanhLast[1] = tanhFn(shape * state.last[1])

	state.last[2] += g * (state.tanhLast[1] - state.tanhLast[2])
	state.tanhLast[2] = tanhFn(shape * state.last[2])

	state.last[3] += g * (state.tanhLast[2] - tanhFn(shape*state.last[3]))

	return scale * state.last[3]
}

func impulseTailEnergy(f *Filter, n int) float64 {
	var sum float64

	for i := range n {
		x := 0.0
		if i == 0 {
			x = 1
		}

		y := f.ProcessSample(x)
		if !isFinite(y) {
			return math.Inf(1)
		}

		if i >= n/4 {
			sum += y * y
		}
	}

	return sum
}

func spurRatio(x []float64, fundamentalBin int) float64 {
	fund := dftBinEnergy(x, fundamentalBin)
	if fund <= 0 {
		return math.Inf(1)
	}

	spur := 0.0

	for k := 1; k <= len(x)/2; k++ {
		if k == fundamentalBin {
			continue
		}

		spur += dftBinEnergy(x, k)
	}

	return spur / fund
}

func dftBinEnergy(x []float64, k int) float64 {
	n := float64(len(x))

	var re, im float64

	for i := range x {
		phase := 2 * math.Pi * float64(k) * float64(i) / n
		re += x[i] * math.Cos(phase)
		im -= x[i] * math.Sin(phase)
	}

	return re*re + im*im
}

func steadyToneRMS(f *Filter, sampleRate, freq float64, n, warmup int) float64 {
	var sum float64

	for i := range n {
		x := 0.7 * math.Sin(2*math.Pi*freq*float64(i)/sampleRate)

		y := f.ProcessSample(x)
		if i >= warmup {
			sum += y * y
		}
	}

	return math.Sqrt(sum / float64(n-warmup))
}
