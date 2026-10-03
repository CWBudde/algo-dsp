package signal

import (
	"fmt"
	"math"
)

// StreamKind selects a continuous signal generator.
type StreamKind string

const (
	// StreamSilence generates exact positive zero.
	StreamSilence StreamKind = "silence"
	// StreamSine generates a zero-phase sine at StartHz.
	StreamSine StreamKind = "sine"
	// StreamWhiteNoise generates uniform noise in [-Amplitude, Amplitude).
	StreamWhiteNoise StreamKind = "white-noise"
	// StreamPinkNoise generates Voss-McCartney noise with five contribution bands.
	StreamPinkNoise StreamKind = "pink-noise"
	// StreamLinearSweep generates a sine with linearly varying frequency.
	StreamLinearSweep StreamKind = "linear-sweep"
	// StreamLogSweep generates a sine with exponentially varying frequency.
	StreamLogSweep StreamKind = "log-sweep"
)

// StreamConfig describes one finite mono stream. Use independent generators and
// seeds for independent noise channels. Frequencies are in (0, Nyquist]. Sweep
// duration is Frames/SampleRate; its end frequency is reached at that duration.
// Seed may be any uint64, including zero. Amplitude is a nonnegative linear
// amplitude, not dBFS. Streams do not clip.
type StreamConfig struct {
	Kind                                  StreamKind
	SampleRate, Amplitude, StartHz, EndHz float64
	Frames                                int64
	Seed                                  uint64
}

// StreamGenerator retains global sample position and noise state across blocks.
// GenerateInto32 is allocation-free. It is not safe for concurrent use.
type StreamGenerator struct {
	cfg                                 StreamConfig
	position                            int64
	random                              uint64
	pink                                [5]float64
	step, sweepRate, logK, logStartRate float64
}

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

// NewStreamGenerator validates a finite stream configuration. Existing one-shot
// Generator methods keep their original behavior and RNG; this API uses a
// specified SplitMix64 sequence so continuous noise is independent of blocks.
func NewStreamGenerator(cfg StreamConfig) (*StreamGenerator, error) {
	if !finite(cfg.SampleRate) || cfg.SampleRate <= 0 || !finite(cfg.Amplitude) ||
		cfg.Amplitude < 0 || cfg.Amplitude > math.MaxFloat32 || cfg.Frames <= 0 {
		return nil, fmt.Errorf("signal.stream: positive finite rate/frame count and representable nonnegative amplitude required")
	}

	frequency := func(hz float64) bool { return finite(hz) && hz > 0 && hz <= cfg.SampleRate/2 }

	switch cfg.Kind {
	case StreamSilence, StreamWhiteNoise, StreamPinkNoise:
	case StreamSine:
		if !frequency(cfg.StartHz) {
			return nil, fmt.Errorf("signal.stream: sine frequency must be in (0, Nyquist]")
		}
	case StreamLinearSweep, StreamLogSweep:
		if !frequency(cfg.StartHz) || !frequency(cfg.EndHz) {
			return nil, fmt.Errorf("signal.stream: sweep frequencies must be in (0, Nyquist]")
		}
	default:
		return nil, fmt.Errorf("signal.stream: unknown kind %q", cfg.Kind)
	}

	g := &StreamGenerator{cfg: cfg, random: cfg.Seed}

	g.step = 2 * math.Pi * (cfg.StartHz / cfg.SampleRate)
	if cfg.Kind == StreamLinearSweep {
		g.sweepRate = (cfg.EndHz/cfg.SampleRate - cfg.StartHz/cfg.SampleRate) / float64(cfg.Frames)
	}

	if cfg.Kind == StreamLogSweep {
		g.logK = (math.Log(cfg.EndHz) - math.Log(cfg.StartHz)) / float64(cfg.Frames)
		g.logStartRate = math.Log(cfg.StartHz) - math.Log(cfg.SampleRate)
	}

	return g, nil
}

// Position returns the number of samples already generated.
func (g *StreamGenerator) Position() int64 { return g.position }

// uniform returns one value in [0,1), using the top 53 bits of SplitMix64.
func (g *StreamGenerator) uniform() float64 {
	g.random += 0x9e3779b97f4a7c15
	z := g.random
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	z ^= z >> 31

	return float64(z>>11) * (1.0 / (1 << 53))
}

func (g *StreamGenerator) sample(position int64) float64 {
	n := float64(position)

	switch g.cfg.Kind {
	case StreamSine:
		return math.Sin(g.step * n)
	case StreamLinearSweep:
		return math.Sin(g.step*n + math.Pi*g.sweepRate*n*n)
	case StreamLogSweep:
		if g.logK == 0 {
			return math.Sin(g.step * n)
		}

		if g.logK*n > 500 {
			// Expm1 could overflow for a legal sweep spanning subnormal Hz to
			// Nyquist. Evaluate normalized frequency in log space instead.
			integral := (math.Exp(g.logStartRate+g.logK*n) - g.cfg.StartHz/g.cfg.SampleRate) / g.logK
			return math.Sin(2 * math.Pi * integral)
		}

		return math.Sin(g.step * math.Expm1(g.logK*n) / g.logK)
	case StreamWhiteNoise:
		return g.uniform()*2 - 1
	case StreamPinkNoise:
		thresholds := [5]float64{0.00198, 0.01478, 0.06378, 0.23378, 0.91578}
		weights := [5]float64{0.23980, 0.18727, 0.16380, 0.194685, 0.214463}

		u, value := g.uniform(), g.uniform()*2-1
		for band, threshold := range thresholds {
			if u <= threshold {
				g.pink[band] = value * weights[band]
				break
			}
		}

		sum := 0.0
		for _, contribution := range g.pink {
			sum += contribution
		}

		// Rounded published band weights sum slightly above one. Normalize
		// them so Amplitude remains an upper bound, including at MaxFloat32.
		return sum / (0.23980 + 0.18727 + 0.16380 + 0.194685 + 0.214463)
	default:
		return 0
	}
}

// GenerateInto32 writes the next len(dst) samples and advances Position. An empty
// destination is allowed. A block extending past Frames fails without modifying
// either destination or stream state. No call allocates after construction.
func (g *StreamGenerator) GenerateInto32(dst []float32) error {
	if int64(len(dst)) > g.cfg.Frames-g.position {
		return fmt.Errorf("signal.stream: block exceeds configured frame count")
	}

	for i := range dst {
		dst[i] = float32(g.cfg.Amplitude * g.sample(g.position+int64(i)))
	}

	g.position += int64(len(dst))

	return nil
}
