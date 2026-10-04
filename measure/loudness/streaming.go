package loudness

import (
	"fmt"
	"math"
	"slices"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// StreamingSnapshot reports BS.1770 K-weighted loudness in LUFS and LRA in LU.
// Availability flags distinguish incomplete windows and below-gate silence.
// LRAStable requires at least sixty seconds; shorter measurements are provisional.
type StreamingSnapshot struct {
	Momentary, ShortTerm, Integrated, LRA, MaxMomentary, MaxShortTerm float64
	Frames                                                            int64
	HasMomentary, HasShortTerm, HasIntegrated, HasLRA, LRAStable      bool
}

// StreamingMeter measures complete 400 ms and 3 s windows every 10 ms. Integrated
// gating uses 400 ms windows at 100 ms hops; LRA samples 3 s windows at 100 ms hops
// with -70 LUFS / -20 LU gates and the nearest-rank 10th/95th percentiles in
// EBU Tech 3342. Window endpoints use rounded absolute time and do not drift.
// All history and percentile workspace is reserved at construction. Processing,
// Snapshot and Reset allocate nothing; Snapshot scans and sorts bounded history
// and should be called at display cadence, outside an audio callback.
type StreamingMeter struct {
	rate                              float64
	maxFrames, frames, tick, nextTick int64
	weights                           []float64
	shelves, highpasses               []biquad.Section
	hops                              [300]float64
	hopFrames                         [300]int64
	write                             int
	hopSum                            float64
	hopStart                          int64
	integrated, short, scratch        []float64
	reading                           StreamingSnapshot
	failure                           error
}

// NewStreamingMeter validates packed channel weights and bounded frame capacity.
// Config has the same rate, channel and layout contract as IntegratedAnalyzer.
// Its total owned workspace is limited to 64 MiB; no layout is inferred.
func NewStreamingMeter(config IntegratedConfig) (*StreamingMeter, error) {
	_, _, capacity, err := validateIntegratedConfig(config)
	if err != nil {
		return nil, fmt.Errorf("loudness.streaming.new: %w", err)
	}

	capacity += 32
	if int64(capacity) > (MaxIntegratedWorkspaceBytes-int64(config.Channels)*256-8192)/24 {
		return nil, fmt.Errorf("loudness.streaming.new: workspace: %w", ErrLimit)
	}

	m := &StreamingMeter{rate: config.SampleRate, maxFrames: config.MaxFrames, weights: make([]float64, config.Channels), shelves: make([]biquad.Section, config.Channels), highpasses: make([]biquad.Section, config.Channels), integrated: make([]float64, 0, capacity), short: make([]float64, 0, capacity), scratch: make([]float64, capacity)}
	shelf, highpass := integratedKWeighting(config.SampleRate)

	for ch := range m.weights {
		m.weights[ch] = 1
		if config.ChannelWeights != nil {
			m.weights[ch] = config.ChannelWeights[ch]
		}

		m.shelves[ch].Coefficients, m.highpasses[ch].Coefficients = shelf, highpass
	}

	m.Reset()

	return m, nil
}

// Reset starts a fresh measurement while retaining all reserved storage.
func (m *StreamingMeter) Reset() {
	for ch := range m.weights {
		m.shelves[ch].Reset()
		m.highpasses[ch].Reset()
	}

	clear(m.hops[:])
	clear(m.hopFrames[:])
	m.frames, m.tick, m.hopStart = 0, 0, 0
	m.nextTick = int64(math.Round(m.rate / 100))
	m.write = 0
	m.hopSum = 0
	m.integrated = m.integrated[:0]
	m.short = m.short[:0]
	m.failure = nil
	m.reading = StreamingSnapshot{Momentary: math.Inf(-1), ShortTerm: math.Inf(-1), Integrated: math.Inf(-1), MaxMomentary: math.Inf(-1), MaxShortTerm: math.Inf(-1)}
}

// ProcessPlanar consumes finite, equal-length packed float64 channels atomically
// with respect to validation. Magnitudes above1e100 are rejected before mutation
// to bound filter and accumulated-energy arithmetic. All float32 values fit.
func (m *StreamingMeter) ProcessPlanar(block [][]float64) error {
	return processStreaming(m, block, nil, false)
}

// ProcessPlanar32 consumes float32 input without a widening copy.
func (m *StreamingMeter) ProcessPlanar32(block [][]float32) error {
	return processStreaming(m, block, nil, false)
}

// ProcessInterleaved32 consumes complete packed float32 frames without allocation.
func (m *StreamingMeter) ProcessInterleaved32(block []float32) error {
	return processStreaming(m, nil, block, true)
}

func processStreaming[T ~float32 | ~float64](m *StreamingMeter, planar [][]T, interleaved []T, packed bool) error {
	if m == nil || m.rate == 0 {
		return fmt.Errorf("loudness.streaming.process: %w", ErrState)
	}

	if m.failure != nil {
		return m.failure
	}

	channels := len(m.weights)
	frames := 0

	if packed {
		if len(interleaved)%channels != 0 {
			return fmt.Errorf("loudness.streaming.process: shape: %w", ErrInvalid)
		}

		frames = len(interleaved) / channels
	} else {
		if len(planar) != channels {
			return fmt.Errorf("loudness.streaming.process: channels: %w", ErrInvalid)
		}

		frames = len(planar[0])
		for _, values := range planar {
			if len(values) != frames {
				return fmt.Errorf("loudness.streaming.process: shape: %w", ErrInvalid)
			}
		}
	}

	if frames > MaxIntegratedBlockFrames || int64(frames) > m.maxFrames-m.frames {
		return fmt.Errorf("loudness.streaming.process: frames: %w", ErrLimit)
	}

	for frame := 0; frame < frames; frame++ {
		for ch := 0; ch < channels; ch++ {
			var value T
			if packed {
				value = interleaved[frame*channels+ch]
			} else {
				value = planar[ch][frame]
			}

			if !integratedFinite(float64(value)) {
				return fmt.Errorf("loudness.streaming.process: %w", ErrNonFinite)
			}

			if math.Abs(float64(value)) > 1e100 {
				return fmt.Errorf("loudness.streaming.process: amplitude: %w", ErrNumericalOverflow)
			}
		}
	}

	for frame := 0; frame < frames; frame++ {
		energy := 0.0

		for ch := range m.weights {
			if m.weights[ch] == 0 {
				continue
			}

			var value T
			if packed {
				value = interleaved[frame*channels+ch]
			} else {
				value = planar[ch][frame]
			}

			x := m.shelves[ch].ProcessSample(float64(value))
			x = m.highpasses[ch].ProcessSample(x)
			energy += m.weights[ch] * x * x
		}

		m.hopSum += energy

		m.frames++
		if !integratedFinite(m.hopSum) {
			m.failure = fmt.Errorf("loudness.streaming.process: %w", ErrNumericalOverflow)
			return m.failure
		}

		if m.frames == m.nextTick {
			m.recordTick()
		}
	}

	return nil
}

func (m *StreamingMeter) recordTick() {
	m.hops[m.write] = m.hopSum
	m.hopFrames[m.write] = m.frames - m.hopStart
	m.write = (m.write + 1) % len(m.hops)
	m.tick++
	m.hopSum = 0
	m.hopStart = m.frames

	m.nextTick = int64(math.Round(m.rate * float64(m.tick+1) / 100))
	if m.tick >= 40 {
		energy := m.windowPower(40)
		m.reading.Momentary = -.691 + 10*math.Log10(energy)
		m.reading.HasMomentary = true

		m.reading.MaxMomentary = math.Max(m.reading.MaxMomentary, m.reading.Momentary)
		if m.tick%10 == 0 {
			m.integrated = append(m.integrated, energy)
		}
	}

	if m.tick >= 300 {
		energy := m.windowPower(300)
		m.reading.ShortTerm = -.691 + 10*math.Log10(energy)
		m.reading.HasShortTerm = true

		m.reading.MaxShortTerm = math.Max(m.reading.MaxShortTerm, m.reading.ShortTerm)
		if m.tick%10 == 0 {
			m.short = append(m.short, energy)
		}
	}
}

func (m *StreamingMeter) windowPower(ticks int) float64 {
	sum := 0.0
	frames := int64(0)

	for i := 0; i < ticks; i++ {
		at := (m.write - 1 - i + len(m.hops)) % len(m.hops)
		sum += m.hops[at]
		frames += m.hopFrames[at]
	}

	return sum / float64(frames)
}

func streamingGatedMean(values []float64, relative float64) (float64, bool) {
	absPower := math.Pow(10, (-70+.691)/10)
	sum := 0.0
	count := 0

	for _, value := range values {
		if value >= absPower {
			sum += value
			count++
		}
	}

	if count == 0 {
		return 0, false
	}

	cutoff := math.Max(absPower, sum/float64(count)*math.Pow(10, relative/10))
	sum = 0
	count = 0

	for _, value := range values {
		if value >= cutoff {
			sum += value
			count++
		}
	}

	return sum / float64(count), count > 0
}

// Snapshot returns current readings without changing filter clocks or input.
// Maxima use 10 ms complete windows, while I/LRA gates use 100 ms hops.
func (m *StreamingMeter) Snapshot() StreamingSnapshot {
	r := m.reading

	r.Frames = m.frames
	if power, ok := streamingGatedMean(m.integrated, -10); ok {
		r.Integrated = -.691 + 10*math.Log10(power)
		r.HasIntegrated = true
	}

	absPower := math.Pow(10, (-70+.691)/10)
	sum := 0.0
	count := 0

	for _, power := range m.short {
		if power >= absPower {
			sum += power
			count++
		}
	}

	if count > 0 {
		cutoff := math.Max(absPower, sum/float64(count)*.01)
		n := 0

		for _, power := range m.short {
			if power >= cutoff {
				m.scratch[n] = power
				n++
			}
		}

		if n > 0 {
			values := m.scratch[:n]
			slices.Sort(values)

			lo := int(math.Round(float64(n-1) * .1))
			hi := int(math.Round(float64(n-1) * .95))
			r.LRA = 10 * math.Log10(values[hi]/values[lo])
			r.HasLRA = true
		}
	}

	r.LRAStable = r.HasLRA && float64(m.frames) >= 60*m.rate
	m.reading = r

	return r
}

// Reading returns the latest 10 ms M/S readings and cached I/LRA without
// scanning history. I/LRA are refreshed only by Snapshot; this permits cheap
// display polling while recalculating gated programme readings at 1 Hz.
func (m *StreamingMeter) Reading() StreamingSnapshot {
	r := m.reading
	r.Frames = m.frames
	r.LRAStable = r.HasLRA && float64(m.frames) >= 60*m.rate

	return r
}
