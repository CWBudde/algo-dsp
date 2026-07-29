package webdemo

import (
	"math"
	"testing"
)

// TestSanitizeSpectralPitchFrameSize pins the frame-size snapping rule.
//
// The implementation names its locals confusingly (it computes the *upper*
// power of two into a variable called `lower`, then shifts it down), so the
// behaviour is worth stating explicitly: clamp to [256, 4096], then snap to the
// nearer power of two, preferring the lower one on a tie.
func TestSanitizeSpectralPitchFrameSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   int
		want int
	}{
		{"zero clamps up", 0, 256},
		{"negative clamps up", -1000, 256},
		{"below range clamps up", 100, 256},
		{"above range clamps down", 100000, 4096},
		{"exact power of two is kept", 256, 256},
		{"exact power of two is kept (512)", 512, 512},
		{"exact power of two is kept (1024)", 1024, 1024},
		{"exact power of two is kept (2048)", 2048, 2048},
		{"exact power of two is kept (4096)", 4096, 4096},
		{"just above a power of two snaps down", 513, 512},
		{"just below a power of two snaps up", 1023, 1024},
		{"midpoint ties snap down", 768, 512},
		{"just above the midpoint snaps up", 769, 1024},
		{"just below the midpoint snaps down", 767, 512},
		{"near the top of the range", 4095, 4096},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := sanitizeSpectralPitchFrameSize(tc.in)
			if got != tc.want {
				t.Errorf("sanitizeSpectralPitchFrameSize(%d) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// TestSanitizeSpectralPitchFrameSizeAlwaysValid is the property the FFT plan
// depends on: whatever arrives from JS, the result is a power of two inside the
// supported range.
func TestSanitizeSpectralPitchFrameSizeAlwaysValid(t *testing.T) {
	t.Parallel()

	check := func(n int) {
		got := sanitizeSpectralPitchFrameSize(n)

		if got < 256 || got > 4096 {
			t.Fatalf("sanitizeSpectralPitchFrameSize(%d) = %d, outside [256, 4096]", n, got)
		}

		if got&(got-1) != 0 {
			t.Fatalf("sanitizeSpectralPitchFrameSize(%d) = %d, not a power of two", n, got)
		}
	}

	for n := -10; n <= 5000; n++ {
		check(n)
	}

	for _, n := range []int{math.MaxInt32, math.MinInt32, math.MaxInt, math.MinInt} {
		check(n)
	}
}

// TestSetEffectsClampsOutOfRangeValues feeds deliberately absurd values through
// SetEffects and checks the stored parameters land inside their documented
// ranges. These arrive unvalidated from the browser.
func TestSetEffectsClampsOutOfRangeValues(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)

	huge := 1e9

	err := e.SetEffects(EffectsParams{
		ChorusMix:            huge,
		ChorusDepth:          huge,
		ChorusSpeedHz:        huge,
		ChorusStages:         999,
		FlangerRateHz:        -huge,
		FlangerDepth:         huge,
		FlangerBaseDelay:     huge,
		FlangerFeedback:      huge,
		FlangerMix:           -huge,
		RingModCarrierHz:     huge,
		RingModMix:           huge,
		BitCrusherBitDepth:   huge,
		BitCrusherDownsample: 99999,
		BitCrusherMix:        -huge,
		WidenerWidth:         huge,
		WidenerMix:           huge,
		PhaserRateHz:         huge,
		PhaserMinFreqHz:      -huge,
		PhaserMaxFreqHz:      -huge,
		PhaserStages:         -5,
		ReverbModel:          "not-a-model",
	})
	if err != nil {
		t.Fatalf("SetEffects: %v", err)
	}

	got := e.effects

	checks := []struct {
		name     string
		value    float64
		min, max float64
	}{
		{"ChorusMix", got.ChorusMix, 0, 1},
		{"ChorusDepth", got.ChorusDepth, 0, 0.01},
		{"ChorusSpeedHz", got.ChorusSpeedHz, 0.05, 5},
		{"FlangerRateHz", got.FlangerRateHz, 0.05, 5},
		{"FlangerDepth", got.FlangerDepth, 0, 0.0099},
		{"FlangerBaseDelay", got.FlangerBaseDelay, 0.0001, 0.01},
		{"FlangerFeedback", got.FlangerFeedback, -0.99, 0.99},
		{"FlangerMix", got.FlangerMix, 0, 1},
		{"RingModCarrierHz", got.RingModCarrierHz, 1, e.sampleRate * 0.49},
		{"RingModMix", got.RingModMix, 0, 1},
		{"BitCrusherBitDepth", got.BitCrusherBitDepth, 1, 32},
		{"BitCrusherMix", got.BitCrusherMix, 0, 1},
		{"WidenerWidth", got.WidenerWidth, 0, 4},
		{"WidenerMix", got.WidenerMix, 0, 1},
		{"PhaserRateHz", got.PhaserRateHz, 0.05, 5},
		{"PhaserMinFreqHz", got.PhaserMinFreqHz, 20, e.sampleRate * 0.45},
	}

	for _, c := range checks {
		if c.value < c.min || c.value > c.max {
			t.Errorf("%s = %v, want within [%v, %v]", c.name, c.value, c.min, c.max)
		}
	}

	if got.ChorusStages < 1 || got.ChorusStages > 6 {
		t.Errorf("ChorusStages = %d, want within [1, 6]", got.ChorusStages)
	}

	if got.BitCrusherDownsample < 1 || got.BitCrusherDownsample > 256 {
		t.Errorf("BitCrusherDownsample = %d, want within [1, 256]", got.BitCrusherDownsample)
	}

	if got.PhaserStages < 1 {
		t.Errorf("PhaserStages = %d, want at least 1", got.PhaserStages)
	}

	// The phaser sweep must stay ordered, or the modulation range inverts.
	if got.PhaserMaxFreqHz <= got.PhaserMinFreqHz {
		t.Errorf("PhaserMaxFreqHz (%v) must exceed PhaserMinFreqHz (%v)",
			got.PhaserMaxFreqHz, got.PhaserMinFreqHz)
	}

	// An unrecognised reverb model falls back rather than leaving the engine
	// with no reverb implementation selected.
	if got.ReverbModel != "freeverb" && got.ReverbModel != reverbModelFDN {
		t.Errorf("ReverbModel = %q, want a known model", got.ReverbModel)
	}
}

// TestSetEffectsFlangerDelayBudget covers the coupled constraint: base delay
// plus depth must stay inside the delay line, or the modulation would read
// past its end.
func TestSetEffectsFlangerDelayBudget(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)

	err := e.SetEffects(EffectsParams{
		FlangerBaseDelay: 0.009,
		FlangerDepth:     0.009,
		ReverbModel:      "freeverb",
	})
	if err != nil {
		t.Fatalf("SetEffects: %v", err)
	}

	total := e.effects.FlangerBaseDelay + e.effects.FlangerDepth
	if total > 0.01+1e-12 {
		t.Errorf("base delay + depth = %v, want at most 0.01", total)
	}

	if e.effects.FlangerDepth < 0 {
		t.Errorf("FlangerDepth = %v, want it non-negative", e.effects.FlangerDepth)
	}
}

func TestSetEffectsAcceptsZeroValue(t *testing.T) {
	t.Parallel()

	// The zero value is what a browser sends before any UI has been touched.
	e := newTestEngine(t)

	if err := e.SetEffects(EffectsParams{}); err != nil {
		t.Fatalf("SetEffects on the zero value: %v", err)
	}

	// The engine must still render finite audio afterwards.
	block := make([]float32, 512)
	e.Render(block)

	for i, v := range block {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Fatalf("sample %d is not finite after a zero-value SetEffects", i)
		}
	}
}

// TestSetEffectsRejectsMalformedGraph checks that a bad graph string surfaces
// as an error instead of being silently ignored.
func TestSetEffectsRejectsMalformedGraph(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)

	err := e.SetEffects(EffectsParams{
		ReverbModel:    "freeverb",
		ChainGraphJSON: "{not valid json",
	})
	if err == nil {
		t.Fatal("expected an error for a malformed chain graph")
	}
}

// TestSetEffectsSkipsRedundantGraphReload documents the caching added to keep
// parameter changes off the graph-recompilation path: an unchanged graph string
// must not be reloaded on every call.
func TestSetEffectsSkipsRedundantGraphReload(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)

	const graph = `{"nodes":[{"id":"_input","type":"_input"},{"id":"_output","type":"_output"}],` +
		`"connections":[{"from":"_input","to":"_output"}]}`

	params := EffectsParams{ReverbModel: "freeverb", ChainGraphJSON: graph}

	if err := e.SetEffects(params); err != nil {
		t.Fatalf("first SetEffects: %v", err)
	}

	if e.loadedGraphJSON != graph {
		t.Fatalf("loadedGraphJSON = %q, want the graph to be recorded", e.loadedGraphJSON)
	}

	// A parameter-only change keeps the same graph string.
	params.ChorusMix = 0.5
	if err := e.SetEffects(params); err != nil {
		t.Fatalf("second SetEffects: %v", err)
	}

	if e.loadedGraphJSON != graph {
		t.Errorf("loadedGraphJSON = %q, want it unchanged", e.loadedGraphJSON)
	}

	// Changing the graph does take effect.
	params.ChainGraphJSON = `{"nodes":[{"id":"_input","type":"_input"},` +
		`{"id":"_output","type":"_output"}],"connections":[]}`
	if err := e.SetEffects(params); err != nil {
		t.Fatalf("third SetEffects: %v", err)
	}

	if e.loadedGraphJSON != params.ChainGraphJSON {
		t.Errorf("loadedGraphJSON = %q, want the new graph", e.loadedGraphJSON)
	}
}

func TestCompressorAndLimiterCurvesAreMonotonic(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)

	if err := e.SetCompressor(CompressorParams{
		Enabled:     true,
		ThresholdDB: -20,
		Ratio:       4,
		KneeDB:      6,
		AttackMs:    10,
		ReleaseMs:   100,
	}); err != nil {
		t.Fatalf("SetCompressor: %v", err)
	}

	if err := e.SetLimiter(LimiterParams{
		Enabled:   true,
		Threshold: -0.1,
		Release:   100,
	}); err != nil {
		t.Fatalf("SetLimiter: %v", err)
	}

	inputs := make([]float64, 0, 121)
	for db := -100.0; db <= 20.0; db++ {
		inputs = append(inputs, db)
	}

	for name, curve := range map[string][]float64{
		"compressor": e.CompressorCurveDB(inputs),
		"limiter":    e.LimiterCurveDB(inputs),
	} {
		if len(curve) != len(inputs) {
			t.Fatalf("%s curve has %d points, want %d", name, len(curve), len(inputs))
		}

		for i, out := range curve {
			if math.IsNaN(out) || math.IsInf(out, 0) {
				t.Fatalf("%s curve point %d is not finite: %v", name, i, out)
			}
		}

		// A downward compressor never inverts: louder in is never quieter out.
		for i := 1; i < len(curve); i++ {
			if curve[i] < curve[i-1]-1e-9 {
				t.Errorf("%s curve is not monotonic at %v dB: %v < %v",
					name, inputs[i], curve[i], curve[i-1])
			}
		}

		// And it never expands: the slope above threshold is at most 1:1.
		for i := 1; i < len(curve); i++ {
			if slope := curve[i] - curve[i-1]; slope > 1+1e-9 {
				t.Errorf("%s curve slope at %v dB is %v, want at most 1", name, inputs[i], slope)
			}
		}
	}
}

func TestNodeResponseCurveKnownNodes(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)

	freqs := []float64{20, 100, 1000, 10000, 20000}

	for _, node := range []string{"hp", "low", "mid", "high", "lp"} {
		got := e.NodeResponseCurveDB(node, freqs)
		if len(got) != len(freqs) {
			t.Fatalf("node %q returned %d points, want %d", node, len(got), len(freqs))
		}

		for i, db := range got {
			if math.IsNaN(db) || math.IsInf(db, 0) {
				t.Errorf("node %q point %d is not finite: %v", node, i, db)
			}
		}
	}
}
