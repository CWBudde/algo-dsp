package effectchain

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/core"
	"github.com/cwbudde/algo-dsp/dsp/effects"
	"github.com/cwbudde/algo-dsp/dsp/effects/reverb"
)

// convReverbRuntime handles the "reverb-conv" node type using partitioned convolution.
type convReverbRuntime struct {
	fx         *reverb.ConvolutionReverb
	irIndex    int
	irProvider IRProvider
}

func (r *convReverbRuntime) Configure(_ Context, p Params) error {
	irIndex := int(p.GetNum("irIndex", 0))
	wet := p.GetNum("wet", 0.35)

	if r.fx == nil || r.irIndex != irIndex {
		if r.irProvider == nil {
			return nil
		}

		samples, _, ok := r.irProvider.GetIR(irIndex)
		if !ok || len(samples) == 0 {
			return nil
		}

		ch0 := samples[0]
		kernel := make([]float64, len(ch0))
		copy(kernel, ch0)

		if len(samples) > 1 {
			ch1 := samples[1]

			n := min(len(ch0), len(ch1))
			for i := range n {
				kernel[i] = (ch0[i] + ch1[i]) * 0.5
			}
		}

		cr, err := reverb.NewConvolutionReverb(kernel, 7)
		if err != nil {
			return fmt.Errorf("effectchain: create convolution reverb: %w", err)
		}

		r.fx = cr
		r.irIndex = irIndex
	}

	if r.fx != nil {
		r.fx.SetWetDry(wet, 1.0)
	}

	return nil
}

func (r *convReverbRuntime) Process(block []float64) {
	if r.fx == nil {
		return
	}

	_ = r.fx.ProcessInPlace(block)
}

type vocoderRuntime struct {
	fx         *effects.Vocoder
	carrierBuf []float64
}

func (r *vocoderRuntime) Configure(ctx Context, p Params) error {
	err := r.fx.SetSampleRate(ctx.SampleRate)
	if err != nil {
		return fmt.Errorf("effectchain: set vocoder sample rate: %w", err)
	}

	err = r.fx.SetAttack(core.Clamp(p.GetNum("attackMs", 0.5), 0.01, 100))
	if err != nil {
		return fmt.Errorf("effectchain: set vocoder attack: %w", err)
	}

	err = r.fx.SetRelease(core.Clamp(p.GetNum("releaseMs", 2.0), 0.01, 1000))
	if err != nil {
		return fmt.Errorf("effectchain: set vocoder release: %w", err)
	}

	err = r.fx.SetInputLevel(core.Clamp(p.GetNum("inputLevel", 0), 0, 10))
	if err != nil {
		return fmt.Errorf("effectchain: set vocoder input level: %w", err)
	}

	err = r.fx.SetSynthLevel(core.Clamp(p.GetNum("synthLevel", 0), 0, 10))
	if err != nil {
		return fmt.Errorf("effectchain: set vocoder synth level: %w", err)
	}

	err = r.fx.SetVocoderLevel(core.Clamp(p.GetNum("vocoderLevel", 1), 0, 10))
	if err != nil {
		return fmt.Errorf("effectchain: set vocoder level: %w", err)
	}

	return nil
}

func (r *vocoderRuntime) Process(block []float64) {
	if len(r.carrierBuf) < len(block) {
		r.carrierBuf = make([]float64, len(block))
	}

	copy(r.carrierBuf, block)
	_ = r.fx.ProcessBlock(block, r.carrierBuf, block)
}

// ProcessVocoder processes with separate modulator and carrier signals.
func (r *vocoderRuntime) ProcessVocoder(modulator, carrier []float64) {
	_ = r.fx.ProcessBlock(modulator, carrier, modulator)
}

func (r *vocoderRuntime) ProcessWithSidechain(main, sidechain []float64) {
	_ = r.fx.ProcessBlock(main, sidechain, main)
}
