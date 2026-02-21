//go:build js && wasm

package main

import (
	"syscall/js"

	"github.com/cwbudde/algo-dsp/internal/webdemo"
)

var (
	engine *webdemo.Engine
	funcs  []js.Func
)

func main() {
	api := js.Global().Get("Object").New()
	api.Set("init", export(func(args []js.Value) any {
		sr := 48000.0
		if len(args) > 0 {
			sr = args[0].Float()
		}
		e, err := webdemo.NewEngine(sr)
		if err != nil {
			return err.Error()
		}
		engine = e
		return js.Null()
	}))

	api.Set("setTransport", export(func(args []js.Value) any {
		if engine == nil || len(args) < 2 {
			return js.Null()
		}
		shuffle := 0.0
		if len(args) >= 3 {
			shuffle = args[2].Float()
		}
		engine.SetTransport(args[0].Float(), args[1].Float(), shuffle)
		return js.Null()
	}))

	api.Set("setRunning", export(func(args []js.Value) any {
		if engine == nil || len(args) < 1 {
			return js.Null()
		}
		engine.SetRunning(args[0].Bool())
		return js.Null()
	}))

	api.Set("setWaveform", export(func(args []js.Value) any {
		if engine == nil || len(args) < 1 {
			return js.Null()
		}
		engine.SetWaveform(args[0].String())
		return js.Null()
	}))

	api.Set("setSteps", export(func(args []js.Value) any {
		if engine == nil || len(args) < 1 {
			return js.Null()
		}
		arr := args[0]
		steps := make([]webdemo.StepConfig, arr.Length())
		for i := 0; i < arr.Length(); i++ {
			item := arr.Index(i)
			steps[i] = webdemo.StepConfig{
				Enabled: item.Get("enabled").Bool(),
				FreqHz:  item.Get("freq").Float(),
			}
		}
		engine.SetSteps(steps)
		return js.Null()
	}))

	api.Set("setEQ", export(func(args []js.Value) any {
		if engine == nil || len(args) < 1 {
			return js.Null()
		}
		p := args[0]
		err := engine.SetEQ(webdemo.EQParams{
			HPFamily:   p.Get("hpFamily").String(),
			HPType:     p.Get("hpType").String(),
			HPOrder:    p.Get("hpOrder").Int(),
			HPFreq:     p.Get("hpFreq").Float(),
			HPGain:     p.Get("hpGain").Float(),
			HPQ:        p.Get("hpQ").Float(),
			LowFamily:  p.Get("lowFamily").String(),
			LowType:    p.Get("lowType").String(),
			LowOrder:   p.Get("lowOrder").Int(),
			LowFreq:    p.Get("lowFreq").Float(),
			LowGain:    p.Get("lowGain").Float(),
			LowQ:       p.Get("lowQ").Float(),
			MidFamily:  p.Get("midFamily").String(),
			MidType:    p.Get("midType").String(),
			MidOrder:   p.Get("midOrder").Int(),
			MidFreq:    p.Get("midFreq").Float(),
			MidGain:    p.Get("midGain").Float(),
			MidQ:       p.Get("midQ").Float(),
			HighFamily: p.Get("highFamily").String(),
			HighType:   p.Get("highType").String(),
			HighOrder:  p.Get("highOrder").Int(),
			HighFreq:   p.Get("highFreq").Float(),
			HighGain:   p.Get("highGain").Float(),
			HighQ:      p.Get("highQ").Float(),
			LPFamily:   p.Get("lpFamily").String(),
			LPType:     p.Get("lpType").String(),
			LPOrder:    p.Get("lpOrder").Int(),
			LPFreq:     p.Get("lpFreq").Float(),
			LPGain:     p.Get("lpGain").Float(),
			LPQ:        p.Get("lpQ").Float(),
			Master:     p.Get("master").Float(),
		})
		if err != nil {
			return err.Error()
		}
		return js.Null()
	}))

	api.Set("setEffects", export(func(args []js.Value) any {
		if engine == nil || len(args) < 1 {
			return js.Null()
		}
		p := args[0]
		err := engine.SetEffects(webdemo.EffectsParams{
			ChorusEnabled:          p.Get("chorusEnabled").Bool(),
			ChorusMix:              p.Get("chorusMix").Float(),
			ChorusDepth:            p.Get("chorusDepth").Float(),
			ChorusSpeedHz:          p.Get("chorusSpeedHz").Float(),
			ChorusStages:           p.Get("chorusStages").Int(),
			FlangerEnabled:         p.Get("flangerEnabled").Bool(),
			FlangerRateHz:          p.Get("flangerRateHz").Float(),
			FlangerDepth:           p.Get("flangerDepth").Float(),
			FlangerBaseDelay:       p.Get("flangerBaseDelay").Float(),
			FlangerFeedback:        p.Get("flangerFeedback").Float(),
			FlangerMix:             p.Get("flangerMix").Float(),
			RingModEnabled:         p.Get("ringModEnabled").Bool(),
			RingModCarrierHz:       p.Get("ringModCarrierHz").Float(),
			RingModMix:             p.Get("ringModMix").Float(),
			BitCrusherEnabled:      p.Get("bitCrusherEnabled").Bool(),
			BitCrusherBitDepth:     p.Get("bitCrusherBitDepth").Float(),
			BitCrusherDownsample:   p.Get("bitCrusherDownsample").Int(),
			BitCrusherMix:          p.Get("bitCrusherMix").Float(),
			WidenerEnabled:         p.Get("widenerEnabled").Bool(),
			WidenerWidth:           p.Get("widenerWidth").Float(),
			WidenerMix:             p.Get("widenerMix").Float(),
			PhaserEnabled:          p.Get("phaserEnabled").Bool(),
			PhaserRateHz:           p.Get("phaserRateHz").Float(),
			PhaserMinFreqHz:        p.Get("phaserMinFreqHz").Float(),
			PhaserMaxFreqHz:        p.Get("phaserMaxFreqHz").Float(),
			PhaserStages:           p.Get("phaserStages").Int(),
			PhaserFeedback:         p.Get("phaserFeedback").Float(),
			PhaserMix:              p.Get("phaserMix").Float(),
			TremoloEnabled:         p.Get("tremoloEnabled").Bool(),
			TremoloRateHz:          p.Get("tremoloRateHz").Float(),
			TremoloDepth:           p.Get("tremoloDepth").Float(),
			TremoloSmoothingMs:     p.Get("tremoloSmoothingMs").Float(),
			TremoloMix:             p.Get("tremoloMix").Float(),
			DelayEnabled:           p.Get("delayEnabled").Bool(),
			DelayTime:              p.Get("delayTime").Float(),
			DelayFeedback:          p.Get("delayFeedback").Float(),
			DelayMix:               p.Get("delayMix").Float(),
			TimePitchEnabled:       p.Get("timePitchEnabled").Bool(),
			TimePitchSemitones:     p.Get("timePitchSemitones").Float(),
			TimePitchSequence:      p.Get("timePitchSequence").Float(),
			TimePitchOverlap:       p.Get("timePitchOverlap").Float(),
			TimePitchSearch:        p.Get("timePitchSearch").Float(),
			SpectralPitchEnabled:   p.Get("spectralPitchEnabled").Bool(),
			SpectralPitchSemitones: p.Get("spectralPitchSemitones").Float(),
			SpectralPitchFrameSize: p.Get("spectralPitchFrameSize").Int(),
			SpectralPitchHop:       p.Get("spectralPitchHop").Int(),
			ReverbEnabled:          p.Get("reverbEnabled").Bool(),
			ReverbModel:            p.Get("reverbModel").String(),
			ReverbWet:              p.Get("reverbWet").Float(),
			ReverbDry:              p.Get("reverbDry").Float(),
			ReverbRoomSize:         p.Get("reverbRoomSize").Float(),
			ReverbDamp:             p.Get("reverbDamp").Float(),
			ReverbGain:             p.Get("reverbGain").Float(),
			ReverbRT60:             p.Get("reverbRT60").Float(),
			ReverbPreDelay:         p.Get("reverbPreDelay").Float(),
			ReverbModDepth:         p.Get("reverbModDepth").Float(),
			ReverbModRate:          p.Get("reverbModRate").Float(),
			HarmonicBassEnabled:    p.Get("harmonicBassEnabled").Bool(),
			HarmonicBassFrequency:  p.Get("harmonicBassFrequency").Float(),
			HarmonicBassInputGain:  p.Get("harmonicBassInputGain").Float(),
			HarmonicBassHighGain:   p.Get("harmonicBassHighGain").Float(),
			HarmonicBassOriginal:   p.Get("harmonicBassOriginal").Float(),
			HarmonicBassHarmonic:   p.Get("harmonicBassHarmonic").Float(),
			HarmonicBassDecay:      p.Get("harmonicBassDecay").Float(),
			HarmonicBassResponseMs: p.Get("harmonicBassResponseMs").Float(),
			HarmonicBassHighpass:   p.Get("harmonicBassHighpass").Int(),
			ChainGraphJSON:         p.Get("chainGraphJSON").String(),
		})
		if err != nil {
			return err.Error()
		}
		return js.Null()
	}))

	api.Set("setCompressor", export(func(args []js.Value) any {
		if engine == nil || len(args) < 1 {
			return js.Null()
		}
		p := args[0]
		err := engine.SetCompressor(webdemo.CompressorParams{
			Enabled:      p.Get("enabled").Bool(),
			ThresholdDB:  p.Get("thresholdDB").Float(),
			Ratio:        p.Get("ratio").Float(),
			KneeDB:       p.Get("kneeDB").Float(),
			AttackMs:     p.Get("attackMs").Float(),
			ReleaseMs:    p.Get("releaseMs").Float(),
			MakeupGainDB: p.Get("makeupGainDB").Float(),
			AutoMakeup:   p.Get("autoMakeup").Bool(),
		})
		if err != nil {
			return err.Error()
		}
		return js.Null()
	}))

	api.Set("setLimiter", export(func(args []js.Value) any {
		if engine == nil || len(args) < 1 {
			return js.Null()
		}
		p := args[0]
		err := engine.SetLimiter(webdemo.LimiterParams{
			Enabled:   p.Get("enabled").Bool(),
			Threshold: p.Get("threshold").Float(),
			Release:   p.Get("release").Float(),
		})
		if err != nil {
			return err.Error()
		}
		return js.Null()
	}))

	api.Set("setSpectrum", export(func(args []js.Value) any {
		if engine == nil || len(args) < 1 {
			return js.Null()
		}
		p := args[0]
		err := engine.SetSpectrum(webdemo.SpectrumParams{
			FFTSize:   p.Get("fftSize").Int(),
			Overlap:   p.Get("overlap").Float(),
			Window:    p.Get("window").String(),
			Smoothing: p.Get("smoothing").Float(),
		})
		if err != nil {
			return err.Error()
		}
		return js.Null()
	}))

	api.Set("render", export(func(args []js.Value) any {
		if engine == nil || len(args) < 1 {
			return js.Global().Get("Float32Array").New(0)
		}
		n := args[0].Int()
		buf := make([]float32, n)
		engine.Render(buf)
		arr := js.Global().Get("Float32Array").New(n)
		for i := 0; i < n; i++ {
			arr.SetIndex(i, buf[i])
		}
		return arr
	}))

	api.Set("responseCurve", export(func(args []js.Value) any {
		if engine == nil || len(args) < 1 {
			return js.Global().Get("Float32Array").New(0)
		}
		input := args[0]
		freqs := make([]float64, input.Length())
		for i := 0; i < input.Length(); i++ {
			freqs[i] = input.Index(i).Float()
		}
		resp := engine.ResponseCurveDB(freqs)
		arr := js.Global().Get("Float32Array").New(len(resp))
		for i := range resp {
			arr.SetIndex(i, resp[i])
		}
		return arr
	}))

	api.Set("nodeResponseCurve", export(func(args []js.Value) any {
		if engine == nil || len(args) < 2 {
			return js.Global().Get("Float32Array").New(0)
		}
		node := args[0].String()
		input := args[1]
		freqs := make([]float64, input.Length())
		for i := 0; i < input.Length(); i++ {
			freqs[i] = input.Index(i).Float()
		}
		resp := engine.NodeResponseCurveDB(node, freqs)
		arr := js.Global().Get("Float32Array").New(len(resp))
		for i := range resp {
			arr.SetIndex(i, resp[i])
		}
		return arr
	}))

	api.Set("spectrumCurve", export(func(args []js.Value) any {
		if engine == nil || len(args) < 1 {
			return js.Global().Get("Float32Array").New(0)
		}
		input := args[0]
		freqs := make([]float64, input.Length())
		for i := 0; i < input.Length(); i++ {
			freqs[i] = input.Index(i).Float()
		}
		resp := engine.SpectrumCurveDB(freqs)
		arr := js.Global().Get("Float32Array").New(len(resp))
		for i := range resp {
			arr.SetIndex(i, resp[i])
		}
		return arr
	}))

	api.Set("compressorCurve", export(func(args []js.Value) any {
		if engine == nil || len(args) < 1 {
			return js.Global().Get("Float32Array").New(0)
		}
		input := args[0]
		dbs := make([]float64, input.Length())
		for i := 0; i < input.Length(); i++ {
			dbs[i] = input.Index(i).Float()
		}
		resp := engine.CompressorCurveDB(dbs)
		arr := js.Global().Get("Float32Array").New(len(resp))
		for i := range resp {
			arr.SetIndex(i, resp[i])
		}
		return arr
	}))

	api.Set("limiterCurve", export(func(args []js.Value) any {
		if engine == nil || len(args) < 1 {
			return js.Global().Get("Float32Array").New(0)
		}
		input := args[0]
		dbs := make([]float64, input.Length())
		for i := 0; i < input.Length(); i++ {
			dbs[i] = input.Index(i).Float()
		}
		resp := engine.LimiterCurveDB(dbs)
		arr := js.Global().Get("Float32Array").New(len(resp))
		for i := range resp {
			arr.SetIndex(i, resp[i])
		}
		return arr
	}))

	api.Set("currentStep", export(func(args []js.Value) any {
		if engine == nil {
			return -1
		}
		return engine.CurrentStep()
	}))

	js.Global().Set("AlgoDSPDemo", api)
	select {}
}

func export(fn func([]js.Value) any) js.Func {
	f := js.FuncOf(func(_ js.Value, args []js.Value) any {
		return fn(args)
	})
	funcs = append(funcs, f)
	return f
}
