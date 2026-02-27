package effectchain

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/core"
	"github.com/cwbudde/algo-dsp/dsp/effects/dynamics"
)

type compressorRuntime struct {
	fx *dynamics.Compressor
}

func (r *compressorRuntime) Configure(ctx Context, p Params) error {
	err := r.fx.SetSampleRate(ctx.SampleRate)
	if err != nil {
		return fmt.Errorf("effectchain: configure compressor sample rate: %w", err)
	}

	err = r.fx.SetThreshold(core.Clamp(p.GetNum("thresholdDB", -20), -60, 0))
	if err != nil {
		return fmt.Errorf("effectchain: configure compressor threshold: %w", err)
	}

	err = r.fx.SetRatio(core.Clamp(p.GetNum("ratio", 4), 1, 100))
	if err != nil {
		return fmt.Errorf("effectchain: configure compressor ratio: %w", err)
	}

	err = r.fx.SetKnee(core.Clamp(p.GetNum("kneeDB", 6), 0, 24))
	if err != nil {
		return fmt.Errorf("effectchain: configure compressor knee: %w", err)
	}

	err = r.fx.SetAttack(core.Clamp(p.GetNum("attackMs", 10), 0.1, 1000))
	if err != nil {
		return fmt.Errorf("effectchain: configure compressor attack: %w", err)
	}

	err = r.fx.SetRelease(core.Clamp(p.GetNum("releaseMs", 100), 1, 5000))
	if err != nil {
		return fmt.Errorf("effectchain: configure compressor release: %w", err)
	}

	err = r.fx.SetAutoMakeup(false)
	if err != nil {
		return fmt.Errorf("effectchain: configure compressor auto makeup: %w", err)
	}

	err = r.fx.SetMakeupGain(core.Clamp(p.GetNum("makeupGainDB", 0), 0, 24))
	if err != nil {
		return fmt.Errorf("effectchain: configure compressor makeup gain: %w", err)
	}

	return nil
}

func (r *compressorRuntime) Process(block []float64) {
	r.fx.ProcessInPlace(block)
}

type limiterRuntime struct {
	fx *dynamics.Limiter
}

func (r *limiterRuntime) Configure(ctx Context, p Params) error {
	err := r.fx.SetSampleRate(ctx.SampleRate)
	if err != nil {
		return fmt.Errorf("effectchain: configure limiter sample rate: %w", err)
	}

	err = r.fx.SetThreshold(core.Clamp(p.GetNum("thresholdDB", -0.1), -24, 0))
	if err != nil {
		return fmt.Errorf("effectchain: configure limiter threshold: %w", err)
	}

	err = r.fx.SetRelease(core.Clamp(p.GetNum("releaseMs", 100), 1, 5000))
	if err != nil {
		return fmt.Errorf("effectchain: configure limiter release: %w", err)
	}

	return nil
}

func (r *limiterRuntime) Process(block []float64) {
	r.fx.ProcessInPlace(block)
}

type lookaheadLimiterRuntime struct {
	fx *dynamics.LookaheadLimiter
}

func (r *lookaheadLimiterRuntime) Configure(ctx Context, p Params) error {
	err := r.fx.SetSampleRate(ctx.SampleRate)
	if err != nil {
		return fmt.Errorf("effectchain: configure lookahead limiter sample rate: %w", err)
	}

	err = r.fx.SetThreshold(core.Clamp(p.GetNum("thresholdDB", -1), -24, 0))
	if err != nil {
		return fmt.Errorf("effectchain: configure lookahead limiter threshold: %w", err)
	}

	err = r.fx.SetRelease(core.Clamp(p.GetNum("releaseMs", 100), 1, 5000))
	if err != nil {
		return fmt.Errorf("effectchain: configure lookahead limiter release: %w", err)
	}

	err = r.fx.SetLookahead(core.Clamp(p.GetNum("lookaheadMs", 3), 0, 200))
	if err != nil {
		return fmt.Errorf("effectchain: configure lookahead limiter lookahead: %w", err)
	}

	return nil
}

func (r *lookaheadLimiterRuntime) Process(block []float64) {
	r.fx.ProcessInPlace(block)
}

func (r *lookaheadLimiterRuntime) ProcessWithSidechain(program, sidechain []float64) {
	r.fx.ProcessInPlaceSidechain(program, sidechain)
}

type gateRuntime struct {
	fx *dynamics.Gate
}

func (r *gateRuntime) Configure(ctx Context, p Params) error {
	err := r.fx.SetSampleRate(ctx.SampleRate)
	if err != nil {
		return fmt.Errorf("effectchain: configure gate sample rate: %w", err)
	}

	err = r.fx.SetThreshold(core.Clamp(p.GetNum("thresholdDB", -40), -80, 0))
	if err != nil {
		return fmt.Errorf("effectchain: configure gate threshold: %w", err)
	}

	err = r.fx.SetRatio(core.Clamp(p.GetNum("ratio", 10), 1, 100))
	if err != nil {
		return fmt.Errorf("effectchain: configure gate ratio: %w", err)
	}

	err = r.fx.SetKnee(core.Clamp(p.GetNum("kneeDB", 6), 0, 24))
	if err != nil {
		return fmt.Errorf("effectchain: configure gate knee: %w", err)
	}

	err = r.fx.SetAttack(core.Clamp(p.GetNum("attackMs", 0.1), 0.1, 1000))
	if err != nil {
		return fmt.Errorf("effectchain: configure gate attack: %w", err)
	}

	err = r.fx.SetHold(core.Clamp(p.GetNum("holdMs", 50), 0, 5000))
	if err != nil {
		return fmt.Errorf("effectchain: configure gate hold: %w", err)
	}

	err = r.fx.SetRelease(core.Clamp(p.GetNum("releaseMs", 100), 1, 5000))
	if err != nil {
		return fmt.Errorf("effectchain: configure gate release: %w", err)
	}

	err = r.fx.SetRange(core.Clamp(p.GetNum("rangeDB", -80), -120, 0))
	if err != nil {
		return fmt.Errorf("effectchain: configure gate range: %w", err)
	}

	return nil
}

func (r *gateRuntime) Process(block []float64) {
	r.fx.ProcessInPlace(block)
}

type expanderRuntime struct {
	fx *dynamics.Expander
}

func (r *expanderRuntime) Configure(ctx Context, p Params) error {
	err := r.fx.SetSampleRate(ctx.SampleRate)
	if err != nil {
		return fmt.Errorf("effectchain: configure expander sample rate: %w", err)
	}

	err = r.fx.SetThreshold(core.Clamp(p.GetNum("thresholdDB", -35), -80, 0))
	if err != nil {
		return fmt.Errorf("effectchain: configure expander threshold: %w", err)
	}

	err = r.fx.SetRatio(core.Clamp(p.GetNum("ratio", 2), 1, 100))
	if err != nil {
		return fmt.Errorf("effectchain: configure expander ratio: %w", err)
	}

	err = r.fx.SetKnee(core.Clamp(p.GetNum("kneeDB", 6), 0, 24))
	if err != nil {
		return fmt.Errorf("effectchain: configure expander knee: %w", err)
	}

	err = r.fx.SetAttack(core.Clamp(p.GetNum("attackMs", 1), 0.1, 1000))
	if err != nil {
		return fmt.Errorf("effectchain: configure expander attack: %w", err)
	}

	err = r.fx.SetRelease(core.Clamp(p.GetNum("releaseMs", 100), 1, 5000))
	if err != nil {
		return fmt.Errorf("effectchain: configure expander release: %w", err)
	}

	err = r.fx.SetRange(core.Clamp(p.GetNum("rangeDB", -60), -120, 0))
	if err != nil {
		return fmt.Errorf("effectchain: configure expander range: %w", err)
	}

	err = r.fx.SetTopology(normalizeDynamicsTopology(p.Str["topology"]))
	if err != nil {
		return fmt.Errorf("effectchain: configure expander topology: %w", err)
	}

	err = r.fx.SetDetectorMode(normalizeDynamicsDetectorMode(p.Str["detector"]))
	if err != nil {
		return fmt.Errorf("effectchain: configure expander detector: %w", err)
	}

	err = r.fx.SetRMSWindow(core.Clamp(p.GetNum("rmsWindowMs", 30), 1, 1000))
	if err != nil {
		return fmt.Errorf("effectchain: configure expander RMS window: %w", err)
	}

	return nil
}

func (r *expanderRuntime) Process(block []float64) {
	r.fx.ProcessInPlace(block)
}

type deesserRuntime struct {
	fx *dynamics.DeEsser
}

func (r *deesserRuntime) Configure(ctx Context, p Params) error {
	err := r.fx.SetSampleRate(ctx.SampleRate)
	if err != nil {
		return fmt.Errorf("effectchain: configure de-esser sample rate: %w", err)
	}

	err = r.fx.SetFrequency(core.Clamp(p.GetNum("freqHz", 6000), 1000, ctx.SampleRate*0.49))
	if err != nil {
		return fmt.Errorf("effectchain: configure de-esser frequency: %w", err)
	}

	err = r.fx.SetQ(core.Clamp(p.GetNum("q", 1.5), 0.1, 10))
	if err != nil {
		return fmt.Errorf("effectchain: configure de-esser q: %w", err)
	}

	err = r.fx.SetThreshold(core.Clamp(p.GetNum("thresholdDB", -20), -80, 0))
	if err != nil {
		return fmt.Errorf("effectchain: configure de-esser threshold: %w", err)
	}

	err = r.fx.SetRatio(core.Clamp(p.GetNum("ratio", 4), 1, 100))
	if err != nil {
		return fmt.Errorf("effectchain: configure de-esser ratio: %w", err)
	}

	err = r.fx.SetKnee(core.Clamp(p.GetNum("kneeDB", 3), 0, 12))
	if err != nil {
		return fmt.Errorf("effectchain: configure de-esser knee: %w", err)
	}

	err = r.fx.SetAttack(core.Clamp(p.GetNum("attackMs", 0.5), 0.01, 50))
	if err != nil {
		return fmt.Errorf("effectchain: configure de-esser attack: %w", err)
	}

	err = r.fx.SetRelease(core.Clamp(p.GetNum("releaseMs", 20), 1, 500))
	if err != nil {
		return fmt.Errorf("effectchain: configure de-esser release: %w", err)
	}

	err = r.fx.SetRange(core.Clamp(p.GetNum("rangeDB", -24), -60, 0))
	if err != nil {
		return fmt.Errorf("effectchain: configure de-esser range: %w", err)
	}

	err = r.fx.SetMode(normalizeDeesserMode(p.Str["mode"]))
	if err != nil {
		return fmt.Errorf("effectchain: configure de-esser mode: %w", err)
	}

	err = r.fx.SetDetector(normalizeDeesserDetector(p.Str["detector"]))
	if err != nil {
		return fmt.Errorf("effectchain: configure de-esser detector: %w", err)
	}

	order := min(max(int(math.Round(p.GetNum("filterOrder", 2))), 1), 4)

	err = r.fx.SetFilterOrder(order)
	if err != nil {
		return fmt.Errorf("effectchain: configure de-esser filter order: %w", err)
	}

	r.fx.SetListen(p.GetNum("listen", 0) >= 0.5)

	return nil
}

func (r *deesserRuntime) Process(block []float64) {
	r.fx.ProcessInPlace(block)
}

type transientShaperRuntime struct {
	fx *dynamics.TransientShaper
}

func (r *transientShaperRuntime) Configure(ctx Context, p Params) error {
	err := r.fx.SetSampleRate(ctx.SampleRate)
	if err != nil {
		return fmt.Errorf("effectchain: configure transient shaper sample rate: %w", err)
	}

	err = r.fx.SetAttackAmount(core.Clamp(p.GetNum("attack", 0), -1, 1))
	if err != nil {
		return fmt.Errorf("effectchain: configure transient shaper attack amount: %w", err)
	}

	err = r.fx.SetSustainAmount(core.Clamp(p.GetNum("sustain", 0), -1, 1))
	if err != nil {
		return fmt.Errorf("effectchain: configure transient shaper sustain amount: %w", err)
	}

	err = r.fx.SetAttack(core.Clamp(p.GetNum("attackMs", 10), 0.1, 200))
	if err != nil {
		return fmt.Errorf("effectchain: configure transient shaper attack: %w", err)
	}

	err = r.fx.SetRelease(core.Clamp(p.GetNum("releaseMs", 120), 1, 2000))
	if err != nil {
		return fmt.Errorf("effectchain: configure transient shaper release: %w", err)
	}

	return nil
}

func (r *transientShaperRuntime) Process(block []float64) {
	r.fx.ProcessInPlace(block)
}

type multibandRuntime struct {
	fx        *dynamics.MultibandCompressor
	lastBands int
	lastOrder int
	lastC1    float64
	lastC2    float64
	lastSR    float64
}

//nolint:cyclop
func (r *multibandRuntime) Configure(ctx Context, p Params) error {
	bands := min(max(int(math.Round(p.GetNum("bands", 3))), 2), 3)

	order := min(max(int(math.Round(p.GetNum("order", 4))), 2), 24)

	if order%2 != 0 {
		order++
	}

	c1 := core.Clamp(p.GetNum("cross1Hz", 250), 40, ctx.SampleRate*0.2)
	c2 := core.Clamp(p.GetNum("cross2Hz", 3000), c1+100, ctx.SampleRate*0.45)

	rebuild := r.fx == nil ||
		r.lastBands != bands ||
		r.lastOrder != order ||
		math.Abs(r.lastC1-c1) > 1e-9 ||
		math.Abs(r.lastC2-c2) > 1e-9 ||
		math.Abs(r.lastSR-ctx.SampleRate) > 1e-9

	if rebuild {
		freqs := []float64{c1}
		if bands == 3 {
			freqs = append(freqs, c2)
		}

		fx, err := dynamics.NewMultibandCompressor(freqs, order, ctx.SampleRate)
		if err != nil {
			return fmt.Errorf("effectchain: create multiband compressor: %w", err)
		}

		r.fx = fx
		r.lastBands = bands
		r.lastOrder = order
		r.lastC1 = c1
		r.lastC2 = c2
		r.lastSR = ctx.SampleRate
	}

	err := r.fx.SetBandThreshold(0, core.Clamp(p.GetNum("lowThresholdDB", -20), -80, 0))
	if err != nil {
		return fmt.Errorf("effectchain: configure multiband low threshold: %w", err)
	}

	err = r.fx.SetBandRatio(0, core.Clamp(p.GetNum("lowRatio", 2.5), 1, 20))
	if err != nil {
		return fmt.Errorf("effectchain: configure multiband low ratio: %w", err)
	}

	err = r.fx.SetBandThreshold(1, core.Clamp(p.GetNum("midThresholdDB", -18), -80, 0))
	if err != nil {
		return fmt.Errorf("effectchain: configure multiband mid threshold: %w", err)
	}

	err = r.fx.SetBandRatio(1, core.Clamp(p.GetNum("midRatio", 3.0), 1, 20))
	if err != nil {
		return fmt.Errorf("effectchain: configure multiband mid ratio: %w", err)
	}

	if bands == 3 {
		err = r.fx.SetBandThreshold(2, core.Clamp(p.GetNum("highThresholdDB", -14), -80, 0))
		if err != nil {
			return fmt.Errorf("effectchain: configure multiband high threshold: %w", err)
		}

		err = r.fx.SetBandRatio(2, core.Clamp(p.GetNum("highRatio", 4.0), 1, 20))
		if err != nil {
			return fmt.Errorf("effectchain: configure multiband high ratio: %w", err)
		}
	}

	attack := core.Clamp(p.GetNum("attackMs", 8), 0.1, 1000)
	release := core.Clamp(p.GetNum("releaseMs", 120), 1, 5000)
	knee := core.Clamp(p.GetNum("kneeDB", 6), 0, 24)
	makeup := core.Clamp(p.GetNum("makeupGainDB", 0), 0, 24)
	autoMakeup := p.GetNum("autoMakeup", 0) >= 0.5

	for b := range r.fx.NumBands() {
		err := r.fx.SetBandAttack(b, attack)
		if err != nil {
			return fmt.Errorf("effectchain: configure multiband attack for band %d: %w", b, err)
		}

		err = r.fx.SetBandRelease(b, release)
		if err != nil {
			return fmt.Errorf("effectchain: configure multiband release for band %d: %w", b, err)
		}

		err = r.fx.SetBandKnee(b, knee)
		if err != nil {
			return fmt.Errorf("effectchain: configure multiband knee for band %d: %w", b, err)
		}

		err = r.fx.SetBandAutoMakeup(b, autoMakeup)
		if err != nil {
			return fmt.Errorf("effectchain: configure multiband auto makeup for band %d: %w", b, err)
		}

		if !autoMakeup {
			err = r.fx.SetBandMakeupGain(b, makeup)
			if err != nil {
				return fmt.Errorf("effectchain: configure multiband makeup gain for band %d: %w", b, err)
			}
		}
	}

	return nil
}

func (r *multibandRuntime) Process(block []float64) {
	if r.fx == nil {
		return
	}

	r.fx.ProcessInPlace(block)
}
