package effects

import (
	"fmt"
	"math"
)

const (
	defaultBitCrusherBitDepth   = 8.0
	defaultBitCrusherDownsample = 1
	defaultBitCrusherMix        = 1.0
	minBitCrusherBitDepth       = 1.0
	maxBitCrusherBitDepth       = 32.0
	maxBitCrusherDownsample     = 256
)

// BitCrusherOption mutates bit crusher construction parameters.
type BitCrusherOption func(*bitCrusherConfig) error

type bitCrusherConfig struct {
	bitDepth   float64
	downsample int
	mix        float64
}

func defaultBitCrusherConfig() bitCrusherConfig {
	return bitCrusherConfig{
		bitDepth:   defaultBitCrusherBitDepth,
		downsample: defaultBitCrusherDownsample,
		mix:        defaultBitCrusherMix,
	}
}

// WithBitCrusherBitDepth sets the target bit depth for quantization.
// Fractional values are supported for smooth parameter sweeps.
// Range: [1, 32].
func WithBitCrusherBitDepth(bitDepth float64) BitCrusherOption {
	return func(cfg *bitCrusherConfig) error {
		if bitDepth < minBitCrusherBitDepth || bitDepth > maxBitCrusherBitDepth ||
			math.IsNaN(bitDepth) || math.IsInf(bitDepth, 0) {
			return fmt.Errorf("bit crusher bit depth must be in [%g, %g]: %f",
				minBitCrusherBitDepth, maxBitCrusherBitDepth, bitDepth)
		}
		cfg.bitDepth = bitDepth
		return nil
	}
}

// WithBitCrusherDownsample sets the sample rate reduction factor.
// A value of 1 means no downsampling; 4 means every 4th sample is held.
// Range: [1, 256].
func WithBitCrusherDownsample(factor int) BitCrusherOption {
	return func(cfg *bitCrusherConfig) error {
		if factor < 1 || factor > maxBitCrusherDownsample {
			return fmt.Errorf("bit crusher downsample factor must be in [1, %d]: %d",
				maxBitCrusherDownsample, factor)
		}
		cfg.downsample = factor
		return nil
	}
}

// WithBitCrusherMix sets the dry/wet mix in [0, 1].
func WithBitCrusherMix(mix float64) BitCrusherOption {
	return func(cfg *bitCrusherConfig) error {
		if mix < 0 || mix > 1 || math.IsNaN(mix) || math.IsInf(mix, 0) {
			return fmt.Errorf("bit crusher mix must be in [0, 1]: %f", mix)
		}
		cfg.mix = mix
		return nil
	}
}

// BitCrusher reduces bit depth and/or effective sample rate for lo-fi
// aesthetics. It combines two independent degradation mechanisms:
//
//   - Quantization: reduces amplitude resolution by snapping samples to a
//     grid determined by [BitDepth]. The input is assumed to be in [-1, 1].
//     Values outside this range are quantized but not clipped.
//
//   - Downsampling: holds each sample for [Downsample] consecutive output
//     samples, simulating a lower effective sample rate (sample-and-hold).
//
// Both effects can be used independently or combined. With BitDepth=32 and
// Downsample=1, the effect is transparent.
type BitCrusher struct {
	sampleRate float64
	bitDepth   float64
	downsample int
	mix        float64

	// Precomputed quantization parameters.
	quantLevels float64

	// Sample-and-hold state.
	holdCounter int
	holdValue   float64
}

// NewBitCrusher creates a bit crusher with the given sample rate and optional
// configuration overrides.
func NewBitCrusher(sampleRate float64, opts ...BitCrusherOption) (*BitCrusher, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("bit crusher sample rate must be > 0 and finite: %f", sampleRate)
	}

	cfg := defaultBitCrusherConfig()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	bc := &BitCrusher{
		sampleRate: sampleRate,
		bitDepth:   cfg.bitDepth,
		downsample: cfg.downsample,
		mix:        cfg.mix,
	}
	bc.updateQuantLevels()
	return bc, nil
}

// SetSampleRate updates the sample rate.
func (bc *BitCrusher) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("bit crusher sample rate must be > 0 and finite: %f", sampleRate)
	}
	bc.sampleRate = sampleRate
	return nil
}

// SetBitDepth sets the quantization bit depth in [1, 32].
func (bc *BitCrusher) SetBitDepth(bitDepth float64) error {
	if bitDepth < minBitCrusherBitDepth || bitDepth > maxBitCrusherBitDepth ||
		math.IsNaN(bitDepth) || math.IsInf(bitDepth, 0) {
		return fmt.Errorf("bit crusher bit depth must be in [%g, %g]: %f",
			minBitCrusherBitDepth, maxBitCrusherBitDepth, bitDepth)
	}
	bc.bitDepth = bitDepth
	bc.updateQuantLevels()
	return nil
}

// SetDownsample sets the downsample factor in [1, 256].
func (bc *BitCrusher) SetDownsample(factor int) error {
	if factor < 1 || factor > maxBitCrusherDownsample {
		return fmt.Errorf("bit crusher downsample factor must be in [1, %d]: %d",
			maxBitCrusherDownsample, factor)
	}
	bc.downsample = factor
	return nil
}

// SetMix sets the dry/wet mix in [0, 1].
func (bc *BitCrusher) SetMix(mix float64) error {
	if mix < 0 || mix > 1 || math.IsNaN(mix) || math.IsInf(mix, 0) {
		return fmt.Errorf("bit crusher mix must be in [0, 1]: %f", mix)
	}
	bc.mix = mix
	return nil
}

// Reset clears the sample-and-hold state.
func (bc *BitCrusher) Reset() {
	bc.holdCounter = 0
	bc.holdValue = 0
}

// ProcessSample processes one sample through the bit crusher.
func (bc *BitCrusher) ProcessSample(input float64) float64 {
	// Sample-and-hold: only update the held value every N samples.
	bc.holdCounter++
	if bc.holdCounter >= bc.downsample {
		bc.holdCounter = 0
		bc.holdValue = bc.quantize(input)
	}

	return input*(1-bc.mix) + bc.holdValue*bc.mix
}

// ProcessInPlace applies the bit crusher to buf in place.
func (bc *BitCrusher) ProcessInPlace(buf []float64) {
	for i := range buf {
		buf[i] = bc.ProcessSample(buf[i])
	}
}

// SampleRate returns the sample rate in Hz.
func (bc *BitCrusher) SampleRate() float64 { return bc.sampleRate }

// BitDepth returns the quantization bit depth.
func (bc *BitCrusher) BitDepth() float64 { return bc.bitDepth }

// Downsample returns the downsample factor.
func (bc *BitCrusher) Downsample() int { return bc.downsample }

// Mix returns the dry/wet mix in [0, 1].
func (bc *BitCrusher) Mix() float64 { return bc.mix }

func (bc *BitCrusher) updateQuantLevels() {
	bc.quantLevels = math.Exp2(bc.bitDepth - 1)
}

// quantize snaps a sample to the nearest quantization level.
// Input is assumed in [-1, 1] but values outside are quantized without clipping.
func (bc *BitCrusher) quantize(sample float64) float64 {
	return math.Round(sample*bc.quantLevels) / bc.quantLevels
}
