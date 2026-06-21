package dynamics

import (
	"math"
	"testing"
)

// dbToAmpRef and ampToDBRef are independent dB conversions used by the test
// references, deliberately not sharing the package helpers.
func dbToAmpRef(db float64) float64 { return math.Pow(10, db/20.0) }

func ampToDBRef(amp float64) float64 {
	if amp <= 0 {
		return math.Inf(-1)
	}

	return 20.0 * math.Log10(amp)
}

// legacyCompressorOutputDB mirrors TCustomDynamicProcessor.CharacteristicCurve_dB
// for the simple (hard-knee) compressor in DAV_DspDynamics.pas:
//
//	CharacteristicCurve(in) = TranslatePeakToGain(|in|) * in
//	TranslatePeakToGain(p)  = p < thr ? makeup0 : makeup1 * p^(legacyRatio-1)
//	makeup0 = 1, makeup1 = thr^(1-legacyRatio), legacyRatio = 1/ratio
func legacyCompressorOutputDB(inputDB, thresholdDB, ratio float64) float64 {
	level := dbToAmpRef(inputDB)
	threshold := dbToAmpRef(thresholdDB)
	legacyRatio := 1.0 / ratio
	makeup1 := math.Pow(threshold, 1.0-legacyRatio)

	gain := 1.0
	if level >= threshold {
		gain = makeup1 * math.Pow(level, legacyRatio-1.0)
	}

	return ampToDBRef(level * gain)
}

// legacyExpanderOutputDB is the analytic downward-expander transfer law that the
// Go gain computer implements: below threshold the gain rolls off as
// (level/threshold)^(ratio-1), clamped to the range floor.
func legacyExpanderOutputDB(inputDB, thresholdDB, ratio, rangeDB float64) float64 {
	level := dbToAmpRef(inputDB)
	threshold := dbToAmpRef(thresholdDB)
	rangeLin := dbToAmpRef(rangeDB)

	gain := 1.0
	if level < threshold {
		gain = math.Pow(level/threshold, ratio-1.0)
		if gain < rangeLin {
			gain = rangeLin
		}
	}

	return ampToDBRef(level * gain)
}

func TestStaticCurveCompressorLegacyParity(t *testing.T) {
	const sampleRate = 48000.0

	thresholds := []float64{-30, -18, -6}
	ratios := []float64{1.5, 2, 4, 8}

	for _, thr := range thresholds {
		for _, ratio := range ratios {
			c, err := NewCompressor(sampleRate)
			if err != nil {
				t.Fatalf("NewCompressor: %v", err)
			}

			mustNoErr(t, c.SetAutoMakeup(false))
			mustNoErr(t, c.SetMakeupGain(0))
			mustNoErr(t, c.SetKnee(0))
			mustNoErr(t, c.SetThreshold(thr))
			mustNoErr(t, c.SetRatio(ratio))

			curve, err := StaticCurve(c, -60, 0, 1)
			if err != nil {
				t.Fatalf("StaticCurve: %v", err)
			}

			for _, pt := range curve {
				want := legacyCompressorOutputDB(pt.InputDB, thr, ratio)
				if math.Abs(pt.OutputDB-want) > 1e-9 {
					t.Fatalf("thr=%g ratio=%g in=%g: out=%.12f want=%.12f", thr, ratio, pt.InputDB, pt.OutputDB, want)
				}

				if math.Abs(pt.GainReductionDB-(pt.OutputDB-pt.InputDB)) > 1e-12 {
					t.Fatalf("gain-reduction mismatch at in=%g: gr=%.12f out-in=%.12f", pt.InputDB, pt.GainReductionDB, pt.OutputDB-pt.InputDB)
				}

				// Above threshold a compressor only ever attenuates.
				if pt.InputDB > thr+1e-9 && pt.GainReductionDB > 1e-9 {
					t.Fatalf("expected attenuation above threshold at in=%g: gr=%g", pt.InputDB, pt.GainReductionDB)
				}
			}
		}
	}
}

func TestStaticCurveExpanderLegacyParity(t *testing.T) {
	const sampleRate = 48000.0

	const rangeDB = -60.0

	thresholds := []float64{-50, -35, -20}
	ratios := []float64{1.5, 2, 4}

	for _, thr := range thresholds {
		for _, ratio := range ratios {
			e, err := NewExpander(sampleRate)
			if err != nil {
				t.Fatalf("NewExpander: %v", err)
			}

			mustNoErr(t, e.SetKnee(0))
			mustNoErr(t, e.SetThreshold(thr))
			mustNoErr(t, e.SetRatio(ratio))
			mustNoErr(t, e.SetRange(rangeDB))

			curve, err := StaticCurve(e, -90, 0, 1)
			if err != nil {
				t.Fatalf("StaticCurve: %v", err)
			}

			for _, pt := range curve {
				want := legacyExpanderOutputDB(pt.InputDB, thr, ratio, rangeDB)
				if math.Abs(pt.OutputDB-want) > 1e-9 {
					t.Fatalf("thr=%g ratio=%g in=%g: out=%.12f want=%.12f", thr, ratio, pt.InputDB, pt.OutputDB, want)
				}

				// Below threshold a downward expander only ever attenuates.
				if pt.InputDB < thr-1e-9 && pt.GainReductionDB > 1e-9 {
					t.Fatalf("expected attenuation below threshold at in=%g: gr=%g", pt.InputDB, pt.GainReductionDB)
				}
			}
		}
	}
}

// TestStaticCurveCompressorKneeSweep validates the soft-knee curve against the
// legacy hard-knee asymptote: knee=0 reproduces legacy exactly, and knee>0
// produces a continuous, monotonic transition that re-joins the legacy curve far
// from the threshold.
func TestStaticCurveCompressorKneeSweep(t *testing.T) {
	const sampleRate = 48000.0

	const (
		thr   = -24.0
		ratio = 4.0
	)

	for _, knee := range []float64{0, 6, 12, 18} {
		c, err := NewCompressor(sampleRate)
		if err != nil {
			t.Fatalf("NewCompressor: %v", err)
		}

		mustNoErr(t, c.SetAutoMakeup(false))
		mustNoErr(t, c.SetMakeupGain(0))
		mustNoErr(t, c.SetThreshold(thr))
		mustNoErr(t, c.SetRatio(ratio))
		mustNoErr(t, c.SetKnee(knee))

		curve, err := StaticCurve(c, -72, 0, 0.5)
		if err != nil {
			t.Fatalf("StaticCurve: %v", err)
		}

		var prevOut float64

		for i, pt := range curve {
			// Output is monotonic non-decreasing in input.
			if i > 0 && pt.OutputDB+1e-9 < prevOut {
				t.Fatalf("knee=%g: output not monotonic at in=%g: %.6f < %.6f", knee, pt.InputDB, pt.OutputDB, prevOut)
			}

			prevOut = pt.OutputDB

			// With no makeup gain a compressor never amplifies.
			if pt.OutputDB > pt.InputDB+1e-9 {
				t.Fatalf("knee=%g in=%g: unexpected gain out=%.6f > in=%.6f", knee, pt.InputDB, pt.OutputDB, pt.InputDB)
			}

			// Outside the soft knee (input below thr-knee/2 or above thr+knee/2)
			// the curve must rejoin the legacy hard-knee curve exactly.
			if pt.InputDB < thr-knee/2-1e-6 || pt.InputDB > thr+knee/2+1e-6 {
				want := legacyCompressorOutputDB(pt.InputDB, thr, ratio)
				if math.Abs(pt.OutputDB-want) > 1e-9 {
					t.Fatalf("knee=%g outside-knee in=%g: out=%.12f want=%.12f", knee, pt.InputDB, pt.OutputDB, want)
				}
			}
		}
	}
}

func TestStaticCurveIsNonMutating(t *testing.T) {
	newDriven := func() *Compressor {
		c, err := NewCompressor(48000)
		if err != nil {
			t.Fatalf("NewCompressor: %v", err)
		}

		mustNoErr(t, c.SetThreshold(-18))
		mustNoErr(t, c.SetRatio(4))

		// Drive the detector partway to steady state so the envelope is mid-flight.
		for range 1000 {
			_ = c.ProcessSample(0.5)
		}

		return c
	}

	// Two identically-driven processors: run StaticCurve on one only, then feed
	// both the same sample. If StaticCurve mutated detector state the next
	// streaming sample would diverge.
	withCurve := newDriven()
	reference := newDriven()

	envBefore := withCurve.peakLevel

	if _, err := StaticCurve(withCurve, -60, 0, 1); err != nil {
		t.Fatalf("StaticCurve: %v", err)
	}

	if withCurve.peakLevel != envBefore {
		t.Fatalf("StaticCurve mutated envelope: before=%g after=%g", envBefore, withCurve.peakLevel)
	}

	got := withCurve.ProcessSample(0.5)
	want := reference.ProcessSample(0.5)

	if math.Abs(got-want) != 0 {
		t.Fatalf("streaming output changed after StaticCurve: got=%.15f want=%.15f", got, want)
	}
}

func TestStaticCurveMultibandBand(t *testing.T) {
	mc, err := NewMultibandCompressor([]float64{250, 4000}, 4, 48000)
	if err != nil {
		t.Fatalf("NewMultibandCompressor: %v", err)
	}

	const (
		band  = 1
		thr   = -20.0
		ratio = 3.0
	)

	mustNoErr(t, mc.SetBandAutoMakeup(band, false))
	mustNoErr(t, mc.SetBandMakeupGain(band, 0))
	mustNoErr(t, mc.SetBandKnee(band, 0))
	mustNoErr(t, mc.SetBandThreshold(band, thr))
	mustNoErr(t, mc.SetBandRatio(band, ratio))

	curve, err := mc.BandStaticCurve(band, -60, 0, 1)
	if err != nil {
		t.Fatalf("BandStaticCurve: %v", err)
	}

	for _, pt := range curve {
		want := legacyCompressorOutputDB(pt.InputDB, thr, ratio)
		if math.Abs(pt.OutputDB-want) > 1e-9 {
			t.Fatalf("band=%d in=%g: out=%.12f want=%.12f", band, pt.InputDB, pt.OutputDB, want)
		}
	}

	if _, err := mc.BandStaticCurve(99, -60, 0, 1); err == nil {
		t.Fatalf("expected error for out-of-range band index")
	}
}

func TestStaticCurveValidation(t *testing.T) {
	c, err := NewCompressor(48000)
	if err != nil {
		t.Fatalf("NewCompressor: %v", err)
	}

	cases := []struct {
		name           string
		min, max, step float64
	}{
		{"zero step", -60, 0, 0},
		{"negative step", -60, 0, -1},
		{"min above max", 0, -60, 1},
		{"nan bound", math.NaN(), 0, 1},
		{"inf step", -60, 0, math.Inf(1)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := StaticCurve(c, tc.min, tc.max, tc.step); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}

	if _, err := StaticCurve(nil, -60, 0, 1); err == nil {
		t.Fatalf("expected error for nil processor")
	}
}

func TestStaticCurveEndpointsInclusive(t *testing.T) {
	c, err := NewCompressor(48000)
	if err != nil {
		t.Fatalf("NewCompressor: %v", err)
	}

	// Step that does not divide the range evenly must still include both ends.
	curve, err := StaticCurve(c, -60, 0, 7)
	if err != nil {
		t.Fatalf("StaticCurve: %v", err)
	}

	if curve[0].InputDB != -60 {
		t.Fatalf("first point not min: %g", curve[0].InputDB)
	}

	if last := curve[len(curve)-1].InputDB; last != 0 {
		t.Fatalf("last point not max: %g", last)
	}
}

func mustNoErr(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
