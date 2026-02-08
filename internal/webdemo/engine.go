package webdemo

import (
	"fmt"
	"math"
	"math/cmplx"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
	"github.com/cwbudde/algo-dsp/dsp/filter/design/shelving"
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

// EQParams defines the 3-band EQ parameters.
type EQParams struct {
	LowFreq  float64
	LowGain  float64
	MidFreq  float64
	MidGain  float64
	HighFreq float64
	HighGain float64
	MidQ     float64
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
	low  *biquad.Section
	mid  *biquad.Section
	high *biquad.Section
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
			LowFreq:  100,
			LowGain:  0,
			MidFreq:  1000,
			MidGain:  0,
			HighFreq: 6000,
			HighGain: 0,
			MidQ:     1.2,
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
	eq.LowFreq = clamp(eq.LowFreq, 20, e.sampleRate*0.49)
	eq.MidFreq = clamp(eq.MidFreq, 20, e.sampleRate*0.49)
	eq.HighFreq = clamp(eq.HighFreq, 20, e.sampleRate*0.49)
	eq.MidQ = clamp(eq.MidQ, 0.2, 8)
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
		x = e.low.ProcessSample(x)
		x = e.mid.ProcessSample(x)
		x = e.high.ProcessSample(x)
		x *= e.eq.Master
		dst[i] = float32(clamp(x, -1, 1))
	}
}

// ResponseCurveDB returns EQ magnitude response in dB for freqs.
func (e *Engine) ResponseCurveDB(freqs []float64) []float64 {
	out := make([]float64, len(freqs))
	for i, f := range freqs {
		f = clamp(f, 1, e.sampleRate*0.49)
		h := e.low.Response(f, e.sampleRate)
		h *= e.mid.Response(f, e.sampleRate)
		h *= e.high.Response(f, e.sampleRate)
		mag := cmplx.Abs(h) * e.eq.Master
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
	lowCoeffs, err := shelving.ButterworthLowShelf(e.sampleRate, e.eq.LowFreq, e.eq.LowGain, 1)
	if err != nil {
		return fmt.Errorf("build low shelf: %w", err)
	}
	midCoeffs, err := design.PeakCascade(e.sampleRate, e.eq.MidFreq, e.eq.MidQ, e.eq.MidGain, 1)
	if err != nil {
		return fmt.Errorf("build peaking: %w", err)
	}
	highCoeffs, err := shelving.ButterworthHighShelf(e.sampleRate, e.eq.HighFreq, e.eq.HighGain, 1)
	if err != nil {
		return fmt.Errorf("build high shelf: %w", err)
	}

	e.low = biquad.NewSection(lowCoeffs[0])
	e.mid = biquad.NewSection(midCoeffs[0])
	e.high = biquad.NewSection(highCoeffs[0])
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
