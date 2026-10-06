package effectchain

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects/dynamics"
	"github.com/cwbudde/algo-dsp/dsp/effects/modulation"
	"github.com/cwbudde/algo-dsp/dsp/effects/spatial"
	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
	"github.com/cwbudde/algo-dsp/dsp/filter/weighting"
)

type equalizerRuntime struct{ fx *biquad.Chain }

func (r *equalizerRuntime) Configure(ctx Context, p Params) error {
	var coeffs []biquad.Coefficients

	switch p.Type {
	case "filter-a-weighting":
		r.fx = weighting.New(weighting.TypeA, ctx.SampleRate)
		return nil
	case "filter-c-weighting":
		r.fx = weighting.New(weighting.TypeC, ctx.SampleRate)
		return nil
	case "eq-graphic":
		order := min(max(int(math.Round(p.GetNum("order", 4))), 4), 12)
		if order%2 != 0 {
			order++
		}

		var (
			centers, gains []float64
			coincident     int
		)

		for i, hz := range graphicCenters {
			hz = min(hz, ctx.SampleRate*0.45)
			gain := clamp(p.GetNum(fmt.Sprintf("gain%dDB", i+1), 0), -24, 24)

			last := len(centers) - 1
			if last >= 0 && centers[last] == hz {
				// Low sample rates can place several upper bands at the same
				// center. Average their controls instead of stacking their gains.
				coincident++
				gains[last] += (gain - gains[last]) / float64(coincident)

				continue
			}

			centers = append(centers, hz)
			gains = append(gains, gain)
			coincident = 1
		}

		var err error

		coeffs, err = design.GraphicEQ(ctx.SampleRate, centers, gains, order)
		if err != nil {
			return fmt.Errorf("effectchain: graphic EQ: %w", err)
		}
	default:
		count := min(max(int(math.Round(p.GetNum("bands", 4))), 1), 8)
		for i := 1; i <= count; i++ {
			prefix := fmt.Sprintf("band%d", i)
			hz := clamp(p.GetNum(prefix+"FreqHz", min(80*math.Exp2(float64(i-1)), ctx.SampleRate*0.45)), 20, ctx.SampleRate*0.49)
			gain := clamp(p.GetNum(prefix+"GainDB", 0), -24, 24)
			q := clamp(p.GetNum(prefix+"Q", 1), 0.2, 8)

			kind := p.Str[prefix+"Type"]
			if kind == "" {
				kind = "peak"
			}

			order := min(max(int(math.Round(p.GetNum(prefix+"Order", 2))), 2), 12)
			if order%2 != 0 {
				order++
			}

			sections, err := design.ParametricBand(ctx.SampleRate, hz, gain, q, kind, order)
			if err != nil {
				return fmt.Errorf("effectchain: band %d: %w", i, err)
			}

			coeffs = append(coeffs, sections...)
		}
	}

	fresh := biquad.NewChain(coeffs)
	if r.fx != nil && r.fx.NumSections() == fresh.NumSections() {
		for i := range fresh.NumSections() {
			r.fx.Section(i).Coefficients = fresh.Section(i).Coefficients
		}

		r.fx.SetGain(fresh.Gain())
	} else {
		r.fx = fresh
	}

	return nil
}
func (r *equalizerRuntime) Process(block []float64) { r.fx.ProcessBlock(block) }

type dynamicEQRuntime struct {
	fx    *dynamics.DynamicEQ
	count int
}

func (r *dynamicEQRuntime) Configure(ctx Context, p Params) error {
	count := min(max(int(math.Round(p.GetNum("bands", 4))), 1), 8)
	if r.fx == nil || r.count != count {
		fx, err := dynamics.NewDynamicEQ(ctx.SampleRate)
		if err != nil {
			return wrapConfigureErr(err)
		}

		r.fx = fx
		r.count = 0
	}

	for i := 1; i <= count; i++ {
		prefix := fmt.Sprintf("band%d", i)
		kind := dynamics.EQBandPeak

		switch p.Str[prefix+"Type"] {
		case "lowshelf":
			kind = dynamics.EQBandLowShelf
		case "highshelf":
			kind = dynamics.EQBandHighShelf
		}

		mode := dynamics.EQBandModeDownward

		switch p.Str[prefix+"Mode"] {
		case "static":
			mode = dynamics.EQBandModeStatic
		case "upward":
			mode = dynamics.EQBandModeUpward
		case "upward-below":
			mode = dynamics.EQBandModeUpwardBelow
		}

		knee := clamp(p.GetNum(prefix+"KneeDB", 6), 0, 24)
		cfg := dynamics.EQBandConfig{Type: kind, FrequencyHz: clamp(p.GetNum(prefix+"FreqHz", min(80*math.Exp2(float64(i-1)), ctx.SampleRate*0.45)), 20, ctx.SampleRate*0.49), Q: clamp(p.GetNum(prefix+"Q", 1), 0.2, 8), StaticGainDB: clamp(p.GetNum(prefix+"GainDB", 0), -24, 24), Mode: mode, ThresholdDB: clamp(p.GetNum(prefix+"ThresholdDB", -24), -80, 0), Ratio: clamp(p.GetNum(prefix+"Ratio", 2), 1, 20), KneeDB: &knee, AttackMs: clamp(p.GetNum(prefix+"AttackMs", 10), 0.1, 1000), ReleaseMs: clamp(p.GetNum(prefix+"ReleaseMs", 100), 1, 5000), RangeDB: clamp(p.GetNum(prefix+"RangeDB", 12), 0, 24)}

		var err error
		if r.count < i {
			_, err = r.fx.AddBand(cfg)
			r.count = i
		} else {
			err = r.fx.SetBandConfig(i-1, cfg)
		}

		if err != nil {
			return wrapConfigureErr(err)
		}

		if err = r.fx.SetBandRange(i-1, cfg.RangeDB); err != nil {
			return wrapConfigureErr(err)
		}
	}

	return nil
}
func (r *dynamicEQRuntime) Process(block []float64) { r.fx.ProcessInPlace(block) }

type autoWahRuntime struct{ fx *modulation.AutoWah }

func (r *autoWahRuntime) Configure(ctx Context, p Params) error {
	lo := clamp(p.GetNum("minFreqHz", 300), 20, ctx.SampleRate*0.45)

	hi := clamp(p.GetNum("maxFreqHz", 2200), lo+1, ctx.SampleRate*0.49)
	for _, err := range []error{r.fx.SetSampleRate(ctx.SampleRate), r.fx.SetFrequencyRangeHz(lo, hi), r.fx.SetQ(clamp(p.GetNum("q", 0.8), 0.1, 20)), r.fx.SetSensitivity(clamp(p.GetNum("sensitivity", 2), 0, 20)), r.fx.SetAttackMs(clamp(p.GetNum("attackMs", 2), 0.1, 1000)), r.fx.SetReleaseMs(clamp(p.GetNum("releaseMs", 80), 1, 5000)), r.fx.SetMix(clamp(p.GetNum("mix", 1), 0, 1))} {
		if err != nil {
			return wrapConfigureErr(err)
		}
	}

	return nil
}
func (r *autoWahRuntime) Process(block []float64) { _ = r.fx.ProcessInPlace(block) }

type frequencyShifterRuntime struct {
	fx   *modulation.FrequencyShifter
	down bool
	mix  float64
}

func (r *frequencyShifterRuntime) Configure(ctx Context, p Params) error {
	if err := r.fx.SetSampleRate(ctx.SampleRate); err != nil {
		return wrapConfigureErr(err)
	}

	if err := r.fx.SetShiftHz(clamp(p.GetNum("shiftHz", 100), 0, ctx.SampleRate*0.49)); err != nil {
		return wrapConfigureErr(err)
	}

	r.down = p.Str["direction"] == "down"
	r.mix = clamp(p.GetNum("mix", 1), 0, 1)

	return nil
}

func (r *frequencyShifterRuntime) Process(block []float64) {
	for i, dry := range block {
		wet := 0.0
		if r.down {
			wet = r.fx.ProcessDownshiftSample(dry)
		} else {
			wet = r.fx.ProcessUpshiftSample(dry)
		}

		block[i] = dry + (wet-dry)*r.mix
	}
}

type spatialRuntime struct {
	panner *spatial.StereoPanner
	haas   *spatial.HaasDelay
	cross  *spatial.CrosstalkSimulator
}

func (r *spatialRuntime) Configure(ctx Context, p Params) error {
	var errs []error

	switch {
	case r.panner != nil:
		law := spatial.PanLawEqualPower

		switch p.Str["law"] {
		case "linear":
			law = spatial.PanLawLinear
		case "compromise":
			law = spatial.PanLawCompromise
		}

		errs = []error{r.panner.SetSampleRate(ctx.SampleRate), r.panner.SetPosition(clamp(p.GetNum("position", 0), -1, 1)), r.panner.SetLaw(law), r.panner.SetAutoPanRate(clamp(p.GetNum("autoPanRateHz", 0), 0, 20)), r.panner.SetAutoPanDepth(clamp(p.GetNum("autoPanDepth", 0), 0, 1))}
	case r.haas != nil:
		channel := spatial.HaasChannelRight
		if p.Str["channel"] == "left" {
			channel = spatial.HaasChannelLeft
		}

		errs = []error{r.haas.SetSampleRate(ctx.SampleRate), r.haas.SetDelayMs(clamp(p.GetNum("delayMs", 15), 0, 40)), r.haas.SetChannel(channel)}
	case r.cross != nil:
		preset := spatial.CrosstalkPresetHandcrafted

		switch p.Str["preset"] {
		case "ircam":
			preset = spatial.CrosstalkPresetIRCAM
		case "hdphx":
			preset = spatial.CrosstalkPresetHDPHX
		}

		errs = []error{r.cross.SetSampleRate(ctx.SampleRate), r.cross.SetDiameter(clamp(p.GetNum("diameter", 0.175), 0.01, 1)), r.cross.SetSpeedOfSound(clamp(p.GetNum("speedOfSound", 343), 100, 1000)), r.cross.SetCrossfeedMix(clamp(p.GetNum("crossfeedMix", 0.2), 0, 1)), r.cross.SetPreset(preset)}
		r.cross.SetPolarityInvert(p.GetNum("polarityInvert", 0) >= 0.5)
	}

	for _, err := range errs {
		if err != nil {
			return wrapConfigureErr(err)
		}
	}

	return nil
}

func (r *spatialRuntime) Process(block []float64) {
	for i, x := range block {
		l, r := r.processStereo(x, x)
		block[i] = (l + r) * 0.5
	}
}

func (r *spatialRuntime) processStereo(l, rgt float64) (float64, float64) {
	switch {
	case r.panner != nil:
		return r.panner.ProcessStereo(l, rgt)
	case r.haas != nil:
		return r.haas.ProcessStereo(l, rgt)
	default:
		return r.cross.ProcessStereo(l, rgt)
	}
}

func (r *spatialRuntime) ProcessStereo(l, rgt []float64) {
	for i := range l {
		l[i], rgt[i] = r.processStereo(l[i], rgt[i])
	}
}

func registerExtended(r *Registry) {
	for _, id := range []string{"eq-parametric", "eq-graphic", "filter-a-weighting", "filter-c-weighting"} {
		r.MustRegister(id, func(Context) (Runtime, error) { return &equalizerRuntime{}, nil })
	}

	r.MustRegister("dyn-eq", func(Context) (Runtime, error) { return &dynamicEQRuntime{}, nil })
	r.MustRegister("auto-wah", func(ctx Context) (Runtime, error) {
		fx, err := modulation.NewAutoWah(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("auto-wah", err)
		}

		return &autoWahRuntime{fx: fx}, nil
	})
	r.MustRegister("frequency-shifter", func(ctx Context) (Runtime, error) {
		fx, err := modulation.NewFrequencyShifter(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("frequency-shifter", err)
		}

		return &frequencyShifterRuntime{fx: fx}, nil
	})
	r.MustRegister("panner", func(ctx Context) (Runtime, error) {
		fx, err := spatial.NewStereoPanner(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("panner", err)
		}

		return &spatialRuntime{panner: fx}, nil
	})
	r.MustRegister("haas", func(ctx Context) (Runtime, error) {
		fx, err := spatial.NewHaasDelay(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("haas", err)
		}

		return &spatialRuntime{haas: fx}, nil
	})
	r.MustRegister("crosstalk", func(ctx Context) (Runtime, error) {
		fx, err := spatial.NewCrosstalkSimulator(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("crosstalk", err)
		}

		return &spatialRuntime{cross: fx}, nil
	})
}
