package webdemo

import (
	"math"
	"testing"
)

func TestSetWaveform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want Waveform
	}{
		{"triangle", WaveTriangle},
		{"saw", WaveSaw},
		{"square", WaveSquare},
		{"sine", WaveSine},
		// Anything unrecognised falls back to sine rather than erroring, since
		// the value arrives unvalidated from a <select> in the browser.
		{"", WaveSine},
		{"Triangle", WaveSine},
		{"bogus", WaveSine},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			e := newTestEngine(t)
			e.SetWaveform(tc.name)

			if e.waveform != tc.want {
				t.Errorf("SetWaveform(%q) gave %v, want %v", tc.name, e.waveform, tc.want)
			}
		})
	}
}

func TestWaveSampleRanges(t *testing.T) {
	t.Parallel()

	// Phase is kept in [-pi, pi] by nextSample.
	for _, w := range []Waveform{WaveSine, WaveTriangle, WaveSaw, WaveSquare} {
		for i := range 512 {
			phase := -math.Pi + 2*math.Pi*float64(i)/512

			got := waveSample(w, phase)
			if math.IsNaN(got) || math.IsInf(got, 0) {
				t.Fatalf("waveform %v at phase %v is not finite: %v", w, phase, got)
			}

			if got < -1.0001 || got > 1.0001 {
				t.Errorf("waveform %v at phase %v = %v, want within [-1, 1]", w, phase, got)
			}
		}
	}
}

func TestWaveSampleShapes(t *testing.T) {
	t.Parallel()

	const tol = 1e-9

	// Zero crossing at phase 0 for the odd-symmetric shapes.
	for _, w := range []Waveform{WaveSine, WaveTriangle, WaveSaw} {
		if got := waveSample(w, 0); math.Abs(got) > tol {
			t.Errorf("waveform %v at phase 0 = %v, want 0", w, got)
		}
	}

	// Square is a hard two-level signal.
	if got := waveSample(WaveSquare, math.Pi/2); got != 1 {
		t.Errorf("square at +pi/2 = %v, want 1", got)
	}

	if got := waveSample(WaveSquare, -math.Pi/2); got != -1 {
		t.Errorf("square at -pi/2 = %v, want -1", got)
	}

	// Saw ramps linearly from -1 to +1 across the phase range.
	if got := waveSample(WaveSaw, math.Pi); math.Abs(got-1) > tol {
		t.Errorf("saw at +pi = %v, want 1", got)
	}

	if got := waveSample(WaveSaw, -math.Pi); math.Abs(got+1) > tol {
		t.Errorf("saw at -pi = %v, want -1", got)
	}

	// Triangle peaks at +/-pi/2.
	if got := waveSample(WaveTriangle, math.Pi/2); math.Abs(got-1) > tol {
		t.Errorf("triangle at +pi/2 = %v, want 1", got)
	}
}

func TestSetTransport(t *testing.T) {
	t.Parallel()

	t.Run("stores valid values", func(t *testing.T) {
		t.Parallel()

		e := newTestEngine(t)
		e.SetTransport(140, 0.4, 0.5)

		if e.tempoBPM != 140 {
			t.Errorf("tempo = %v, want 140", e.tempoBPM)
		}

		if e.decaySec != 0.4 {
			t.Errorf("decay = %v, want 0.4", e.decaySec)
		}

		if e.shuffle != 0.5 {
			t.Errorf("shuffle = %v, want 0.5", e.shuffle)
		}
	})

	t.Run("rejects non-positive tempo", func(t *testing.T) {
		t.Parallel()

		e := newTestEngine(t)
		before := e.tempoBPM

		e.SetTransport(0, 0.2, 0)
		e.SetTransport(-100, 0.2, 0)

		if e.tempoBPM != before {
			t.Errorf("tempo = %v, want it unchanged at %v", e.tempoBPM, before)
		}
	})

	t.Run("clamps decay and shuffle", func(t *testing.T) {
		t.Parallel()

		e := newTestEngine(t)

		e.SetTransport(120, -1, -1)

		if e.decaySec != minDecaySeconds {
			t.Errorf("decay = %v, want the %v floor", e.decaySec, minDecaySeconds)
		}

		if e.shuffle != 0 {
			t.Errorf("shuffle = %v, want 0", e.shuffle)
		}

		e.SetTransport(120, 1, 10)

		if e.shuffle != 1 {
			t.Errorf("shuffle = %v, want 1", e.shuffle)
		}
	})
}

func TestShuffleRatio(t *testing.T) {
	t.Parallel()

	if got := shuffleRatio(0); got != 0 {
		t.Errorf("shuffleRatio(0) = %v, want 0", got)
	}

	if got, want := shuffleRatio(1), 1.0/3.0; math.Abs(got-want) > 1e-12 {
		t.Errorf("shuffleRatio(1) = %v, want %v", got, want)
	}

	// Out-of-range inputs are clamped, not extrapolated.
	if got := shuffleRatio(-5); got != 0 {
		t.Errorf("shuffleRatio(-5) = %v, want 0", got)
	}

	if got, want := shuffleRatio(5), 1.0/3.0; math.Abs(got-want) > 1e-12 {
		t.Errorf("shuffleRatio(5) = %v, want %v", got, want)
	}

	// Monotonic across the control range.
	prev := shuffleRatio(0)

	for i := 1; i <= 100; i++ {
		got := shuffleRatio(float64(i) / 100)
		if got < prev {
			t.Fatalf("shuffleRatio is not monotonic at %v: %v < %v", float64(i)/100, got, prev)
		}

		prev = got
	}
}

// TestStepDurationShufflePreservesPairDuration is the property that makes
// shuffle a groove rather than a tempo change: a long step plus the following
// short step still spans exactly two straight steps.
func TestStepDurationShufflePreservesPairDuration(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)
	e.SetTransport(120, 0.2, 0.7)

	base := e.stepDurationSamples()

	for step := 0; step < 8; step += 2 {
		long := e.stepDurationSamplesForStep(step)
		short := e.stepDurationSamplesForStep(step + 1)

		if long <= short {
			t.Errorf("step %d: expected the even step to be the long one (%v vs %v)", step, long, short)
		}

		if got, want := long+short, 2*base; math.Abs(got-want) > 1e-9 {
			t.Errorf("step pair %d duration = %v, want %v", step, got, want)
		}
	}
}

func TestStepDurationNoShuffleIsUniform(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)
	e.SetTransport(120, 0.2, 0)

	base := e.stepDurationSamples()
	for step := range stepCount {
		if got := e.stepDurationSamplesForStep(step); got != base {
			t.Errorf("step %d = %v, want the uniform %v", step, got, base)
		}
	}
}

func TestSetStepsIgnoresExtrasAndFixesBadFrequencies(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)

	steps := make([]StepConfig, stepCount+8)
	for i := range steps {
		steps[i] = StepConfig{Enabled: true, FreqHz: 440}
	}

	steps[0].FreqHz = 0
	steps[1].FreqHz = -220

	// More steps than the engine holds must not panic or overflow.
	e.SetSteps(steps)

	if e.steps[0].FreqHz != 110 {
		t.Errorf("zero frequency became %v, want the 110 fallback", e.steps[0].FreqHz)
	}

	if e.steps[1].FreqHz != 110 {
		t.Errorf("negative frequency became %v, want the 110 fallback", e.steps[1].FreqHz)
	}

	if e.steps[2].FreqHz != 440 {
		t.Errorf("valid frequency became %v, want 440", e.steps[2].FreqHz)
	}
}

func TestSetStepsAcceptsShortSlice(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)

	before := e.steps[5]

	e.SetSteps([]StepConfig{{Enabled: true, FreqHz: 200}})

	if e.steps[0].FreqHz != 200 {
		t.Errorf("step 0 = %v, want 200", e.steps[0].FreqHz)
	}

	if e.steps[5] != before {
		t.Errorf("step 5 changed to %+v, want it untouched at %+v", e.steps[5], before)
	}
}

func TestSetRunningResetsPosition(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)

	e.currentStep = 7
	e.samplesUntilNextStep = 1234

	e.SetRunning(true)

	if e.currentStep != 0 {
		t.Errorf("currentStep = %d, want 0 on start", e.currentStep)
	}

	if e.samplesUntilNextStep != 0 {
		t.Errorf("samplesUntilNextStep = %v, want 0 on start", e.samplesUntilNextStep)
	}

	// Re-asserting running must not rewind an already-playing sequence.
	e.currentStep = 3
	e.SetRunning(true)

	if e.currentStep != 3 {
		t.Errorf("currentStep = %d, want it left at 3", e.currentStep)
	}
}

// TestVoiceStealing covers the copy(e.voices, e.voices[1:]) path, which drops
// the oldest voice once the polyphony limit is reached.
func TestVoiceStealing(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)
	e.SetTransport(120, 0.5, 0)

	for i := range stepCount {
		e.steps[i] = StepConfig{Enabled: true, FreqHz: 100 + float64(i)}
	}

	for range maxVoices * 3 {
		e.triggerCurrentStep()
		e.currentStep = (e.currentStep + 1) % stepCount
	}

	if len(e.voices) > maxVoices {
		t.Fatalf("voice count reached %d, want at most %d", len(e.voices), maxVoices)
	}

	if len(e.voices) == 0 {
		t.Fatal("no voices were triggered")
	}
}

func TestTriggerSkipsDisabledSteps(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)

	for i := range stepCount {
		e.steps[i] = StepConfig{Enabled: false, FreqHz: 440}
	}

	e.voices = e.voices[:0]
	e.currentStep = 0
	e.triggerCurrentStep()

	if len(e.voices) != 0 {
		t.Errorf("a disabled step produced %d voices, want 0", len(e.voices))
	}
}

func TestEnvelopeShape(t *testing.T) {
	t.Parallel()

	const (
		attack = 240
		decay  = 24000
		peak   = 0.22
	)

	// Rises through the attack.
	prev := envelope(0, attack, decay)
	for age := 1; age < attack; age++ {
		got := envelope(age, attack, decay)
		if got < prev {
			t.Fatalf("attack is not monotonic at age %d: %v < %v", age, got, prev)
		}

		prev = got
	}

	if got := envelope(attack, attack, decay); math.Abs(got-peak) > 1e-9 {
		t.Errorf("envelope at the end of attack = %v, want the %v peak", got, peak)
	}

	// Falls through the decay.
	prev = envelope(attack, attack, decay)
	for age := attack + 1; age < decay; age += 97 {
		got := envelope(age, attack, decay)
		if got > prev {
			t.Fatalf("decay is not monotonic at age %d: %v > %v", age, got, prev)
		}

		prev = got
	}

	// Never negative, never above the peak.
	for age := 0; age <= decay; age += 13 {
		got := envelope(age, attack, decay)
		if got < 0 || got > peak+1e-9 {
			t.Fatalf("envelope at age %d = %v, want within [0, %v]", age, got, peak)
		}
	}
}

// TestEnvelopeDegenerateDecay covers decay <= attack, which the guard in
// envelope() exists for and which a very short decay setting can produce.
func TestEnvelopeDegenerateDecay(t *testing.T) {
	t.Parallel()

	got := envelope(100, 240, 100)
	if math.IsNaN(got) || math.IsInf(got, 0) {
		t.Fatalf("degenerate decay gave a non-finite envelope: %v", got)
	}

	if got != 0.0001 {
		t.Errorf("degenerate decay gave %v, want the 0.0001 end level", got)
	}
}

func TestDefaultStepFreqWraps(t *testing.T) {
	t.Parallel()

	// The pattern is 16 steps over an 8-note table, so it must wrap safely.
	for i := range stepCount {
		got := defaultStepFreq(i)
		if got <= 0 {
			t.Fatalf("defaultStepFreq(%d) = %v, want a positive frequency", i, got)
		}

		if want := defaultStepFreq(i % 8); got != want {
			t.Errorf("defaultStepFreq(%d) = %v, want %v", i, got, want)
		}
	}
}

// TestRenderProducesAudioWhenRunning is the end-to-end check that the
// sequencer, envelope and effects path together make sound.
func TestRenderProducesAudioWhenRunning(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)

	for i := range stepCount {
		e.steps[i] = StepConfig{Enabled: true, FreqHz: 220}
	}

	e.SetTransport(120, 0.2, 0)
	e.SetRunning(true)

	block := make([]float32, 4096)

	peak := float32(0)

	for range 8 {
		e.Render(block)

		for _, v := range block {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				t.Fatal("render produced a non-finite sample")
			}

			peak = float32(math.Max(float64(peak), math.Abs(float64(v))))
		}
	}

	if peak == 0 {
		t.Fatal("render produced silence while running")
	}

	if peak > 1 {
		t.Errorf("render peaked at %v, want it within full scale", peak)
	}
}

func TestRenderIsSilentWhenStopped(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)
	e.SetRunning(false)

	block := make([]float32, 2048)
	e.Render(block)

	for i, v := range block {
		if v != 0 {
			t.Fatalf("sample %d = %v, want silence before anything is triggered", i, v)
		}
	}
}
