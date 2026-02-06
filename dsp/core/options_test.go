package core

import "testing"

func TestApplyProcessorOptions(t *testing.T) {
	cfg := ApplyProcessorOptions(WithSampleRate(96000), WithBlockSize(2048))
	if cfg.SampleRate != 96000 {
		t.Fatalf("sample rate = %v, want 96000", cfg.SampleRate)
	}
	if cfg.BlockSize != 2048 {
		t.Fatalf("block size = %d, want 2048", cfg.BlockSize)
	}
}

func TestInvalidOptionsIgnored(t *testing.T) {
	cfg := ApplyProcessorOptions(WithSampleRate(0), WithBlockSize(-1))
	def := DefaultProcessorConfig()
	if cfg != def {
		t.Fatalf("cfg = %#v, want %#v", cfg, def)
	}
}
