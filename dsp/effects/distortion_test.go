package effects

import (
	"math"
	"testing"
)

func TestDistortionValidation(t *testing.T) {
	_, err := NewDistortion(0)
	if err == nil {
		t.Fatal("expected error for invalid sample rate")
	}

	_, err = NewDistortion(48000, WithDistortionDrive(100))
	if err == nil {
		t.Fatal("expected error for invalid drive")
	}

	_, err = NewDistortion(48000, WithDistortionMode(DistortionMode(999)))
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}

	_, err = NewDistortion(48000,
		WithDistortionMode(DistortionModeChebyshev),
		WithChebyshevOrder(4),
		WithChebyshevHarmonicMode(ChebyshevHarmonicOdd))
	if err == nil {
		t.Fatal("expected error for odd-mode with even order")
	}

	d, err := NewDistortion(48000)
	if err != nil {
		t.Fatalf("NewDistortion() error = %v", err)
	}

	err = d.SetChebyshevHarmonicMode(ChebyshevHarmonicEven)
	if err == nil {
		t.Fatal("expected parity validation error for default odd order")
	}
}

func TestDistortionMixZeroPassthrough(t *testing.T) {
	d, err := NewDistortion(48000,
		WithDistortionMode(DistortionModeHardClip),
		WithDistortionDrive(10),
		WithDistortionMix(0),
	)
	if err != nil {
		t.Fatalf("NewDistortion() error = %v", err)
	}

	for _, in := range []float64{-1.2, -0.5, 0, 0.4, 1.3} {
		out := d.ProcessSample(in)
		if math.Abs(out-in) > 1e-12 {
			t.Fatalf("mix=0 passthrough mismatch: in=%g out=%g", in, out)
		}
	}
}

func TestDistortionHardClipTransferCurve(t *testing.T) {
	d, err := NewDistortion(48000,
		WithDistortionMode(DistortionModeHardClip),
		WithDistortionDrive(1),
		WithDistortionClipLevel(0.5),
		WithDistortionMix(1),
	)
	if err != nil {
		t.Fatalf("NewDistortion() error = %v", err)
	}

	cases := []struct {
		in   float64
		want float64
	}{
		{-2, -1},
		{-0.5, -1},
		{-0.25, -0.5},
		{0, 0},
		{0.25, 0.5},
		{0.5, 1},
		{2, 1},
	}

	for _, tc := range cases {
		got := d.ProcessSample(tc.in)
		if math.Abs(got-tc.want) > 1e-12 {
			t.Fatalf("hard clip mismatch: in=%g got=%g want=%g", tc.in, got, tc.want)
		}
	}
}

func TestDistortionAllFormulaModesFiniteAndBounded(t *testing.T) {
	modes := []DistortionMode{
		DistortionModeWaveshaper1,
		DistortionModeWaveshaper2,
		DistortionModeWaveshaper3,
		DistortionModeWaveshaper4,
		DistortionModeWaveshaper5,
		DistortionModeWaveshaper6,
		DistortionModeWaveshaper7,
		DistortionModeWaveshaper8,
		DistortionModeSaturate,
		DistortionModeSaturate2,
		DistortionModeSoftSat,
	}

	for _, mode := range modes {
		d, err := NewDistortion(48000,
			WithDistortionMode(mode),
			WithDistortionDrive(4),
			WithDistortionShape(0.7),
			WithDistortionMix(1),
		)
		if err != nil {
			t.Fatalf("NewDistortion(mode=%d) error = %v", mode, err)
		}

		for _, in := range []float64{-2, -1, -0.2, 0, 0.2, 1, 2} {
			got := d.ProcessSample(in)
			if math.IsNaN(got) || math.IsInf(got, 0) {
				t.Fatalf("mode=%d produced non-finite output for in=%g", mode, in)
			}

			if math.Abs(got) > 1.0000001 {
				t.Fatalf("mode=%d exceeded output bound: in=%g out=%g", mode, in, got)
			}
		}
	}
}

func TestDistortionTanhApproxCloseToExact(t *testing.T) {
	exact, err := NewDistortion(48000,
		WithDistortionMode(DistortionModeTanh),
		WithDistortionApproxMode(DistortionApproxExact),
		WithDistortionDrive(2.5),
		WithDistortionMix(1),
	)
	if err != nil {
		t.Fatalf("NewDistortion(exact) error = %v", err)
	}

	approx, err := NewDistortion(48000,
		WithDistortionMode(DistortionModeTanh),
		WithDistortionApproxMode(DistortionApproxPolynomial),
		WithDistortionDrive(2.5),
		WithDistortionMix(1),
	)
	if err != nil {
		t.Fatalf("NewDistortion(approx) error = %v", err)
	}

	for i := -100; i <= 100; i++ {
		in := float64(i) / 50.0
		yExact := exact.ProcessSample(in)

		yApprox := approx.ProcessSample(in)
		if diff := math.Abs(yExact - yApprox); diff > 0.05 {
			t.Fatalf("approx too far from exact at in=%g: diff=%g", in, diff)
		}
	}
}

func TestDistortionChebyshevHarmonicBalance(t *testing.T) {
	d, err := NewDistortion(48000,
		WithDistortionMode(DistortionModeChebyshev),
		WithChebyshevOrder(3),
		WithChebyshevHarmonicMode(ChebyshevHarmonicOdd),
		WithDistortionDrive(1),
		WithDistortionMix(1),
	)
	if err != nil {
		t.Fatalf("NewDistortion() error = %v", err)
	}

	const n = 4096

	in := make([]float64, n)

	out := make([]float64, n)
	for i := range n {
		in[i] = math.Cos(2 * math.Pi * float64(i) / float64(n))
		out[i] = d.ProcessSample(in[i])
	}

	amp1 := harmonicAmplitude(out, 1)
	amp2 := harmonicAmplitude(out, 2)
	amp3 := harmonicAmplitude(out, 3)

	if amp3 <= amp1 {
		t.Fatalf("expected 3rd harmonic dominance: h1=%g h3=%g", amp1, amp3)
	}

	if amp2 > amp3*0.1 {
		t.Fatalf("expected low even harmonic content in odd mode: h2=%g h3=%g", amp2, amp3)
	}
}

func TestDistortionChebyshevDCBypassReducesDC(t *testing.T) {
	dNoDC, err := NewDistortion(48000,
		WithDistortionMode(DistortionModeChebyshev),
		WithChebyshevOrder(2),
		WithChebyshevHarmonicMode(ChebyshevHarmonicEven),
		WithChebyshevDCBypass(false),
		WithDistortionMix(1),
	)
	if err != nil {
		t.Fatalf("NewDistortion(no dc bypass) error = %v", err)
	}

	dWithDC, err := NewDistortion(48000,
		WithDistortionMode(DistortionModeChebyshev),
		WithChebyshevOrder(2),
		WithChebyshevHarmonicMode(ChebyshevHarmonicEven),
		WithChebyshevDCBypass(true),
		WithDistortionMix(1),
	)
	if err != nil {
		t.Fatalf("NewDistortion(with dc bypass) error = %v", err)
	}

	const n = 4000

	var sumNoDC, sumWithDC float64

	for i := range n {
		x := math.Cos(2 * math.Pi * float64(i) / 128)
		sumNoDC += dNoDC.ProcessSample(x)
		sumWithDC += dWithDC.ProcessSample(x)
	}

	meanNoDC := sumNoDC / n

	meanWithDC := sumWithDC / n
	if math.Abs(meanWithDC) >= math.Abs(meanNoDC)*0.2 {
		t.Fatalf("expected DC bypass to reduce mean strongly: no_dc=%g with_dc=%g", meanNoDC, meanWithDC)
	}
}

func TestDistortionProcessInPlace(t *testing.T) {
	d1, err := NewDistortion(48000,
		WithDistortionMode(DistortionModeSoftSat),
		WithDistortionDrive(3),
		WithDistortionMix(1),
	)
	if err != nil {
		t.Fatalf("NewDistortion() error = %v", err)
	}

	d2, err := NewDistortion(48000,
		WithDistortionMode(DistortionModeSoftSat),
		WithDistortionDrive(3),
		WithDistortionMix(1),
	)
	if err != nil {
		t.Fatalf("NewDistortion() error = %v", err)
	}

	buf := make([]float64, 256)
	for i := range buf {
		buf[i] = math.Sin(2 * math.Pi * float64(i) / 53)
	}

	want := make([]float64, len(buf))
	for i := range buf {
		want[i] = d1.ProcessSample(buf[i])
	}

	got := append([]float64(nil), buf...)
	d2.ProcessInPlace(got)

	for i := range got {
		if math.Abs(got[i]-want[i]) > 1e-12 {
			t.Fatalf("ProcessInPlace mismatch at %d: got=%g want=%g", i, got[i], want[i])
		}
	}
}

func TestChebyshevWeightsDefaultIsLegacyTN(t *testing.T) {
	// With all-zero weights (default), ProcessSample should equal T_N(x)*gain.
	// T_3(0.5) = 4*(0.5)^3 - 3*(0.5) = 0.5 - 1.5 = -1.0; clampUnitDist(-1.0) = -1.0
	d, err := NewDistortion(48000,
		WithDistortionMode(DistortionModeChebyshev),
		WithChebyshevOrder(3),
		WithDistortionDrive(1),
		WithDistortionMix(1),
		WithDistortionOutputLevel(1),
		WithChebyshevGainLevel(1),
	)
	if err != nil {
		t.Fatalf("NewDistortion() error = %v", err)
	}

	got := d.ProcessSample(0.5)
	want := -1.0 // clampUnitDist(T_3(0.5)) = clampUnitDist(-1.0) = -1.0

	if math.Abs(got-want) > 1e-10 {
		t.Fatalf("legacy T_N mismatch: got=%g want=%g", got, want)
	}
}

func TestChebyshevWeightsAdditiveBlend(t *testing.T) {
	const (
		x   = 0.3
		tol = 1e-10
	)

	baseOpts := []DistortionOption{
		WithDistortionMode(DistortionModeChebyshev),
		WithChebyshevOrder(3),
		WithDistortionDrive(1),
		WithDistortionMix(1),
		WithDistortionOutputLevel(1),
		WithChebyshevGainLevel(1),
	}

	cases := []struct {
		weights []float64
		want    float64
		label   string
	}{
		// weights=[1,0,0] -> T_1(x) = x
		{[]float64{1, 0, 0}, x, "T_1"},
		// weights=[0,1,0] -> T_2(x) = 2x^2-1
		{[]float64{0, 1, 0}, 2*x*x - 1, "T_2"},
		// weights=[0,0,1] -> T_3(x) = 4x^3-3x
		{[]float64{0, 0, 1}, clampUnitDistHelper(4*x*x*x - 3*x), "T_3"},
	}

	for _, tc := range cases {
		opts := append(baseOpts, WithChebyshevWeights(tc.weights))

		d, err := NewDistortion(48000, opts...)
		if err != nil {
			t.Fatalf("[%s] NewDistortion() error = %v", tc.label, err)
		}

		got := d.ProcessSample(x)
		if math.Abs(got-tc.want) > tol {
			t.Fatalf("[%s] mismatch: got=%g want=%g", tc.label, got, tc.want)
		}
	}
}

// clampUnitDistHelper is a test helper mirroring clampUnitDist for expected value calculation.
func clampUnitDistHelper(v float64) float64 {
	if v < -1 {
		return -1
	}

	if v > 1 {
		return 1
	}

	return v
}

func TestSetChebyshevWeightsValidation(t *testing.T) {
	d, err := NewDistortion(48000,
		WithDistortionMode(DistortionModeChebyshev),
		WithChebyshevOrder(3),
		WithDistortionDrive(1),
		WithDistortionMix(1),
		WithDistortionOutputLevel(1),
		WithChebyshevGainLevel(1),
	)
	if err != nil {
		t.Fatalf("NewDistortion() error = %v", err)
	}

	// Too many weights (>16) must return an error.
	err = d.SetChebyshevWeights(make([]float64, 17))
	if err == nil {
		t.Fatal("expected error for >16 weights")
	}

	// NaN weight must return an error.
	err = d.SetChebyshevWeights([]float64{math.NaN()})
	if err == nil {
		t.Fatal("expected error for NaN weight")
	}

	// Inf weight must return an error.
	err = d.SetChebyshevWeights([]float64{math.Inf(1)})
	if err == nil {
		t.Fatal("expected error for Inf weight")
	}

	// Valid 3-element weights must succeed.
	err = d.SetChebyshevWeights([]float64{1, 0, 0})
	if err != nil {
		t.Fatalf("unexpected error for valid weights: %v", err)
	}

	// Verify the weights are applied: with [1,0,0] output should equal T_1(x)=x.
	const x = 0.4

	got := d.ProcessSample(x)
	want := x // T_1(x)*gain=1 with mix=1, output=1, drive=1

	if math.Abs(got-want) > 1e-10 {
		t.Fatalf("weights not applied: got=%g want=%g", got, want)
	}
}

func TestChebyshevWeightsZeroAfterSet(t *testing.T) {
	// Create distortion with order=3, set weights to [1,0,0] (selects T_1).
	d, err := NewDistortion(48000,
		WithDistortionMode(DistortionModeChebyshev),
		WithChebyshevOrder(3),
		WithDistortionDrive(1),
		WithDistortionMix(1),
		WithDistortionOutputLevel(1),
		WithChebyshevGainLevel(1),
	)
	if err != nil {
		t.Fatalf("NewDistortion() error = %v", err)
	}

	err = d.SetChebyshevWeights([]float64{1, 0, 0})
	if err != nil {
		t.Fatalf("SetChebyshevWeights([1,0,0]) error = %v", err)
	}

	const (
		x   = 0.5
		tol = 1e-10
	)

	// With weights=[1,0,0], output should equal T_1(0.5)=0.5.
	got := d.ProcessSample(x)
	if math.Abs(got-x) > tol {
		t.Fatalf("expected T_1 output %g, got %g", x, got)
	}

	// Reset weights to all zeros -> legacy T_3 path.
	err = d.SetChebyshevWeights([]float64{0, 0, 0})
	if err != nil {
		t.Fatalf("SetChebyshevWeights([0,0,0]) error = %v", err)
	}

	// Legacy T_3(0.5) = -1.0
	got = d.ProcessSample(x)
	want := -1.0

	if math.Abs(got-want) > tol {
		t.Fatalf("expected legacy T_3 output %g after weight reset, got %g", want, got)
	}
}

func harmonicAmplitude(x []float64, k int) float64 {
	var re, im float64

	n := float64(len(x))
	for i := range x {
		phase := 2 * math.Pi * float64(k) * float64(i) / n
		re += x[i] * math.Cos(phase)
		im -= x[i] * math.Sin(phase)
	}

	return 2 * math.Hypot(re, im) / n
}
