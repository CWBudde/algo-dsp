//go:build js && wasm

package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"syscall/js"

	"github.com/cwbudde/algo-dsp/internal/webdemo"
)

var (
	engine *webdemo.Engine
	funcs  []js.Func

	audioBridge renderBridge
)

// renderBridge carries one block of audio from Go to JS.
//
// The obvious implementation -- allocate a Float32Array and fill it with
// SetIndex -- costs one syscall/js crossing per sample. At 48 kHz with a
// 1024-frame buffer that is roughly 48,000 crossings and 200 kB of garbage per
// second, on the same thread that renders the UI.
//
// Instead the buffers are allocated once per block size and reused: Go writes
// the block into a []byte and hands it over with a single js.CopyBytesToJS, and
// JS reads it through a Float32Array view of the same ArrayBuffer. One crossing
// per callback, no allocation in steady state.
//
// The returned Float32Array is reused across calls, so the caller must consume
// it before the next render. The audio callback does (`out.set(chunk)`).
type renderBridge struct {
	samples []float32
	raw     []byte

	// u8 and f32 are views onto the same ArrayBuffer: u8 is the destination
	// js.CopyBytesToJS requires, f32 is what JS actually reads.
	u8  js.Value
	f32 js.Value
}

// resize reallocates the buffers for a block of n samples.
func (b *renderBridge) resize(n int) {
	b.samples = make([]float32, n)
	b.raw = make([]byte, n*bytesPerFloat32)

	buffer := js.Global().Get("ArrayBuffer").New(n * bytesPerFloat32)
	b.u8 = js.Global().Get("Uint8Array").New(buffer)
	b.f32 = js.Global().Get("Float32Array").New(buffer)
}

// render fills and returns a Float32Array of n samples.
func (b *renderBridge) render(n int) js.Value {
	if len(b.samples) != n {
		b.resize(n)
	}

	engine.Render(b.samples)

	// WASM is little-endian, which is also how a Float32Array reads its
	// backing buffer, so this layout matches what JS will see.
	for i, v := range b.samples {
		binary.LittleEndian.PutUint32(b.raw[i*bytesPerFloat32:], math.Float32bits(v))
	}

	js.CopyBytesToJS(b.u8, b.raw)

	return b.f32
}

const bytesPerFloat32 = 4

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
			RotarySpeakerEnabled:   p.Get("rotarySpeakerEnabled").Bool(),
			RotaryMix:              p.Get("rotaryMix").Float(),
			RotaryDrive:            p.Get("rotaryDrive").Float(),
			RotaryStereoWidth:      p.Get("rotaryStereoWidth").Float(),
			RotaryCrossoverHz:      p.Get("rotaryCrossoverHz").Float(),
			RotarySpeedFast:        p.Get("rotarySpeedFast").Bool(),
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

	api.Set("render", exportWithFallback(func(args []js.Value) any {
		if engine == nil || len(args) < 1 {
			return emptyFloat32Array()
		}

		n := args[0].Int()
		if n <= 0 {
			return emptyFloat32Array()
		}

		return audioBridge.render(n)
	}, emptyFloat32Array))

	// The curve getters share one shape: take a Float32Array of x values,
	// return a Float32Array of dB values.
	setCurve(api, "responseCurve", func(_ js.Value, xs []float64) []float64 {
		return engine.ResponseCurveDB(xs)
	})
	setCurve(api, "spectrumCurve", func(_ js.Value, xs []float64) []float64 {
		return engine.SpectrumCurveDB(xs)
	})
	setCurve(api, "compressorCurve", func(_ js.Value, xs []float64) []float64 {
		return engine.CompressorCurveDB(xs)
	})
	setCurve(api, "limiterCurve", func(_ js.Value, xs []float64) []float64 {
		return engine.LimiterCurveDB(xs)
	})
	setCurve(api, "nodeResponseCurve", func(node js.Value, xs []float64) []float64 {
		return engine.NodeResponseCurveDB(node.String(), xs)
	})

	api.Set("currentStep", export(func(args []js.Value) any {
		if engine == nil {
			return -1
		}
		return engine.CurrentStep()
	}))

	api.Set("getIRNames", export(func(args []js.Value) any {
		if engine == nil {
			return js.Global().Get("Array").New(0)
		}
		names := engine.GetIRNames()
		arr := js.Global().Get("Array").New(len(names))
		for i, name := range names {
			arr.SetIndex(i, name)
		}
		return arr
	}))

	js.Global().Set("AlgoDSPDemo", api)
	select {}
}

// export wraps fn so that a panic becomes an error string instead of killing
// the WASM instance.
//
// The js.Value accessors (Float, Int, Bool, String) panic on a type mismatch,
// and this file calls them on properties fetched by name. An unrecovered panic
// aborts the Go runtime: audio stops, AlgoDSPDemo becomes unusable, and only a
// page reload recovers -- which, because the demo restores its parameters from
// localStorage, replays the same bad value. Recovering here turns a malformed
// parameter into an ordinary error return that the caller already handles.
func export(fn func([]js.Value) any) js.Func {
	return exportWithFallback(fn, nil)
}

// exportWithFallback is export with a caller-supplied result for the panic
// path. Use it where the JS side expects a typed value rather than a string.
func exportWithFallback(fn func([]js.Value) any, onPanic func() any) js.Func {
	f := js.FuncOf(func(_ js.Value, args []js.Value) (result any) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			msg := fmt.Sprintf("algo-dsp: recovered from %v", r)
			js.Global().Get("console").Call("error", msg)
			if onPanic != nil {
				result = onPanic()
				return
			}
			result = msg
		}()

		return fn(args)
	})
	funcs = append(funcs, f)

	return f
}

// emptyFloat32Array is the panic fallback for the typed-array getters, which
// the drawing and audio code consume as Float32Array.
func emptyFloat32Array() any {
	return js.Global().Get("Float32Array").New(0)
}

// setCurve registers one of the dB-curve getters.
//
// They all share a shape: the last argument is an array of x values and the
// result is an array of dB values. nodeResponseCurve additionally takes a node
// name first, which it reads from head; the others ignore head.
func setCurve(api js.Value, name string, fn func(head js.Value, xs []float64) []float64) {
	api.Set(name, exportWithFallback(func(args []js.Value) any {
		if engine == nil || len(args) == 0 {
			return emptyFloat32Array()
		}

		xs := readFloat64Slice(args[len(args)-1])
		if len(xs) == 0 {
			return emptyFloat32Array()
		}

		return newFloat32Array(fn(args[0], xs))
	}, emptyFloat32Array))
}

// readFloat64Slice copies a JS array-like of numbers into a Go slice.
func readFloat64Slice(v js.Value) []float64 {
	n := v.Length()
	out := make([]float64, n)

	for i := range out {
		out[i] = v.Index(i).Float()
	}

	return out
}

// newFloat32Array copies a Go slice into a fresh JS Float32Array.
func newFloat32Array[T float32 | float64](vals []T) js.Value {
	arr := js.Global().Get("Float32Array").New(len(vals))
	for i, v := range vals {
		arr.SetIndex(i, float64(v))
	}

	return arr
}
