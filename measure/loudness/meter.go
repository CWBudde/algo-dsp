package loudness

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
)

const (
	// K-weighting filter parameters from BS.1770.
	kWeightingShelfFreq = 1500.0
	kWeightingShelfGain = 4.0

	kWeightingHpfFreq = 38.0

	// Integration window durations in seconds.
	momentaryDuration = 0.4
	shortTermDuration = 3.0

	// Gating parameters.
	absThreshold    = -70.0
	relThreshold    = -10.0
	blockOverlap    = 0.75 // 75% overlap for integrated loudness gating
	blockStepFactor = 1.0 - blockOverlap
)

// Meter implements EBU R128 / ITU-R BS.1770 loudness metering.
type Meter struct {
	sampleRate float64
	channels   int

	// K-weighting filters per channel
	shelfFilters []*biquad.Section
	hpfFilters   []*biquad.Section

	// History for integration (sliding window)
	momWindowSamples   int
	shortWindowSamples int
	momHistory         [][]float64 // Squares of samples
	shortHistory       [][]float64 // Squares of samples
	momWriteIdx        int
	shortWriteIdx      int

	// Running sums for sliding windows
	momRunningSums   []float64
	shortRunningSums []float64

	// Integrated loudness state
	integrationRunning bool
	totalSamples       int64
	blockSamples       int
	blockSamplesStep   int
	samplesSinceStep   int

	// Gating blocks
	blocks []float64 // Linear power blocks (sum of channel powers)

	// Peak tracking
	truePeak []float64 // Simple peak for now, True Peak requires oversampling
}

// NewMeter creates a new loudness meter with the given options.
func NewMeter(opts ...MeterOption) *Meter {
	cfg := ApplyMeterOptions(opts...)

	meter := &Meter{
		sampleRate: cfg.SampleRate,
		channels:   cfg.Channels,
	}

	meter.reconfigure()

	return meter
}

func (m *Meter) reconfigure() {
	m.shelfFilters = make([]*biquad.Section, m.channels)
	m.hpfFilters = make([]*biquad.Section, m.channels)

	q := 1.0 / math.Sqrt(2)
	shelfCoeffs := design.HighShelf(kWeightingShelfFreq, kWeightingShelfGain, q, m.sampleRate)
	hpfCoeffs := design.Highpass(kWeightingHpfFreq, q, m.sampleRate)

	for i := range m.channels {
		m.shelfFilters[i] = biquad.NewSection(shelfCoeffs)
		m.hpfFilters[i] = biquad.NewSection(hpfCoeffs)
	}

	m.momWindowSamples = int(math.Round(momentaryDuration * m.sampleRate))
	m.shortWindowSamples = int(math.Round(shortTermDuration * m.sampleRate))

	m.momHistory = make([][]float64, m.channels)

	m.shortHistory = make([][]float64, m.channels)
	for i := range m.channels {
		m.momHistory[i] = make([]float64, m.momWindowSamples)
		m.shortHistory[i] = make([]float64, m.shortWindowSamples)
	}

	m.momRunningSums = make([]float64, m.channels)
	m.shortRunningSums = make([]float64, m.channels)
	m.truePeak = make([]float64, m.channels)

	m.blockSamples = m.momWindowSamples

	m.blockSamplesStep = max(int(math.Round(momentaryDuration*blockStepFactor*m.sampleRate)), 1)

	m.Reset()
}

// Reset clears all integration state and peak values.
func (m *Meter) Reset() {
	for i := range m.channels {
		m.shelfFilters[i].Reset()
		m.hpfFilters[i].Reset()

		for j := range m.momHistory[i] {
			m.momHistory[i][j] = 0
		}

		for j := range m.shortHistory[i] {
			m.shortHistory[i][j] = 0
		}

		m.momRunningSums[i] = 0
		m.shortRunningSums[i] = 0
		m.truePeak[i] = 0
	}

	m.momWriteIdx = 0
	m.shortWriteIdx = 0
	m.samplesSinceStep = 0
	m.totalSamples = 0
	m.blocks = nil
}

// StartIntegration starts accumulating blocks for integrated loudness.
func (m *Meter) StartIntegration() {
	m.integrationRunning = true
}

// StopIntegration stops accumulating blocks for integrated loudness.
func (m *Meter) StopIntegration() {
	m.integrationRunning = false
}

// ProcessSample processes a single multi-channel sample (frame).
func (m *Meter) ProcessSample(samples []float64) {
	if len(samples) < m.channels {
		return
	}

	sumCurrentBlock := 0.0

	for i := range m.channels {
		// 1. K-Weighting
		val := m.shelfFilters[i].ProcessSample(samples[i])
		val = m.hpfFilters[i].ProcessSample(val)

		// 2. Peak tracking
		absVal := math.Abs(samples[i])
		if absVal > m.truePeak[i] {
			m.truePeak[i] = absVal
		}

		sq := val * val

		// 3. Momentary integration (sliding window)
		oldMom := m.momHistory[i][m.momWriteIdx]
		m.momHistory[i][m.momWriteIdx] = sq

		m.momRunningSums[i] += sq - oldMom
		if m.momRunningSums[i] < 0 {
			m.momRunningSums[i] = 0
		}

		// 4. Short-term integration (sliding window)
		oldShort := m.shortHistory[i][m.shortWriteIdx]
		m.shortHistory[i][m.shortWriteIdx] = sq

		m.shortRunningSums[i] += sq - oldShort
		if m.shortRunningSums[i] < 0 {
			m.shortRunningSums[i] = 0
		}

		// For integrated loudness gating
		sumCurrentBlock += m.momRunningSums[i]
	}

	m.momWriteIdx = (m.momWriteIdx + 1) % m.momWindowSamples
	m.shortWriteIdx = (m.shortWriteIdx + 1) % m.shortWindowSamples

	if m.integrationRunning {
		m.totalSamples++

		m.samplesSinceStep++
		if m.samplesSinceStep >= m.blockSamplesStep {
			m.samplesSinceStep = 0
			// Add a block for integrated loudness (mean square of the last 400ms)
			// sumCurrentBlock is sum of running sums, each running sum is sum of sq in 400ms.
			// The gating block value is the mean of the sum of channel mean squares.
			// Actually BS.1770-4: z_ij is the mean square of the K-filtered signal for channel i and block j.
			// l_j = sum_i(G_i * z_ij) where G_i is channel weighting.

			meanSqSum := 0.0
			for i := range m.channels {
				meanSqSum += m.momRunningSums[i] / float64(m.momWindowSamples)
			}

			m.blocks = append(m.blocks, meanSqSum)
		}
	}
}

// ProcessBlock processes a block of interleaved samples.
func (m *Meter) ProcessBlock(block []float64) {
	for i := 0; i < len(block); i += m.channels {
		m.ProcessSample(block[i : i+m.channels])
	}
}

// Momentary returns the current momentary loudness in LUFS.
func (m *Meter) Momentary() float64 {
	meanSqSum := 0.0
	for i := range m.channels {
		meanSqSum += m.momRunningSums[i] / float64(m.momWindowSamples)
	}

	return toLUFS(meanSqSum)
}

// ShortTerm returns the current short-term loudness in LUFS.
func (m *Meter) ShortTerm() float64 {
	meanSqSum := 0.0
	for i := range m.channels {
		meanSqSum += m.shortRunningSums[i] / float64(m.shortWindowSamples)
	}

	return toLUFS(meanSqSum)
}

// Integrated returns the integrated loudness in LUFS since StartIntegration.
func (m *Meter) Integrated() float64 {
	if len(m.blocks) == 0 {
		return -math.Inf(1)
	}

	// 1. Absolute gating
	var absGated []float64

	absGatedSum := 0.0

	for _, b := range m.blocks {
		l := toLUFS(b)
		if l > absThreshold {
			absGated = append(absGated, b)
			absGatedSum += b
		}
	}

	if len(absGated) == 0 {
		return -math.Inf(1)
	}

	// 2. Relative gating
	gammaRel := toLUFS(absGatedSum/float64(len(absGated))) + relThreshold

	var (
		relGatedSum   float64
		relGatedCount int
	)

	for _, b := range absGated {
		if toLUFS(b) > gammaRel {
			relGatedSum += b
			relGatedCount++
		}
	}

	if relGatedCount == 0 {
		return -math.Inf(1)
	}

	return toLUFS(relGatedSum / float64(relGatedCount))
}

// Peaks returns the maximum absolute peak value per channel since Reset.
func (m *Meter) Peaks() []float64 {
	p := make([]float64, m.channels)
	copy(p, m.truePeak)

	return p
}

func toLUFS(meanSquare float64) float64 {
	if meanSquare <= 0 {
		return -120.0 // Effective floor
	}

	return -0.691 + 10.0*math.Log10(meanSquare)
}
