package effects

import (
	"math"
	"testing"
)

func TestDistortionValidation(t *testing.T) {
	if _, err := NewDistortion(0); err == nil {
		t.Fatal("expected error for invalid sample rate")
	}

	if _, err := NewDistortion(48000, WithDistortionDrive(100)); err == nil {
		t.Fatal("expected error for invalid drive")
	}

	if _, err := NewDistortion(48000, WithDistortionMode(DistortionMode(999))); err == nil {
		t.Fatal("expected error for invalid mode")
	}

	if _, err := NewDistortion(48000,
		WithDistortionMode(DistortionModeChebyshev),
		WithChebyshevOrder(4),
		WithChebyshevHarmonicMode(ChebyshevHarmonicOdd)); err == nil {
		t.Fatal("expected error for odd-mode with even order")
	}

	d, err := NewDistortion(48000)
	if err != nil {
		t.Fatalf("NewDistortion() error = %v", err)
	}

	if err := d.SetChebyshevHarmonicMode(ChebyshevHarmonicEven); err == nil {
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
	for i := 0; i < n; i++ {
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

	for i := 0; i < n; i++ {
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
