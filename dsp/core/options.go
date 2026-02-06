package core

// ProcessorConfig defines common DSP processing settings.
type ProcessorConfig struct {
	SampleRate float64
	BlockSize  int
}

// ProcessorOption mutates a ProcessorConfig.
type ProcessorOption func(*ProcessorConfig)

// DefaultProcessorConfig returns sensible defaults for offline and streaming use.
func DefaultProcessorConfig() ProcessorConfig {
	return ProcessorConfig{
		SampleRate: 48000,
		BlockSize:  1024,
	}
}

// WithSampleRate sets the processing sample rate.
func WithSampleRate(sampleRate float64) ProcessorOption {
	return func(cfg *ProcessorConfig) {
		if sampleRate > 0 {
			cfg.SampleRate = sampleRate
		}
	}
}

// WithBlockSize sets the processing block size.
func WithBlockSize(blockSize int) ProcessorOption {
	return func(cfg *ProcessorConfig) {
		if blockSize > 0 {
			cfg.BlockSize = blockSize
		}
	}
}

// ApplyProcessorOptions applies zero or more options to the default config.
func ApplyProcessorOptions(opts ...ProcessorOption) ProcessorConfig {
	cfg := DefaultProcessorConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
}
