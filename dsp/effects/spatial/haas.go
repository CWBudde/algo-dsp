package spatial

import (
	"fmt"
	"math"
)

const (
	defaultHaasDelayMs = 15.0

	minHaasDelayMs = 0.0  // exclusive lower bound; a Haas delay must delay
	maxHaasDelayMs = 50.0 // beyond ~40 ms it is heard as a discrete echo, not a Haas shift
)

// HaasChannel selects which channel the Haas delay is applied to.
type HaasChannel int

const (
	// HaasChannelRight delays the right channel, shifting the perceived image
	// toward the left (the earlier, un-delayed channel).
	HaasChannelRight HaasChannel = iota
	// HaasChannelLeft delays the left channel, shifting the perceived image
	// toward the right.
	HaasChannelLeft
)

func (c HaasChannel) valid() bool {
	return c == HaasChannelRight || c == HaasChannelLeft
}

// HaasDelayOption mutates Haas delay construction parameters.
type HaasDelayOption func(*haasDelayConfig) error

type haasDelayConfig struct {
	delayMs float64
	channel HaasChannel
}

func defaultHaasDelayConfig() haasDelayConfig {
	return haasDelayConfig{
		delayMs: defaultHaasDelayMs,
		channel: HaasChannelRight,
	}
}

// WithHaasDelayMs sets the delay applied to the selected channel, in
// milliseconds. It must satisfy 0 < ms <= 50; the upper bound keeps the effect
// inside the Haas (precedence) zone rather than producing an audible echo.
func WithHaasDelayMs(ms float64) HaasDelayOption {
	return func(cfg *haasDelayConfig) error {
		if err := validateHaasDelayMs(ms); err != nil {
			return err
		}

		cfg.delayMs = ms

		return nil
	}
}

// WithHaasChannel selects which channel is delayed (default
// [HaasChannelRight]).
func WithHaasChannel(channel HaasChannel) HaasDelayOption {
	return func(cfg *haasDelayConfig) error {
		if !channel.valid() {
			return fmt.Errorf("haas delay: invalid channel: %d", channel)
		}

		cfg.channel = channel

		return nil
	}
}

func validateHaasDelayMs(ms float64) error {
	if ms <= minHaasDelayMs || ms > maxHaasDelayMs || math.IsNaN(ms) || math.IsInf(ms, 0) {
		return fmt.Errorf("haas delay must be in (%g, %g] ms: %f", minHaasDelayMs, maxHaasDelayMs, ms)
	}

	return nil
}

// HaasDelay is a short stereo (precedence) delay. It delays one channel by a
// few milliseconds relative to the other, shifting and widening the perceived
// stereo image without introducing an audible echo (the Haas effect).
type HaasDelay struct {
	sampleRate float64
	delayMs    float64
	channel    HaasChannel

	line monoDelay
}

// NewHaasDelay creates a Haas delay for the given sample rate with practical
// defaults and optional overrides.
func NewHaasDelay(sampleRate float64, opts ...HaasDelayOption) (*HaasDelay, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("haas delay sample rate must be > 0 and finite: %f", sampleRate)
	}

	cfg := defaultHaasDelayConfig()

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	h := &HaasDelay{
		sampleRate: sampleRate,
		delayMs:    cfg.delayMs,
		channel:    cfg.channel,
	}
	h.rebuildLine()

	return h, nil
}

// delaySamples converts the configured delay time to an integer sample count
// (at least one sample).
func (h *HaasDelay) delaySamples() int {
	return max(int(math.Round(h.delayMs/1000*h.sampleRate)), 1)
}

func (h *HaasDelay) rebuildLine() {
	// monoDelay realizes a delay of (arg+1) samples, so pass one less to land on
	// exactly delaySamples(). (For sub-2-sample requests the line clamps to its
	// 1-sample minimum.)
	h.line.init(h.delaySamples() - 1)
}

// ProcessStereo processes one stereo sample pair, delaying the selected
// channel and passing the other through unchanged.
func (h *HaasDelay) ProcessStereo(left, right float64) (float64, float64) {
	if h.channel == HaasChannelLeft {
		return h.line.tick(left), right
	}

	return left, h.line.tick(right)
}

// ProcessStereoInPlace applies the Haas delay to paired left/right buffers in
// place. Both buffers must have the same length.
func (h *HaasDelay) ProcessStereoInPlace(left, right []float64) error {
	if len(left) != len(right) {
		return fmt.Errorf("haas delay: left and right buffers must have equal length: %d != %d",
			len(left), len(right))
	}

	for i := range left {
		left[i], right[i] = h.ProcessStereo(left[i], right[i])
	}

	return nil
}

// ProcessInterleavedInPlace applies the Haas delay to an interleaved stereo
// buffer (L, R, L, R, ...) in place. The buffer length must be even.
func (h *HaasDelay) ProcessInterleavedInPlace(buf []float64) error {
	if len(buf)%2 != 0 {
		return fmt.Errorf("haas delay: interleaved buffer length must be even: %d", len(buf))
	}

	for i := 0; i < len(buf); i += 2 {
		buf[i], buf[i+1] = h.ProcessStereo(buf[i], buf[i+1])
	}

	return nil
}

// Reset clears the internal delay-line state.
func (h *HaasDelay) Reset() {
	h.line.reset()
}

// SampleRate returns the sample rate in Hz.
func (h *HaasDelay) SampleRate() float64 { return h.sampleRate }

// DelayMs returns the configured delay time in milliseconds.
func (h *HaasDelay) DelayMs() float64 { return h.delayMs }

// Channel returns the channel that is currently delayed.
func (h *HaasDelay) Channel() HaasChannel { return h.channel }

// SetSampleRate updates the sample rate and rebuilds the delay line, clearing
// its state.
func (h *HaasDelay) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("haas delay sample rate must be > 0 and finite: %f", sampleRate)
	}

	h.sampleRate = sampleRate
	h.rebuildLine()

	return nil
}

// SetDelayMs updates the delay time and rebuilds the delay line, clearing its
// state.
func (h *HaasDelay) SetDelayMs(ms float64) error {
	if err := validateHaasDelayMs(ms); err != nil {
		return err
	}

	h.delayMs = ms
	h.rebuildLine()

	return nil
}

// SetChannel selects which channel is delayed. The delay-line state is cleared
// so the newly delayed channel starts from silence.
func (h *HaasDelay) SetChannel(channel HaasChannel) error {
	if !channel.valid() {
		return fmt.Errorf("haas delay: invalid channel: %d", channel)
	}

	h.channel = channel
	h.line.reset()

	return nil
}
