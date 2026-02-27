package webdemo

import (
	"fmt"
	"math"
	"math/cmplx"

	"github.com/cwbudde/algo-dsp/dsp/effects"
	"github.com/cwbudde/algo-dsp/dsp/effects/dynamics"
	"github.com/cwbudde/algo-dsp/dsp/effects/modulation"
	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
	"github.com/cwbudde/algo-dsp/dsp/effects/reverb"
	"github.com/cwbudde/algo-dsp/dsp/effects/spatial"
	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/crossover"
	algofft "github.com/cwbudde/algo-fft"
)

const (
	stepCount       = 16
	minDecaySeconds = 0.01
	maxVoices       = 64
	eqDefaultOrder  = 2
)

// StepConfig defines one sequencer step.
type StepConfig struct {
	Enabled bool
	FreqHz  float64
}

// EQParams defines the 5-node EQ parameters.
type EQParams struct {
	HPFamily   string
	HPType     string
	HPOrder    int
	HPFreq     float64
	HPGain     float64
	HPQ        float64
	LowFamily  string
	LowType    string
	LowOrder   int
	LowFreq    float64
	LowGain    float64
	LowQ       float64
	MidFamily  string
	MidType    string
	MidOrder   int
	MidFreq    float64
	MidGain    float64
	MidQ       float64
	HighFamily string
	HighType   string
	HighOrder  int
	HighFreq   float64
	HighGain   float64
	HighQ      float64
	LPFamily   string
	LPType     string
	LPOrder    int
	LPFreq     float64
	LPGain     float64
	LPQ        float64
	Master     float64
}

// EffectsParams defines effect parameters for the demo chain.
type EffectsParams struct {
	ChorusEnabled bool
	ChorusMix     float64
	ChorusDepth   float64
	ChorusSpeedHz float64
	ChorusStages  int

	FlangerEnabled   bool
	FlangerRateHz    float64
	FlangerDepth     float64
	FlangerBaseDelay float64
	FlangerFeedback  float64
	FlangerMix       float64

	RingModEnabled   bool
	RingModCarrierHz float64
	RingModMix       float64

	BitCrusherEnabled    bool
	BitCrusherBitDepth   float64
	BitCrusherDownsample int
	BitCrusherMix        float64

	WidenerEnabled bool
	WidenerWidth   float64
	WidenerMix     float64

	PhaserEnabled   bool
	PhaserRateHz    float64
	PhaserMinFreqHz float64
	PhaserMaxFreqHz float64
	PhaserStages    int
	PhaserFeedback  float64
	PhaserMix       float64

	TremoloEnabled     bool
	TremoloRateHz      float64
	TremoloDepth       float64
	TremoloSmoothingMs float64
	TremoloMix         float64

	DelayEnabled  bool
	DelayTime     float64
	DelayFeedback float64
	DelayMix      float64

	TimePitchEnabled   bool
	TimePitchSemitones float64
	TimePitchSequence  float64
	TimePitchOverlap   float64
	TimePitchSearch    float64

	SpectralPitchEnabled   bool
	SpectralPitchSemitones float64
	SpectralPitchFrameSize int
	SpectralPitchHop       int

	ReverbEnabled  bool
	ReverbModel    string
	ReverbWet      float64
	ReverbDry      float64
	ReverbRoomSize float64
	ReverbDamp     float64
	ReverbGain     float64
	ReverbRT60     float64
	ReverbPreDelay float64
	ReverbModDepth float64
	ReverbModRate  float64

	HarmonicBassEnabled    bool
	HarmonicBassFrequency  float64
	HarmonicBassInputGain  float64
	HarmonicBassHighGain   float64
	HarmonicBassOriginal   float64
	HarmonicBassHarmonic   float64
	HarmonicBassDecay      float64
	HarmonicBassResponseMs float64
	HarmonicBassHighpass   int

	ChainGraphJSON string
}

// CompressorParams defines compressor settings.
type CompressorParams struct {
	Enabled      bool
	ThresholdDB  float64
	Ratio        float64
	KneeDB       float64
	AttackMs     float64
	ReleaseMs    float64
	MakeupGainDB float64
	AutoMakeup   bool
}

// LimiterParams defines limiter settings.
type LimiterParams struct {
	Enabled   bool
	Threshold float64
	Release   float64
}

// SpectrumParams defines real-time analyzer settings.
type SpectrumParams struct {
	FFTSize   int
	Overlap   float64
	Window    string
	Smoothing float64
}

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
	hp   *biquad.Chain
	low  *biquad.Chain
	mid  *biquad.Chain
	high *biquad.Chain
	lp   *biquad.Chain

	effects EffectsParams
	chorus  *modulation.Chorus
	flanger *modulation.Flanger
	ringMod *modulation.RingModulator
	crusher *effects.BitCrusher
	widener *spatial.StereoWidener
	phaser  *modulation.Phaser
	tremolo *modulation.Tremolo
	delay   *effects.Delay
	reverb  *reverb.Reverb
	fdn     *reverb.FDNReverb
	bass    *effects.HarmonicBass
	tp      *pitch.PitchShifter
	sp      *pitch.SpectralPitchShifter

	irLib *IRLibrary

	compParams CompressorParams
	compressor *dynamics.Compressor

	limParams LimiterParams
	limiter   *dynamics.Limiter

	renderBlock       []float64
	chainBuf          []float64
	chainMixBuf       []float64
	chainOutBuf       map[string][]float64
	chainSplitLowBuf  map[string][]float64
	chainSplitHighBuf map[string][]float64
	chainCrossover    map[string]*crossover.Crossover

	chainGraph *compiledChainGraph
	chainNodes map[string]*chainNodeRuntime

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
//
//nolint:cyclop
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
			HPFamily:   "rbj",
			HPType:     "highpass",
			HPOrder:    eqDefaultOrder,
			HPFreq:     40,
			HPGain:     0,
			HPQ:        1 / math.Sqrt2,
			LowFamily:  "rbj",
			LowType:    "lowshelf",
			LowOrder:   eqDefaultOrder,
			LowFreq:    100,
			LowGain:    0,
			LowQ:       1 / math.Sqrt2,
			MidFamily:  "rbj",
			MidType:    "peak",
			MidOrder:   eqDefaultOrder,
			MidFreq:    1000,
			MidGain:    0,
			MidQ:       1.2,
			HighFamily: "rbj",
			HighType:   "highshelf",
			HighOrder:  eqDefaultOrder,
			HighFreq:   6000,
			HighGain:   0,
			HighQ:      1 / math.Sqrt2,
			LPFamily:   "rbj",
			LPType:     "lowpass",
			LPOrder:    eqDefaultOrder,
			LPFreq:     12000,
			LPGain:     0,
			LPQ:        1 / math.Sqrt2,
			Master:     0.75,
		},
		effects: EffectsParams{
			ChorusEnabled:          false,
			ChorusMix:              0.18,
			ChorusDepth:            0.003,
			ChorusSpeedHz:          0.35,
			ChorusStages:           3,
			FlangerEnabled:         false,
			FlangerRateHz:          0.25,
			FlangerDepth:           0.0015,
			FlangerBaseDelay:       0.001,
			FlangerFeedback:        0.25,
			FlangerMix:             0.5,
			RingModEnabled:         false,
			RingModCarrierHz:       440,
			RingModMix:             1.0,
			BitCrusherEnabled:      false,
			BitCrusherBitDepth:     8,
			BitCrusherDownsample:   4,
			BitCrusherMix:          1.0,
			WidenerEnabled:         false,
			WidenerWidth:           1.0,
			WidenerMix:             0.5,
			PhaserEnabled:          false,
			PhaserRateHz:           0.4,
			PhaserMinFreqHz:        300,
			PhaserMaxFreqHz:        1600,
			PhaserStages:           6,
			PhaserFeedback:         0.2,
			PhaserMix:              0.5,
			TremoloEnabled:         false,
			TremoloRateHz:          4,
			TremoloDepth:           0.6,
			TremoloSmoothingMs:     5,
			TremoloMix:             1.0,
			DelayEnabled:           false,
			DelayTime:              0.25,
			DelayFeedback:          0.35,
			DelayMix:               0.25,
			TimePitchEnabled:       false,
			TimePitchSemitones:     0,
			TimePitchSequence:      40,
			TimePitchOverlap:       10,
			TimePitchSearch:        15,
			SpectralPitchEnabled:   false,
			SpectralPitchSemitones: 0,
			SpectralPitchFrameSize: 1024,
			SpectralPitchHop:       256,
			ReverbEnabled:          false,
			ReverbModel:            "freeverb",
			ReverbWet:              0.22,
			ReverbDry:              1.0,
			ReverbRoomSize:         0.72,
			ReverbDamp:             0.45,
			ReverbGain:             0.015,
			ReverbRT60:             1.8,
			ReverbPreDelay:         0.01,
			ReverbModDepth:         0.002,
			ReverbModRate:          0.1,
			HarmonicBassEnabled:    false,
			HarmonicBassFrequency:  80,
			HarmonicBassInputGain:  1,
			HarmonicBassHighGain:   1,
			HarmonicBassOriginal:   1,
			HarmonicBassHarmonic:   0,
			HarmonicBassDecay:      0,
			HarmonicBassResponseMs: 20,
			HarmonicBassHighpass:   0,
		},
		compParams: CompressorParams{
			Enabled:      false,
			ThresholdDB:  -20,
			Ratio:        4,
			KneeDB:       6,
			AttackMs:     10,
			ReleaseMs:    100,
			MakeupGainDB: 0,
			AutoMakeup:   true,
		},
		limParams: LimiterParams{
			Enabled:   true,
			Threshold: -0.1,
			Release:   100,
		},
		spectrum: SpectrumParams{
			FFTSize:   2048,
			Overlap:   0.75,
			Window:    "blackmanharris",
			Smoothing: 0.65,
		},
	}

	err := e.initSpectrumAnalyzer()
	if err != nil {
		return nil, err
	}

	chorus, err := modulation.NewChorus()
	if err != nil {
		return nil, err
	}

	e.chorus = chorus

	flanger, err := modulation.NewFlanger(sampleRate)
	if err != nil {
		return nil, err
	}

	e.flanger = flanger

	ringMod, err := modulation.NewRingModulator(sampleRate)
	if err != nil {
		return nil, err
	}

	e.ringMod = ringMod

	crusher, err := effects.NewBitCrusher(sampleRate)
	if err != nil {
		return nil, err
	}

	e.crusher = crusher

	widener, err := spatial.NewStereoWidener(sampleRate)
	if err != nil {
		return nil, err
	}

	e.widener = widener

	phaser, err := modulation.NewPhaser(sampleRate)
	if err != nil {
		return nil, err
	}

	e.phaser = phaser

	tremolo, err := modulation.NewTremolo(sampleRate)
	if err != nil {
		return nil, err
	}

	e.tremolo = tremolo

	delay, err := effects.NewDelay(sampleRate)
	if err != nil {
		return nil, err
	}

	e.delay = delay
	e.reverb = reverb.NewReverb()

	fdn, err := reverb.NewFDNReverb(sampleRate)
	if err != nil {
		return nil, err
	}

	e.fdn = fdn

	bass, err := effects.NewHarmonicBass(sampleRate)
	if err != nil {
		return nil, err
	}

	e.bass = bass

	tp, err := pitch.NewPitchShifter(sampleRate)
	if err != nil {
		return nil, err
	}

	e.tp = tp

	sp, err := pitch.NewSpectralPitchShifter(sampleRate)
	if err != nil {
		return nil, err
	}

	e.sp = sp

	comp, err := dynamics.NewCompressor(sampleRate)
	if err != nil {
		return nil, err
	}

	e.compressor = comp

	lim, err := dynamics.NewLimiter(sampleRate)
	if err != nil {
		return nil, err
	}

	e.limiter = lim

	// Load the embedded IR library (best-effort; demo works without it).
	lib, err := loadEmbeddedIRLib()
	if err == nil {
		e.irLib = lib
	}

	err = e.rebuildEffects()
	if err != nil {
		return nil, err
	}

	err = e.rebuildCompressor()
	if err != nil {
		return nil, err
	}

	err = e.rebuildLimiter()
	if err != nil {
		return nil, err
	}

	for i := range stepCount {
		e.steps[i] = StepConfig{Enabled: i%4 == 0, FreqHz: defaultStepFreq(i)}
	}

	err = e.rebuildEQ()
	if err != nil {
		return nil, err
	}

	e.samplesUntilNextStep = e.stepDurationSamples()

	return e, nil
}

// CurrentStep returns the currently playing step index.
func (e *Engine) CurrentStep() int {
	return e.currentStep
}

// GetIRNames returns the names of available impulse responses, or nil if none loaded.
func (e *Engine) GetIRNames() []string {
	if e.irLib == nil {
		return nil
	}

	return e.irLib.IRNames()
}

// Render fills dst with mono PCM samples in [-1, 1].
func (e *Engine) Render(dst []float32) {
	if len(dst) == 0 {
		return
	}

	block := e.ensureRenderBlock(len(dst))

	for i := range dst {
		if e.running {
			e.samplesUntilNextStep--
			for e.samplesUntilNextStep <= 0 {
				stepIndex := e.currentStep
				e.triggerCurrentStep()
				e.currentStep = (stepIndex + 1) % stepCount
				e.samplesUntilNextStep += e.stepDurationSamplesForStep(stepIndex)
			}
		}

		block[i] = e.nextSample()
	}

	e.processEffectsInPlace(block)
	e.hp.ProcessBlock(block)
	e.low.ProcessBlock(block)
	e.mid.ProcessBlock(block)
	e.high.ProcessBlock(block)
	e.lp.ProcessBlock(block)

	if e.eq.Master != 1 {
		for i := range block {
			block[i] *= e.eq.Master
		}
	}

	if e.compParams.Enabled {
		e.compressor.ProcessInPlace(block)
	}

	if e.limParams.Enabled {
		e.limiter.ProcessInPlace(block)
	}

	for i, x := range block {
		e.pushSpectrumSample(x)
		dst[i] = float32(clamp(x, -1, 1))
	}
}

func (e *Engine) ensureRenderBlock(n int) []float64 {
	if cap(e.renderBlock) < n {
		e.renderBlock = make([]float64, n)
		return e.renderBlock
	}

	e.renderBlock = e.renderBlock[:n]

	return e.renderBlock
}

// ResponseCurveDB returns EQ magnitude response in dB for freqs.
func (e *Engine) ResponseCurveDB(freqs []float64) []float64 {
	out := make([]float64, len(freqs))
	for i, f := range freqs {
		f = clamp(f, 1, e.sampleRate*0.49)
		h := e.hp.Response(f, e.sampleRate)
		h *= e.low.Response(f, e.sampleRate)
		h *= e.mid.Response(f, e.sampleRate)
		h *= e.high.Response(f, e.sampleRate)
		h *= e.lp.Response(f, e.sampleRate)
		mag := cmplx.Abs(h)
		out[i] = 20 * math.Log10(math.Max(1e-12, mag))
	}

	return out
}

// NodeResponseCurveDB returns one EQ node magnitude response in dB for freqs.
func (e *Engine) NodeResponseCurveDB(node string, freqs []float64) []float64 {
	chain := e.hp

	switch node {
	case "hp":
		chain = e.hp
	case "low":
		chain = e.low
	case "mid":
		chain = e.mid
	case "high":
		chain = e.high
	case "lp":
		chain = e.lp
	}

	out := make([]float64, len(freqs))
	for i, f := range freqs {
		f = clamp(f, 1, e.sampleRate*0.49)
		h := chain.Response(f, e.sampleRate)
		mag := cmplx.Abs(h)
		out[i] = 20 * math.Log10(math.Max(1e-12, mag))
	}

	return out
}

// CompressorCurveDB returns the compressor output levels in dB for given input levels in dB.
func (e *Engine) CompressorCurveDB(inputsDB []float64) []float64 {
	out := make([]float64, len(inputsDB))
	for i, db := range inputsDB {
		lin := math.Pow(10, db/20.0)
		outLin := e.compressor.CalculateOutputLevel(lin)
		out[i] = 20 * math.Log10(math.Max(1e-12, outLin))
	}

	return out
}

// LimiterCurveDB returns the limiter output levels in dB for given input levels in dB.
func (e *Engine) LimiterCurveDB(inputsDB []float64) []float64 {
	out := make([]float64, len(inputsDB))
	for i, db := range inputsDB {
		lin := math.Pow(10, db/20.0)
		outLin := e.limiter.CalculateOutputLevel(lin)
		out[i] = 20 * math.Log10(math.Max(1e-12, outLin))
	}

	return out
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
