package features

import (
	"errors"
	"fmt"
	"math"
)

// Sentinel errors returned by this package. Errors are wrapped with context,
// so test for them with errors.Is.
var (
	// ErrInvalidConfig reports an invalid [Config] or [Timing].
	ErrInvalidConfig = errors.New("features: invalid config")
	// ErrInvalidArgument reports an invalid scalar argument.
	ErrInvalidArgument = errors.New("features: invalid argument")
	// ErrNilOption reports a nil [Option].
	ErrNilOption = errors.New("features: nil option")
	// ErrNoAudio reports an empty channel list or empty channels.
	ErrNoAudio = errors.New("features: empty audio")
	// ErrChannelLength reports channels of unequal length.
	ErrChannelLength = errors.New("features: unequal channel lengths")
)

// Default analysis parameters, matching the AudioVisualizer analysis.
const (
	// DefaultSampleRate is the default analysis sample rate in Hz.
	DefaultSampleRate = 24000.0
	// DefaultFFTSize is the default FFT size in samples.
	DefaultFFTSize = 2048
	// DefaultHop is the default hop size in samples (10 ms at 24 kHz).
	DefaultHop = 240
	// DefaultGate is the default RMS gate below which centroid and flux
	// are forced to zero.
	DefaultGate = 1e-4
)

// DefaultBandEdges returns the default band edges in Hz: five bands
// 25–140, 140–400, 400–2000, 2000–6000 and 6000–12000 Hz.
func DefaultBandEdges() []float64 {
	return []float64{25, 140, 400, 2000, 6000, 12000}
}

// Timing describes the frame grid of a feature track: frame i is centred on
// sample i*Hop of a signal sampled at SampleRate.
type Timing struct {
	// SampleRate is the sample rate in Hz.
	SampleRate float64
	// Hop is the hop size in samples.
	Hop int
}

// Validate reports whether the timing is usable.
func (t Timing) Validate() error {
	if !(t.SampleRate > 0) || math.IsInf(t.SampleRate, 0) {
		return fmt.Errorf("%w: sample rate %v", ErrInvalidConfig, t.SampleRate)
	}

	if t.Hop < 1 {
		return fmt.Errorf("%w: hop %d", ErrInvalidConfig, t.Hop)
	}

	return nil
}

// FrameRate returns the number of frames per second, SampleRate/Hop.
func (t Timing) FrameRate() float64 {
	return t.SampleRate / float64(t.Hop)
}

// FrameTime returns the centre time of frame i in seconds,
// float64(i)*Hop/SampleRate.
func (t Timing) FrameTime(i int) float64 {
	return float64(i) * float64(t.Hop) / t.SampleRate
}

// FramePosition returns the fractional frame index of time sec,
// sec*SampleRate/Hop.
func (t Timing) FramePosition(sec float64) float64 {
	return sec * t.SampleRate / float64(t.Hop)
}

// Config holds the parameters of [Extract].
type Config struct {
	// SampleRate is the sample rate of the input in Hz.
	SampleRate float64
	// FFTSize is the FFT length in samples (>= 2).
	FFTSize int
	// Hop is the frame hop in samples (>= 1).
	Hop int
	// BandEdges are the strictly increasing, non-negative band edges in
	// Hz; band b covers edge[b] <= f < edge[b+1]. At least two edges are
	// required.
	BandEdges []float64
	// Gate is the RMS level below which Centroid and Flux are zeroed.
	Gate float64
}

// DefaultConfig returns the AudioVisualizer analysis defaults: 24 kHz,
// FFT size 2048, hop 240 (10 ms), the five [DefaultBandEdges] bands and an
// RMS gate of 1e-4.
func DefaultConfig() Config {
	return Config{
		SampleRate: DefaultSampleRate,
		FFTSize:    DefaultFFTSize,
		Hop:        DefaultHop,
		BandEdges:  DefaultBandEdges(),
		Gate:       DefaultGate,
	}
}

// Timing returns the frame grid of the configuration.
func (c Config) Timing() Timing {
	return Timing{SampleRate: c.SampleRate, Hop: c.Hop}
}

// Validate reports whether the configuration is usable.
func (c Config) Validate() error {
	err := c.Timing().Validate()
	if err != nil {
		return err
	}

	if c.FFTSize < 2 {
		return fmt.Errorf("%w: FFT size %d", ErrInvalidConfig, c.FFTSize)
	}

	if len(c.BandEdges) < 2 {
		return fmt.Errorf("%w: need at least two band edges, got %d", ErrInvalidConfig, len(c.BandEdges))
	}

	for i, e := range c.BandEdges {
		if !(e >= 0) || math.IsInf(e, 0) {
			return fmt.Errorf("%w: band edge %d is %v", ErrInvalidConfig, i, e)
		}

		if i > 0 && e <= c.BandEdges[i-1] {
			return fmt.Errorf("%w: band edges not strictly increasing at %d", ErrInvalidConfig, i)
		}
	}

	if !(c.Gate >= 0) || math.IsInf(c.Gate, 0) {
		return fmt.Errorf("%w: gate %v", ErrInvalidConfig, c.Gate)
	}

	return nil
}

// checkChannels validates a channel set and returns the common length.
func checkChannels(channels [][]float64) (int, error) {
	if len(channels) == 0 || len(channels[0]) == 0 {
		return 0, ErrNoAudio
	}

	n := len(channels[0])
	for i, ch := range channels {
		if len(ch) != n {
			return 0, fmt.Errorf("%w: channel %d has %d samples, channel 0 has %d",
				ErrChannelLength, i, len(ch), n)
		}
	}

	return n, nil
}
