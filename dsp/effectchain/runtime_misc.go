package effectchain

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/core"
	"github.com/cwbudde/algo-dsp/dsp/effects"
	"github.com/cwbudde/algo-dsp/dsp/effects/reverb"
)

const convolutionMaxBlockOrder = 10

// convReverbRuntime handles the "reverb-conv" node type using partitioned convolution.
type convReverbRuntime struct {
	fx               *reverb.ConvolutionReverb
	irIndex          int
	sampleRate       float64
	irProvider       IRProvider
	channelIndex     int
	sourceChannelMap []int
}

func (r *convReverbRuntime) Configure(ctx Context, p Params) error {
	physicalChannel := r.channelIndex
	if len(r.sourceChannelMap) > 0 {
		if physicalChannel < 0 || physicalChannel >= len(r.sourceChannelMap) {
			return fmt.Errorf("effectchain: packed channel %d not covered by source map", physicalChannel)
		}

		physicalChannel = r.sourceChannelMap[physicalChannel]
	}

	irIndex := int(p.GetNum("irIndex", 0))
	wet := p.GetNum("wet", 0.35)

	if r.fx == nil || r.irIndex != irIndex || r.sampleRate != ctx.SampleRate {
		if r.irProvider == nil {
			return fmt.Errorf("effectchain: convolution requires an impulse response provider")
		}

		samples, sampleRate, ok := r.irProvider.GetIR(irIndex)
		if !ok || len(samples) == 0 || len(samples[0]) == 0 {
			return fmt.Errorf("effectchain: unavailable impulse response %d", irIndex)
		}

		if sampleRate != ctx.SampleRate {
			return fmt.Errorf("effectchain: impulse response sample rate %g differs from %g", sampleRate, ctx.SampleRate)
		}

		for _, channel := range samples {
			if len(channel) != len(samples[0]) {
				return fmt.Errorf("effectchain: unequal impulse response channels")
			}

			for _, sample := range channel {
				if math.IsNaN(sample) || math.IsInf(sample, 0) {
					return fmt.Errorf("effectchain: non-finite impulse response")
				}
			}
		}

		if len(samples) > 2 {
			return fmt.Errorf("effectchain: impulse response must be mono or stereo")
		}

		kernel := samples[physicalChannel%len(samples)]

		cr, err := reverb.NewConvolutionReverbWithMaxBlockOrder(kernel, 7, convolutionMaxBlockOrder)
		if err != nil {
			return fmt.Errorf("effectchain: create convolution reverb: %w", err)
		}

		r.fx = cr
		r.fx.SetLatencyAlignedDry(true)
		r.irIndex = irIndex
		r.sampleRate = ctx.SampleRate
	}

	if r.fx != nil {
		r.fx.SetWetDry(core.Clamp(wet, 0, 1), 1.0)
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
