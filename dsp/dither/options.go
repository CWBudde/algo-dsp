package dither

import (
	"fmt"
	"math"
	"math/rand/v2"
)

const (
	defaultBitDepth        = 16
	defaultDitherType      = DitherTriangular
	defaultDitherAmplitude = 1.0
	defaultLimit           = true
	minBitDepth            = 1
	maxBitDepth            = 32
)

type config struct {
	bitDepth        int
	ditherType      DitherType
	ditherAmplitude float64
	limit           bool
	shaper          NoiseShaper
	rng             *rand.Rand
	sharpPreset     bool
	iirShelfFreq    float64 // >0 means use IIR shelf
}

func defaultConfig() config {
	return config{
		bitDepth:        defaultBitDepth,
		ditherType:      defaultDitherType,
		ditherAmplitude: defaultDitherAmplitude,
		limit:           defaultLimit,
	}
}

// Option configures a [Quantizer].
type Option func(*config) error

// WithBitDepth sets the target bit depth for quantization (1–32, default 16).
func WithBitDepth(bits int) Option {
	return func(cfg *config) error {
		if bits < minBitDepth || bits > maxBitDepth {
			return fmt.Errorf("dither: bit depth must be in [%d, %d]: %d", minBitDepth, maxBitDepth, bits)
		}

		cfg.bitDepth = bits

		return nil
	}
}

// WithDitherType sets the dither noise PDF (default [DitherTriangular]).
func WithDitherType(dt DitherType) Option {
	return func(cfg *config) error {
		if !dt.Valid() {
			return fmt.Errorf("dither: invalid dither type: %d", dt)
		}

		cfg.ditherType = dt

		return nil
	}
}

// WithDitherAmplitude sets the dither noise amplitude (default 1.0, must be >= 0).
func WithDitherAmplitude(amp float64) Option {
	return func(cfg *config) error {
		if amp < 0 || math.IsNaN(amp) || math.IsInf(amp, 0) {
			return fmt.Errorf("dither: amplitude must be >= 0 and finite: %f", amp)
		}

		cfg.ditherAmplitude = amp

		return nil
	}
}

// WithLimit enables or disables output limiting to the bit-depth range (default true).
func WithLimit(enabled bool) Option {
	return func(cfg *config) error {
		cfg.limit = enabled
		return nil
	}
}

// WithNoiseShaper sets a custom [NoiseShaper] implementation.
func WithNoiseShaper(ns NoiseShaper) Option {
	return func(cfg *config) error {
		cfg.shaper = ns
		return nil
	}
}

// WithFIRPreset creates an [FIRShaper] from a predefined coefficient [Preset].
func WithFIRPreset(p Preset) Option {
	return func(cfg *config) error {
		if !p.Valid() {
			return fmt.Errorf("dither: invalid preset: %d", p)
		}

		cfg.shaper = NewFIRShaper(p.Coefficients())

		return nil
	}
}

// WithSharpPreset enables sample-rate-adaptive sharp noise shaping.
// The coefficient set is selected automatically based on the Quantizer's sample rate.
func WithSharpPreset() Option {
	return func(cfg *config) error {
		cfg.sharpPreset = true
		return nil
	}
}

// WithIIRShelf creates an [IIRShelfShaper] with the given corner frequency.
// The sample rate is taken from the Quantizer constructor.
func WithIIRShelf(freq float64) Option {
	return func(cfg *config) error {
		if freq <= 0 || math.IsNaN(freq) || math.IsInf(freq, 0) {
			return fmt.Errorf("dither: IIR shelf frequency must be > 0 and finite: %f", freq)
		}

		cfg.iirShelfFreq = freq

		return nil
	}
}

// WithRNG sets a deterministic random number generator for reproducible output.
func WithRNG(rng *rand.Rand) Option {
	return func(cfg *config) error {
		cfg.rng = rng
		return nil
	}
}
