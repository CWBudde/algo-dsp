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
		engine.SetTransport(args[0].Float(), args[1].Float())
		return js.Null()
	}))

	api.Set("setRunning", export(func(args []js.Value) any {
		if engine == nil || len(args) < 1 {
			return js.Null()
		}
		engine.SetRunning(args[0].Bool())
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
			LowFreq:  p.Get("lowFreq").Float(),
			LowGain:  p.Get("lowGain").Float(),
			MidFreq:  p.Get("midFreq").Float(),
			MidGain:  p.Get("midGain").Float(),
			HighFreq: p.Get("highFreq").Float(),
			HighGain: p.Get("highGain").Float(),
			MidQ:     p.Get("midQ").Float(),
			Master:   p.Get("master").Float(),
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
