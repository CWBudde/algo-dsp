package spatial

import (
	"math"
	"testing"
)

func TestNewHaasDelayValidation(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate float64
		opts       []HaasDelayOption
	}{
		{"zero sample rate", 0, nil},
		{"negative sample rate", -48000, nil},
		{"NaN sample rate", math.NaN(), nil},
		{"Inf sample rate", math.Inf(1), nil},
		{"zero delay", 48000, []HaasDelayOption{WithHaasDelayMs(0)}},
		{"negative delay", 48000, []HaasDelayOption{WithHaasDelayMs(-5)}},
		{"delay above max", 48000, []HaasDelayOption{WithHaasDelayMs(maxHaasDelayMs + 1)}},
		{"NaN delay", 48000, []HaasDelayOption{WithHaasDelayMs(math.NaN())}},
		{"invalid channel", 48000, []HaasDelayOption{WithHaasChannel(HaasChannel(99))}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewHaasDelay(tt.sampleRate, tt.opts...); err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestNewHaasDelayDefaults(t *testing.T) {
	h, err := NewHaasDelay(48000)
	if err != nil {
		t.Fatalf("NewHaasDelay: %v", err)
	}

	if h.DelayMs() != defaultHaasDelayMs {
		t.Errorf("DelayMs = %g, want %g", h.DelayMs(), defaultHaasDelayMs)
	}

	if h.Channel() != HaasChannelRight {
		t.Errorf("Channel = %d, want HaasChannelRight", h.Channel())
	}

	if h.SampleRate() != 48000 {
		t.Errorf("SampleRate = %g, want 48000", h.SampleRate())
	}
}

// delayInSamples mirrors the internal conversion so tests can assert the exact
// delay without depending on private helpers beyond the package, including the
// two-sample floor monoDelay imposes.
func delayInSamples(delayMs, sampleRate float64) int {
	return max(int(math.Round(delayMs/1000*sampleRate)), 2)
}

func TestHaasDelayDelaysSelectedChannel(t *testing.T) {
	const sampleRate = 48000.0

	for _, tc := range []struct {
		name    string
		channel HaasChannel
	}{
		{"right delayed", HaasChannelRight},
		{"left delayed", HaasChannelLeft},
	} {
		t.Run(tc.name, func(t *testing.T) {
			delayMs := 10.0
			want := delayInSamples(delayMs, sampleRate)

			h, err := NewHaasDelay(sampleRate, WithHaasDelayMs(delayMs), WithHaasChannel(tc.channel))
			if err != nil {
				t.Fatalf("NewHaasDelay: %v", err)
			}

			const n = 256

			left := make([]float64, n)
			right := make([]float64, n)
			left[0] = 1.0
			right[0] = 1.0

			if err := h.ProcessStereoInPlace(left, right); err != nil {
				t.Fatalf("ProcessStereoInPlace: %v", err)
			}

			delayed, passthrough := right, left
			if tc.channel == HaasChannelLeft {
				delayed, passthrough = left, right
			}

			// Passthrough channel: impulse stays at index 0.
			if passthrough[0] != 1.0 {
				t.Errorf("passthrough[0] = %g, want 1.0", passthrough[0])
			}

			// Delayed channel: impulse appears exactly `want` samples later, and
			// nowhere else.
			for i, v := range delayed {
				expected := 0.0
				if i == want {
					expected = 1.0
				}

				if v != expected {
					t.Errorf("delayed[%d] = %g, want %g", i, v, expected)
				}
			}
		})
	}
}

func TestHaasDelayReset(t *testing.T) {
	h, err := NewHaasDelay(48000, WithHaasDelayMs(5))
	if err != nil {
		t.Fatalf("NewHaasDelay: %v", err)
	}

	// Push an impulse into the line but read fewer samples than the delay, so
	// it is still "in flight".
	_, _ = h.ProcessStereo(0, 1.0)

	h.Reset()

	// After reset, the in-flight sample must be gone: a run of silence must
	// stay silent for the whole delay window.
	for i := 0; i < delayInSamples(5, 48000)+2; i++ {
		_, r := h.ProcessStereo(0, 0)
		if r != 0 {
			t.Fatalf("sample %d after reset = %g, want 0", i, r)
		}
	}
}

func TestHaasDelaySetters(t *testing.T) {
	h, err := NewHaasDelay(48000)
	if err != nil {
		t.Fatalf("NewHaasDelay: %v", err)
	}

	if err := h.SetDelayMs(20); err != nil {
		t.Fatalf("SetDelayMs: %v", err)
	}

	if h.DelayMs() != 20 {
		t.Errorf("DelayMs = %g, want 20", h.DelayMs())
	}

	if err := h.SetChannel(HaasChannelLeft); err != nil {
		t.Fatalf("SetChannel: %v", err)
	}

	if h.Channel() != HaasChannelLeft {
		t.Errorf("Channel = %d, want HaasChannelLeft", h.Channel())
	}

	if err := h.SetSampleRate(44100); err != nil {
		t.Fatalf("SetSampleRate: %v", err)
	}

	if h.SampleRate() != 44100 {
		t.Errorf("SampleRate = %g, want 44100", h.SampleRate())
	}

	// Invalid setter inputs must be rejected.
	if err := h.SetDelayMs(0); err == nil {
		t.Error("SetDelayMs(0) expected error")
	}

	if err := h.SetSampleRate(-1); err == nil {
		t.Error("SetSampleRate(-1) expected error")
	}

	if err := h.SetChannel(HaasChannel(42)); err == nil {
		t.Error("SetChannel(invalid) expected error")
	}
}

func TestHaasDelayInterleavedLengthValidation(t *testing.T) {
	h, err := NewHaasDelay(48000)
	if err != nil {
		t.Fatalf("NewHaasDelay: %v", err)
	}

	if err := h.ProcessInterleavedInPlace([]float64{1, 2, 3}); err == nil {
		t.Error("expected error for odd-length interleaved buffer")
	}

	if err := h.ProcessStereoInPlace([]float64{1, 2}, []float64{1}); err == nil {
		t.Error("expected error for mismatched buffer lengths")
	}
}
