package webdemo

import "math"

// voice represents a synthesized voice with phase and envelope state.
type voice struct {
	waveform    Waveform
	phase       float64
	phaseStep   float64
	ageSamples  int
	decaySample int
}

// Waveform defines oscillator shape for synth voices.
type Waveform int

const (
	WaveSine Waveform = iota
	WaveTriangle
	WaveSaw
	WaveSquare
)

// SetWaveform updates oscillator shape used for newly-triggered voices.
func (e *Engine) SetWaveform(name string) {
	switch name {
	case "triangle":
		e.waveform = WaveTriangle
	case "saw":
		e.waveform = WaveSaw
	case "square":
		e.waveform = WaveSquare
	default:
		e.waveform = WaveSine
	}
}

// SetTransport updates tempo, decay, and shuffle amount.
func (e *Engine) SetTransport(tempoBPM, decaySec, shuffle float64) {
	if tempoBPM > 0 {
		e.tempoBPM = tempoBPM
	}
	if decaySec < minDecaySeconds {
		decaySec = minDecaySeconds
	}
	e.decaySec = decaySec
	e.shuffle = clamp(shuffle, 0, 1)
}

// SetRunning starts or stops new step triggering.
func (e *Engine) SetRunning(running bool) {
	if running && !e.running {
		e.currentStep = 0
		e.samplesUntilNextStep = 0
	}
	e.running = running
}

// SetSteps updates the full 16-step pattern.
func (e *Engine) SetSteps(steps []StepConfig) {
	for i := 0; i < stepCount && i < len(steps); i++ {
		cfg := steps[i]
		if cfg.FreqHz <= 0 {
			cfg.FreqHz = 110
		}
		e.steps[i] = cfg
	}
}

func (e *Engine) triggerCurrentStep() {
	step := e.steps[e.currentStep]
	if !step.Enabled || step.FreqHz <= 0 {
		return
	}
	if len(e.voices) >= maxVoices {
		copy(e.voices, e.voices[1:])
		e.voices = e.voices[:maxVoices-1]
	}
	decaySamples := int(e.decaySec * e.sampleRate)
	if decaySamples < 1 {
		decaySamples = 1
	}
	e.voices = append(e.voices, voice{
		waveform:    e.waveform,
		phase:       0,
		phaseStep:   2 * math.Pi * step.FreqHz / e.sampleRate,
		ageSamples:  0,
		decaySample: decaySamples,
	})
}

func (e *Engine) nextSample() float64 {
	if len(e.voices) == 0 {
		return 0
	}
	attackSamples := int(0.005 * e.sampleRate)
	if attackSamples < 1 {
		attackSamples = 1
	}

	sum := 0.0
	write := 0
	for i := range e.voices {
		v := e.voices[i]
		if v.ageSamples >= v.decaySample {
			continue
		}

		env := envelope(v.ageSamples, attackSamples, v.decaySample)
		sum += env * waveSample(v.waveform, v.phase)

		v.phase += v.phaseStep
		if v.phase > math.Pi {
			v.phase -= 2 * math.Pi
		}
		v.ageSamples++
		e.voices[write] = v
		write++
	}
	e.voices = e.voices[:write]
	return sum
}

func (e *Engine) stepDurationSamples() float64 {
	return e.sampleRate * 60.0 / e.tempoBPM / 4.0
}

func (e *Engine) stepDurationSamplesForStep(stepIndex int) float64 {
	base := e.stepDurationSamples()
	ratio := shuffleRatio(e.shuffle)
	if ratio <= 0 {
		return base
	}
	if stepIndex%2 == 0 {
		return base * (1 + ratio)
	}
	return base * (1 - ratio)
}

func shuffleRatio(shuffle float64) float64 {
	// Map 0..1 control to 0..1/3 timing ratio with a gentle curve.
	return (1.0 / 3.0) * math.Pow(clamp(shuffle, 0, 1), 1.6)
}

func envelope(age, attack, decay int) float64 {
	const start = 0.0001
	const peak = 0.22
	const end = 0.0001

	if age < attack {
		t := float64(age) / float64(attack)
		return start * math.Pow(peak/start, t)
	}
	if decay <= attack {
		return end
	}
	t := float64(age-attack) / float64(decay-attack)
	return peak * math.Pow(end/peak, t)
}

func defaultStepFreq(i int) float64 {
	defaults := [...]float64{130.81, 164.81, 196, 220, 261.63, 329.63, 392, 440}
	return defaults[(i % 8)]
}

func waveSample(w Waveform, phase float64) float64 {
	switch w {
	case WaveTriangle:
		return (2 / math.Pi) * math.Asin(math.Sin(phase))
	case WaveSaw:
		return phase / math.Pi
	case WaveSquare:
		if math.Sin(phase) >= 0 {
			return 1
		}
		return -1
	default:
		return math.Sin(phase)
	}
}
