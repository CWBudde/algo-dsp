package webdemo

import (
	"fmt"
	"math"
	"math/cmplx"
	"strings"

	algofft "github.com/MeKo-Christian/algo-fft"
	"github.com/cwbudde/algo-dsp/dsp/effects"
	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
	"github.com/cwbudde/algo-dsp/dsp/window"
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

// EffectsParams defines chorus and reverb parameters for the demo chain.
type EffectsParams struct {
	ChorusEnabled bool
	ChorusMix     float64
	ChorusDepth   float64
	ChorusSpeedHz float64
	ChorusStages  int

	ReverbEnabled  bool
	ReverbWet      float64
	ReverbDry      float64
	ReverbRoomSize float64
	ReverbDamp     float64
	ReverbGain     float64
}

// SpectrumParams defines real-time analyzer settings.
type SpectrumParams struct {
	FFTSize   int
	Overlap   float64
	Window    string
	Smoothing float64
}

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

// Engine runs the web demo DSP pipeline in Go.
type Engine struct {
	sampleRate float64
	tempoBPM   float64
	decaySec   float64
	shuffle    float64
	running    bool
	waveform   Waveform

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

	effects EffectsParams
	chorus  *effects.Chorus
	reverb  *effects.Reverb

	spectrum             SpectrumParams
	spectrumWindow       []float64
	spectrumWindowGain   float64
	spectrumPlan         *algofft.Plan[complex128]
	spectrumFFTSize      int
	spectrumHopSize      int
	spectrumInput        []complex128
	spectrumOutput       []complex128
	spectrumRing         []float64
	spectrumWrite        int
	spectrumFilled       int
	spectrumSamplesToHop int
	spectrumDB           []float64
	spectrumReady        bool
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
		shuffle:    0,
		waveform:   WaveSine,
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
		effects: EffectsParams{
			ChorusEnabled:  false,
			ChorusMix:      0.18,
			ChorusDepth:    0.003,
			ChorusSpeedHz:  0.35,
			ChorusStages:   3,
			ReverbEnabled:  false,
			ReverbWet:      0.22,
			ReverbDry:      1.0,
			ReverbRoomSize: 0.72,
			ReverbDamp:     0.45,
			ReverbGain:     0.015,
		},
		spectrum: SpectrumParams{
			FFTSize:   2048,
			Overlap:   0.75,
			Window:    "blackmanharris",
			Smoothing: 0.65,
		},
	}
	if err := e.initSpectrumAnalyzer(); err != nil {
		return nil, err
	}
	chorus, err := effects.NewChorus()
	if err != nil {
		return nil, err
	}
	e.chorus = chorus
	e.reverb = effects.NewReverb()
	if err := e.rebuildEffects(); err != nil {
		return nil, err
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

// SetEffects updates chorus/reverb settings.
func (e *Engine) SetEffects(p EffectsParams) error {
	prevChorusEnabled := e.effects.ChorusEnabled
	prevReverbEnabled := e.effects.ReverbEnabled

	p.ChorusMix = clamp(p.ChorusMix, 0, 1)
	p.ChorusDepth = clamp(p.ChorusDepth, 0, 0.01)
	p.ChorusSpeedHz = clamp(p.ChorusSpeedHz, 0.05, 5)
	if p.ChorusStages < 1 {
		p.ChorusStages = 1
	}
	if p.ChorusStages > 6 {
		p.ChorusStages = 6
	}

	p.ReverbWet = clamp(p.ReverbWet, 0, 1.5)
	p.ReverbDry = clamp(p.ReverbDry, 0, 1.5)
	p.ReverbRoomSize = clamp(p.ReverbRoomSize, 0, 0.98)
	p.ReverbDamp = clamp(p.ReverbDamp, 0, 0.99)
	p.ReverbGain = clamp(p.ReverbGain, 0, 0.1)

	e.effects = p
	if err := e.rebuildEffects(); err != nil {
		return err
	}
	if prevChorusEnabled && !p.ChorusEnabled {
		e.chorus.Reset()
	}
	if prevReverbEnabled && !p.ReverbEnabled {
		e.reverb.Reset()
	}
	return nil
}

// SetSpectrum updates analyzer settings used for the EQ graph spectrum.
func (e *Engine) SetSpectrum(p SpectrumParams) error {
	cfg := sanitizeSpectrumParams(p)

	winType, err := spectrumWindowType(cfg.Window)
	if err != nil {
		return err
	}
	win := window.Generate(winType, cfg.FFTSize, window.WithPeriodic())
	if len(win) != cfg.FFTSize {
		return fmt.Errorf("invalid analyzer window size: %d", cfg.FFTSize)
	}
	sum := 0.0
	for _, w := range win {
		sum += w
	}

	plan, err := algofft.NewPlan64(cfg.FFTSize)
	if err != nil {
		return fmt.Errorf("spectrum init fft plan: %w", err)
	}

	hop := int(math.Round(float64(cfg.FFTSize) * (1 - cfg.Overlap)))
	if hop < 1 {
		hop = 1
	}

	e.spectrum = cfg
	e.spectrumWindow = win
	e.spectrumWindowGain = sum / float64(cfg.FFTSize)
	e.spectrumPlan = plan
	e.spectrumFFTSize = cfg.FFTSize
	e.spectrumHopSize = hop
	e.spectrumInput = make([]complex128, cfg.FFTSize)
	e.spectrumOutput = make([]complex128, cfg.FFTSize)
	e.spectrumRing = make([]float64, cfg.FFTSize)
	e.spectrumWrite = 0
	e.spectrumFilled = 0
	e.spectrumSamplesToHop = 0
	e.spectrumDB = make([]float64, cfg.FFTSize/2+1)
	for i := range e.spectrumDB {
		e.spectrumDB[i] = -120
	}
	e.spectrumReady = false
	return nil
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
				stepIndex := e.currentStep
				e.triggerCurrentStep()
				e.currentStep = (stepIndex + 1) % stepCount
				e.samplesUntilNextStep += e.stepDurationSamplesForStep(stepIndex)
			}
		}

		x := e.nextSample()
		if e.effects.ChorusEnabled {
			x = e.chorus.ProcessSample(x)
		}
		if e.effects.ReverbEnabled {
			x = e.reverb.ProcessSample(x)
		}
		x = e.hp.ProcessSample(x)
		x *= e.hpG
		x = e.low.ProcessSample(x)
		x = e.mid.ProcessSample(x)
		x = e.high.ProcessSample(x)
		x = e.lp.ProcessSample(x)
		x *= e.lpG
		x *= e.eq.Master
		e.pushSpectrumSample(x)
		dst[i] = float32(clamp(x, -1, 1))
	}
}

func (e *Engine) rebuildEffects() error {
	if err := e.chorus.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if err := e.chorus.SetMix(e.effects.ChorusMix); err != nil {
		return err
	}
	if err := e.chorus.SetDepth(e.effects.ChorusDepth); err != nil {
		return err
	}
	if err := e.chorus.SetSpeedHz(e.effects.ChorusSpeedHz); err != nil {
		return err
	}
	if err := e.chorus.SetStages(e.effects.ChorusStages); err != nil {
		return err
	}

	e.reverb.SetWet(e.effects.ReverbWet)
	e.reverb.SetDry(e.effects.ReverbDry)
	e.reverb.SetRoomSize(e.effects.ReverbRoomSize)
	e.reverb.SetDamp(e.effects.ReverbDamp)
	e.reverb.SetGain(e.effects.ReverbGain)
	return nil
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

// SpectrumCurveDB returns a smoothed real-time spectrum in dBFS for freqs.
func (e *Engine) SpectrumCurveDB(freqs []float64) []float64 {
	out := make([]float64, len(freqs))
	if !e.spectrumReady || len(e.spectrumDB) == 0 {
		for i := range out {
			out[i] = -120
		}
		return out
	}

	nyquist := e.sampleRate * 0.5
	lastBin := len(e.spectrumDB) - 1
	if lastBin < 1 {
		for i := range out {
			out[i] = -120
		}
		return out
	}
	binHz := e.sampleRate / float64(e.spectrumFFTSize)
	for i, f := range freqs {
		f = clamp(f, 0, nyquist)
		bin := f / binHz
		if bin <= 0 {
			out[i] = e.spectrumDB[0]
			continue
		}
		if bin >= float64(lastBin) {
			out[i] = e.spectrumDB[lastBin]
			continue
		}
		base := int(bin)
		frac := bin - float64(base)
		d0 := e.spectrumDB[base]
		d1 := e.spectrumDB[base+1]
		out[i] = d0 + frac*(d1-d0)
	}
	return out
}

func (e *Engine) initSpectrumAnalyzer() error {
	return e.SetSpectrum(e.spectrum)
}

func (e *Engine) pushSpectrumSample(x float64) {
	e.spectrumRing[e.spectrumWrite] = x
	e.spectrumWrite++
	if e.spectrumWrite >= e.spectrumFFTSize {
		e.spectrumWrite = 0
	}
	if e.spectrumFilled < e.spectrumFFTSize {
		e.spectrumFilled++
	}
	e.spectrumSamplesToHop++
	if e.spectrumFilled < e.spectrumFFTSize || e.spectrumSamplesToHop < e.spectrumHopSize {
		return
	}
	e.spectrumSamplesToHop = 0
	e.updateSpectrumFrame()
}

func (e *Engine) updateSpectrumFrame() {
	const (
		minDB = -120.0
		eps   = 1e-12
	)

	read := e.spectrumWrite
	for i := 0; i < e.spectrumFFTSize; i++ {
		s := e.spectrumRing[read]
		e.spectrumInput[i] = complex(s*e.spectrumWindow[i], 0)
		read++
		if read >= e.spectrumFFTSize {
			read = 0
		}
	}
	if err := e.spectrumPlan.Forward(e.spectrumOutput, e.spectrumInput); err != nil {
		return
	}

	norm := float64(e.spectrumFFTSize) * math.Max(e.spectrumWindowGain, eps)
	last := len(e.spectrumDB) - 1
	for k := 0; k <= last; k++ {
		mag := cmplx.Abs(e.spectrumOutput[k]) / norm
		if k > 0 && k < last {
			mag *= 2
		}
		db := 20 * math.Log10(math.Max(eps, mag))
		if db < minDB {
			db = minDB
		}
		if !e.spectrumReady {
			e.spectrumDB[k] = db
			continue
		}
		smooth := e.spectrum.Smoothing
		e.spectrumDB[k] = smooth*e.spectrumDB[k] + (1-smooth)*db
	}
	e.spectrumReady = true
}

func sanitizeSpectrumParams(p SpectrumParams) SpectrumParams {
	cfg := p
	switch cfg.FFTSize {
	case 256, 512, 1024, 2048, 4096, 8192:
	default:
		cfg.FFTSize = 2048
	}
	cfg.Overlap = clamp(cfg.Overlap, 0.25, 0.95)
	cfg.Smoothing = clamp(cfg.Smoothing, 0, 0.95)
	cfg.Window = strings.ToLower(strings.TrimSpace(cfg.Window))
	if cfg.Window == "" {
		cfg.Window = "blackmanharris"
	}
	return cfg
}

func spectrumWindowType(name string) (window.Type, error) {
	switch name {
	case "hann":
		return window.TypeHann, nil
	case "hamming":
		return window.TypeHamming, nil
	case "blackman":
		return window.TypeBlackman, nil
	case "blackmanharris":
		return window.TypeBlackmanHarris4Term, nil
	case "flattop":
		return window.TypeFlatTop, nil
	default:
		return 0, fmt.Errorf("unsupported spectrum window: %s", name)
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
