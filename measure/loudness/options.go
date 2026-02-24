package loudness

import "github.com/cwbudde/algo-dsp/dsp/core"

// MeterConfig defines configuration for the loudness meter.
type MeterConfig struct {
	core.ProcessorConfig
	Channels int
}

// MeterOption mutates a MeterConfig.
type MeterOption func(*MeterConfig)

// DefaultMeterConfig returns sensible defaults.
func DefaultMeterConfig() MeterConfig {
	return MeterConfig{
		ProcessorConfig: core.DefaultProcessorConfig(),
		Channels:        2,
	}
}

// WithSampleRate sets the processing sample rate.
func WithSampleRate(sampleRate float64) MeterOption {
	return func(cfg *MeterConfig) {
		if sampleRate > 0 {
			cfg.SampleRate = sampleRate
		}
	}
}

// WithChannels sets the number of channels (1 for mono, 2 for stereo).
func WithChannels(channels int) MeterOption {
	return func(cfg *MeterConfig) {
		if channels > 0 {
			cfg.Channels = channels
		}
	}
}

// ApplyMeterOptions applies zero or more options to the default config.
func ApplyMeterOptions(opts ...MeterOption) MeterConfig {
	cfg := DefaultMeterConfig()

	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	return cfg
}
