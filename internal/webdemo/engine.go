package webdemo

import (
	"fmt"
	"math"
	"math/cmplx"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
)

const (
	stepCount       = 16
	minDecaySeconds = 0.01
	maxVoices       = 64
)

// StepConfig defines one sequencer step.
type StepConfig struct {
	Enabled bool
	FreqHz  float64
}

// EQParams defines the 5-node EQ parameters.
type EQParams struct {
	HPFreq   float64
	HPGain   float64
	HPQ      float64
	LowFreq  float64
	LowGain  float64
	LowQ     float64
	MidFreq  float64
	MidGain  float64
	MidQ     float64
	HighFreq float64
	HighGain float64
	HighQ    float64
	LPFreq   float64
	LPGain   float64
	LPQ      float64
	Master   float64
}

type voice struct {
	phase       float64
	phaseStep   float64
	ageSamples  int
	decaySample int
}

// Engine runs the web demo DSP pipeline in Go.
type Engine struct {
	sampleRate float64
	tempoBPM   float64
	decaySec   float64
	running    bool

	steps       [stepCount]StepConfig
	currentStep int

	samplesUntilNextStep float64
	voices               []voice

	eq   EQParams
	hp   *biquad.Section
	hpG  float64
	low  *biquad.Section
	mid  *biquad.Section
	high *biquad.Section
	lp   *biquad.Section
	lpG  float64
}

// NewEngine creates a configured audio engine.
func NewEngine(sampleRate float64) (*Engine, error) {
	if sampleRate <= 0 {
		return nil, fmt.Errorf("sample rate must be > 0: %f", sampleRate)
	}
	e := &Engine{
		sampleRate: sampleRate,
		tempoBPM:   110,
		decaySec:   0.2,
		eq: EQParams{
			HPFreq:   40,
			HPGain:   0,
			HPQ:      1 / math.Sqrt2,
			LowFreq:  100,
			LowGain:  0,
			LowQ:     1 / math.Sqrt2,
			MidFreq:  1000,
			MidGain:  0,
			MidQ:     1.2,
			HighFreq: 6000,
			HighGain: 0,
			HighQ:    1 / math.Sqrt2,
			LPFreq:   12000,
			LPGain:   0,
			LPQ:      1 / math.Sqrt2,
			Master:   0.75,
		},
	}
	for i := 0; i < stepCount; i++ {
		e.steps[i] = StepConfig{Enabled: i%4 == 0, FreqHz: defaultStepFreq(i)}
	}
	if err := e.rebuildEQ(); err != nil {
		return nil, err
	}
	e.samplesUntilNextStep = e.stepDurationSamples()
	return e, nil
}

// SetTransport updates tempo and decay.
func (e *Engine) SetTransport(tempoBPM, decaySec float64) {
	if tempoBPM > 0 {
		e.tempoBPM = tempoBPM
	}
	if decaySec < minDecaySeconds {
		decaySec = minDecaySeconds
	}
	e.decaySec = decaySec
}

// SetRunning starts or stops new step triggering.
func (e *Engine) SetRunning(running bool) {
	if running && !e.running {
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

// SetEQ updates EQ parameters and rebuilds the filters.
func (e *Engine) SetEQ(eq EQParams) error {
	eq.HPFreq = clamp(eq.HPFreq, 20, e.sampleRate*0.49)
	eq.LowFreq = clamp(eq.LowFreq, 20, e.sampleRate*0.49)
	eq.MidFreq = clamp(eq.MidFreq, 20, e.sampleRate*0.49)
	eq.HighFreq = clamp(eq.HighFreq, 20, e.sampleRate*0.49)
	eq.LPFreq = clamp(eq.LPFreq, 20, e.sampleRate*0.49)
	eq.LowGain = clamp(eq.LowGain, -24, 24)
	eq.HPGain = clamp(eq.HPGain, -24, 24)
	eq.MidGain = clamp(eq.MidGain, -24, 24)
	eq.HighGain = clamp(eq.HighGain, -24, 24)
	eq.LPGain = clamp(eq.LPGain, -24, 24)
	eq.HPQ = clamp(eq.HPQ, 0.2, 8)
	eq.LowQ = clamp(eq.LowQ, 0.2, 8)
	eq.MidQ = clamp(eq.MidQ, 0.2, 8)
	eq.HighQ = clamp(eq.HighQ, 0.2, 8)
	eq.LPQ = clamp(eq.LPQ, 0.2, 8)

	eq.LowFreq = clamp(eq.LowFreq, eq.HPFreq*1.15, e.sampleRate*0.49)
	eq.MidFreq = clamp(eq.MidFreq, eq.LowFreq*1.15, e.sampleRate*0.49)
	eq.HighFreq = clamp(eq.HighFreq, eq.MidFreq*1.15, e.sampleRate*0.49)
	eq.LPFreq = clamp(eq.LPFreq, eq.HighFreq*1.15, e.sampleRate*0.49)

	eq.HPFreq = clamp(eq.HPFreq, 20, eq.LowFreq/1.15)
	eq.LowFreq = clamp(eq.LowFreq, eq.HPFreq*1.15, eq.MidFreq/1.15)
	eq.MidFreq = clamp(eq.MidFreq, eq.LowFreq*1.15, eq.HighFreq/1.15)
	eq.HighFreq = clamp(eq.HighFreq, eq.MidFreq*1.15, eq.LPFreq/1.15)

	eq.Master = clamp(eq.Master, 0, 1)
	e.eq = eq
	return e.rebuildEQ()
}

// CurrentStep returns the currently playing step index.
func (e *Engine) CurrentStep() int {
	return e.currentStep
}

// Render fills dst with mono PCM samples in [-1, 1].
func (e *Engine) Render(dst []float32) {
	for i := range dst {
		if e.running {
			e.samplesUntilNextStep -= 1
			for e.samplesUntilNextStep <= 0 {
				e.triggerCurrentStep()
				e.currentStep = (e.currentStep + 1) % stepCount
				e.samplesUntilNextStep += e.stepDurationSamples()
			}
		}

		x := e.nextSample()
		x = e.hp.ProcessSample(x)
		x *= e.hpG
		x = e.low.ProcessSample(x)
		x = e.mid.ProcessSample(x)
		x = e.high.ProcessSample(x)
		x = e.lp.ProcessSample(x)
		x *= e.lpG
		x *= e.eq.Master
		dst[i] = float32(clamp(x, -1, 1))
	}
}

// ResponseCurveDB returns EQ magnitude response in dB for freqs.
func (e *Engine) ResponseCurveDB(freqs []float64) []float64 {
	out := make([]float64, len(freqs))
	for i, f := range freqs {
		f = clamp(f, 1, e.sampleRate*0.49)
		h := e.hp.Response(f, e.sampleRate)
		h *= complex(e.hpG, 0)
		h *= e.low.Response(f, e.sampleRate)
		h *= e.mid.Response(f, e.sampleRate)
		h *= e.high.Response(f, e.sampleRate)
		h *= e.lp.Response(f, e.sampleRate)
		h *= complex(e.lpG, 0)
		mag := cmplx.Abs(h)
		out[i] = 20 * math.Log10(math.Max(1e-12, mag))
	}
	return out
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
		sum += env * math.Sin(v.phase)

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

func (e *Engine) rebuildEQ() error {
	hpCoeffs := design.Highpass(e.eq.HPFreq, e.eq.HPQ, e.sampleRate)
	lowCoeffs := design.LowShelf(e.eq.LowFreq, e.eq.LowGain, e.eq.LowQ, e.sampleRate)
	midCoeffs := design.Peak(e.eq.MidFreq, e.eq.MidGain, e.eq.MidQ, e.sampleRate)
	highCoeffs := design.HighShelf(e.eq.HighFreq, e.eq.HighGain, e.eq.HighQ, e.sampleRate)
	lpCoeffs := design.Lowpass(e.eq.LPFreq, e.eq.LPQ, e.sampleRate)

	e.hp = biquad.NewSection(hpCoeffs)
	e.hpG = math.Pow(10, e.eq.HPGain/20)
	e.low = biquad.NewSection(lowCoeffs)
	e.mid = biquad.NewSection(midCoeffs)
	e.high = biquad.NewSection(highCoeffs)
	e.lp = biquad.NewSection(lpCoeffs)
	e.lpG = math.Pow(10, e.eq.LPGain/20)
	return nil
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

func clamp(v, minV, maxV float64) float64 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func defaultStepFreq(i int) float64 {
	defaults := [...]float64{130.81, 164.81, 196, 220, 261.63, 329.63, 392, 440}
	return defaults[(i % 8)]
}
