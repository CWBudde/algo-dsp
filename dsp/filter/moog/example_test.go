package moog_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/moog"
)

func ExampleNew_subtractiveSweep() {
	f, err := moog.New(48000,
		moog.WithVariant(moog.VariantHuovilainen),
		moog.WithCutoffHz(300),
		moog.WithResonance(1.4),
		moog.WithDrive(2.2),
		moog.WithOversampling(4),
	)
	if err != nil {
		panic(err)
	}

	out := make([]float64, 8)
	for i := range out {
		cutoff := 300 + float64(i)*500
		if err := f.SetCutoffHz(cutoff); err != nil {
			panic(err)
		}

		saw := 2*math.Mod(float64(i)/8, 1) - 1
		out[i] = f.ProcessSample(saw)
	}

	fmt.Printf("%.6f %.6f %.6f\n", out[0], out[1], out[2])
	// Output:
	// -0.000000 -0.000004 -0.000128
}

func ExampleNew_resonanceEmphasis() {
	lowRes, err := moog.New(48000,
		moog.WithVariant(moog.VariantHuovilainen),
		moog.WithCutoffHz(1200),
		moog.WithResonance(0.5),
		moog.WithNormalizeOutput(false),
	)
	if err != nil {
		panic(err)
	}

	highRes, err := moog.New(48000,
		moog.WithVariant(moog.VariantHuovilainen),
		moog.WithCutoffHz(1200),
		moog.WithResonance(3.2),
		moog.WithNormalizeOutput(false),
	)
	if err != nil {
		panic(err)
	}

	peakLow := ringPeak(lowRes, 1024)
	peakHigh := ringPeak(highRes, 1024)
	fmt.Printf("%.3f %.3f\n", peakLow, peakHigh)
	// Output:
	// 0.037 0.056
}

func ExampleNew_drivenSaturationComparison() {
	exact, err := moog.New(48000,
		moog.WithVariant(moog.VariantClassic),
		moog.WithCutoffHz(5000),
		moog.WithResonance(0),
		moog.WithDrive(8),
		moog.WithNormalizeOutput(false),
	)
	if err != nil {
		panic(err)
	}

	lightweight, err := moog.New(48000,
		moog.WithVariant(moog.VariantClassicLightweight),
		moog.WithCutoffHz(5000),
		moog.WithResonance(0),
		moog.WithDrive(8),
		moog.WithNormalizeOutput(false),
	)
	if err != nil {
		panic(err)
	}

	x := 0.75
	yExact := exact.ProcessSample(x)
	yLight := lightweight.ProcessSample(x)
	fmt.Printf("%.6f %.6f\n", yExact, yLight)
	// Output:
	// 4.798519 4.802974
}

func ExampleNew_zdfHighAccuracy() {
	// VariantZDF uses Zero-Delay Feedback with Newton-Raphson iteration
	// for the most accurate cutoff tuning and self-oscillation behavior.
	f, err := moog.New(48000,
		moog.WithVariant(moog.VariantZDF),
		moog.WithCutoffHz(2000),
		moog.WithResonance(2.5),
		moog.WithDrive(3.0),
		moog.WithNewtonIterations(4),
	)
	if err != nil {
		panic(err)
	}

	out := make([]float64, 8)
	for i := range out {
		saw := 2*math.Mod(float64(i)/8, 1) - 1
		out[i] = f.ProcessSample(saw)
	}

	fmt.Printf("%.6f %.6f %.6f\n", out[0], out[1], out[2])
	// Output:
	// -0.000140 -0.001099 -0.004210
}

func ringPeak(f *moog.Filter, n int) float64 {
	peak := 0.0
	for i := 0; i < n; i++ {
		x := 0.0
		if i == 0 {
			x = 1.0
		}

		y := f.ProcessSample(x)
		a := math.Abs(y)
		if a > peak {
			peak = a
		}
	}

	return peak
}
