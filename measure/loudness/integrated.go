package loudness

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

const (
	// MaxIntegratedWorkspaceBytes limits analyzer-owned workspace, not input.
	MaxIntegratedWorkspaceBytes int64 = 64 << 20
	// MaxIntegratedBlockFrames bounds one caller-supplied planar block.
	MaxIntegratedBlockFrames = 65536
)

var (
	// ErrTooShort means no complete 400 ms measurement block is available.
	ErrTooShort = errors.New("loudness: fewer than 400 ms of input")
	// ErrBelowGate means no measurement block passed the absolute -70 LUFS gate.
	ErrBelowGate = errors.New("loudness: all blocks are below the absolute gate")
	// ErrNonFinite identifies NaN or infinite input samples.
	ErrNonFinite = errors.New("loudness: nonfinite input")
	// ErrNumericalOverflow identifies unrepresentable finite-input arithmetic.
	// Analyzer overflow is terminal until Reset.
	ErrNumericalOverflow = errors.New("loudness: unrepresentable arithmetic")
	// ErrLimit identifies a frame, caller-block, or workspace limit violation.
	ErrLimit = errors.New("loudness: configured limit exceeded")
	// ErrState identifies input after finalization or a result before completion.
	ErrState = errors.New("loudness: invalid analyzer state")
	// ErrInvalid identifies invalid configuration, block shape, or gain parameters.
	ErrInvalid = errors.New("loudness: invalid parameters")
)

// IntegratedConfig configures a bounded BS.1770 integrated-only analyzer.
// SampleRate must be finite in [8000,384000], Channels in [1,32], and MaxFrames
// positive. Workspace for the maximum number of 100 ms gating hops is reserved
// at construction; limits exceeding 64 MiB are rejected before allocation.
//
// ChannelWeights are power weights in packed input order, copied at construction.
// Nil means all ones: no speaker layout is inferred from channel count. Explicit
// weights must have Channels entries in [0,16], with at least one positive entry.
// A five-channel L/R/C/Ls/Rs layout uses 1/1/1/1.41/1.41; LFE uses 0.
// Other speaker layouts require their own explicit mapping. Zero-weight channels
// are excluded from loudness but still checked for finite input and sample peaks.
type IntegratedConfig struct {
	SampleRate     float64
	Channels       int
	ChannelWeights []float64
	MaxFrames      int64
}

// IntegratedResult is a finite, completed measurement. SamplePeak includes all
// supplied channels, including zero-weight channels; it is not true peak.
type IntegratedResult struct {
	LUFS, SamplePeak float64
	Frames           int64
}

// IntegratedAnalyzer measures complete 400 ms blocks, with 100 ms hops and
// absolute -70 LUFS / relative -10 LU gating. It retains a combined weighted
// energy ring and about ten scalar energies per second, never a whole source.
// Window j spans nearest-sample endpoints round(j*SampleRate/10) through
// round((j+4)*SampleRate/10), exclusive. Endpoints accumulate nominal time,
// rather than accumulating a rounded hop, so fractional-frame rates do not drift.
// K-weighting is reset at the start of the supplied selection. Incomplete EOF
// blocks are discarded, not padded. This integrated-only API makes no EBU Mode,
// loudness-range, true-peak, or peak-limiting claim.
//
// Construct with NewIntegratedAnalyzer; the zero value is unconfigured.
// A single caller owns the analyzer. Processing, finalization and Reset must not
// run concurrently. Successful processing and FinishStep calls allocate nothing.
// Caller cancellation belongs between bounded blocks/finalization steps.
type IntegratedAnalyzer struct {
	weights      []float64
	shelves      []biquad.Section
	highpasses   []biquad.Section
	ring         []float64
	blocks       []float64
	maxFrames    int64
	sampleRate   float64
	frames       int64
	gateIndex    int64
	firstBlock   int64
	nextBlock    int64
	write        int
	peak         float64
	absMean      float64
	relThreshold float64
	relMean      float64
	relCount     int
	finishIndex  int
	finishing    bool
	finished     bool
	failure      error
}

// NewIntegratedAnalyzer validates all limits before allocating bounded storage.
func NewIntegratedAnalyzer(config IntegratedConfig) (*IntegratedAnalyzer, error) {
	window, firstBlock, capacity, err := validateIntegratedConfig(config)
	if err != nil {
		return nil, fmt.Errorf("loudness.new: %w", err)
	}

	a := &IntegratedAnalyzer{
		weights: make([]float64, config.Channels), shelves: make([]biquad.Section, config.Channels),
		highpasses: make([]biquad.Section, config.Channels), ring: make([]float64, window),
		blocks: make([]float64, 0, capacity), maxFrames: config.MaxFrames,
		sampleRate: config.SampleRate, firstBlock: int64(firstBlock), nextBlock: int64(firstBlock),
	}
	shelf, highpass := integratedKWeighting(config.SampleRate)

	for i := range a.weights {
		a.weights[i] = 1
		if config.ChannelWeights != nil {
			a.weights[i] = config.ChannelWeights[i]
		}

		a.shelves[i].Coefficients = shelf
		a.highpasses[i].Coefficients = highpass
	}

	return a, nil
}

func validateIntegratedConfig(c IntegratedConfig) (int, int, int, error) {
	if !integratedFinite(c.SampleRate) || c.SampleRate < 8000 || c.SampleRate > 384000 || c.Channels < 1 || c.Channels > 32 {
		return 0, 0, 0, fmt.Errorf("sample rate or channel count: %w", ErrInvalid)
	}

	if c.MaxFrames <= 0 {
		return 0, 0, 0, fmt.Errorf("MaxFrames must be positive: %w", ErrLimit)
	}

	if c.ChannelWeights != nil {
		if len(c.ChannelWeights) != c.Channels {
			return 0, 0, 0, fmt.Errorf("channel weight count: %w", ErrInvalid)
		}

		positive := false

		for _, weight := range c.ChannelWeights {
			if !integratedFinite(weight) || weight < 0 || weight > 16 {
				return 0, 0, 0, fmt.Errorf("channel weight must be finite in [0,16]: %w", ErrInvalid)
			}

			positive = positive || weight > 0
		}

		if !positive {
			return 0, 0, 0, fmt.Errorf("at least one positive channel weight required: %w", ErrInvalid)
		}
	}

	window := int(math.Ceil(c.SampleRate * 4 / 10))
	if math.Mod(c.SampleRate, 10) != 0 {
		// One extra frame accommodates independently rounded fractional endpoints.
		// Multiple-of-ten rates retain the original ring and summation order.
		window++
	}

	firstBlock := integratedEndpoint(c.SampleRate, 4)

	capacity := 0.0
	if c.MaxFrames >= firstBlock {
		// Conservative floating bound, checked BEFORE conversion to any integer.
		// MaxInt64 frames can therefore never overflow native or WASM capacities.
		capacity = math.Ceil((float64(c.MaxFrames)+0.5)*10/c.SampleRate) - 3
	}
	// Includes both 7-double biquad states and weights, with conservative
	// per-channel/headroom allowance for slice headers and scalar state.
	fixedBytes := int64(window)*8 + int64(c.Channels)*128 + 1024
	if capacity > float64((MaxIntegratedWorkspaceBytes-fixedBytes)/8) {
		return 0, 0, 0, fmt.Errorf("workspace exceeds %d bytes: %w", MaxIntegratedWorkspaceBytes, ErrLimit)
	}

	count := int(capacity)
	for count > 0 && integratedEndpoint(c.SampleRate, int64(count)+3) > c.MaxFrames {
		count--
	}

	return window, int(firstBlock), count, nil
}

func integratedFinite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func integratedEndpoint(rate float64, tenths int64) int64 {
	return int64(math.Round(rate * float64(tenths) / 10))
}

// ProcessPlanar reads equal-length channel blocks of at most 65536 frames.
// Shapes, frame limits and all input samples are preflighted without mutation;
// invalid input leaves state unchanged. Arithmetic overflow makes the analyzer
// terminal until Reset. Input is never changed or retained.
func (a *IntegratedAnalyzer) ProcessPlanar(block [][]float64) error {
	return processIntegratedPlanar(a, block)
}

// ProcessPlanar32 is ProcessPlanar for float32 input, without a conversion copy.
func (a *IntegratedAnalyzer) ProcessPlanar32(block [][]float32) error {
	return processIntegratedPlanar(a, block)
}

func processIntegratedPlanar[T ~float32 | ~float64](a *IntegratedAnalyzer, block [][]T) error {
	if err := preflightIntegratedBlock(a, block); err != nil {
		return err
	}

	for frame := range len(block[0]) {
		energy := 0.0

		for channel := range block {
			value := float64(block[channel][frame])

			a.peak = math.Max(a.peak, math.Abs(value))
			if a.weights[channel] == 0 {
				continue
			}

			filtered := a.shelves[channel].ProcessSample(value)
			filtered = a.highpasses[channel].ProcessSample(filtered)

			energy += a.weights[channel] * filtered * filtered
			if !integratedFinite(energy) {
				return a.failIntegrated(fmt.Errorf("loudness.process: filtered energy: %w", ErrNumericalOverflow))
			}
		}

		a.ring[a.write] = energy

		a.write++
		if a.write == len(a.ring) {
			a.write = 0
		}

		a.frames++
		if a.frames == a.nextBlock {
			if err := a.recordIntegratedBlock(); err != nil {
				return err
			}

			a.gateIndex++
			a.nextBlock = integratedEndpoint(a.sampleRate, a.gateIndex+4)
		}
	}

	return nil
}

func preflightIntegratedBlock[T ~float32 | ~float64](a *IntegratedAnalyzer, block [][]T) error {
	if a == nil || a.maxFrames <= 0 || len(a.ring) == 0 {
		return fmt.Errorf("loudness.process: unconfigured analyzer: %w", ErrState)
	}

	if a.failure != nil {
		return a.failure
	}

	if a.finishing {
		return fmt.Errorf("loudness.process: input is finalized: %w", ErrState)
	}

	if len(block) != len(a.weights) || len(block) == 0 {
		return fmt.Errorf("loudness.process: channel count: %w", ErrInvalid)
	}

	frames := len(block[0])
	if frames > MaxIntegratedBlockFrames || int64(frames) > a.maxFrames-a.frames {
		return fmt.Errorf("loudness.process: frame count: %w", ErrLimit)
	}

	for _, channel := range block {
		if len(channel) != frames {
			return fmt.Errorf("loudness.process: unequal channel lengths: %w", ErrInvalid)
		}

		for _, value := range channel {
			if !integratedFinite(float64(value)) {
				return fmt.Errorf("loudness.process: input sample: %w", ErrNonFinite)
			}
		}
	}

	return nil
}

func (a *IntegratedAnalyzer) recordIntegratedBlock() error {
	// Rebase the window using only positive energies: subtracting outgoing
	// enormous values from a rolling sum can leave a spurious loud residual
	// indefinitely after a transition to near-silence. This scan costs at most
	// four energy additions per input frame amortized, and retains no source.
	frames := int(a.frames - integratedEndpoint(a.sampleRate, a.gateIndex))
	start := (a.write + len(a.ring) - frames) % len(a.ring)
	sum := 0.0

	if start < a.write {
		for _, energy := range a.ring[start:a.write] {
			sum += energy
		}
	} else {
		// Sum in ascending physical ring order, including the full-ring case,
		// preserving bit-for-bit results at existing multiple-of-ten rates.
		for _, energy := range a.ring[:a.write] {
			sum += energy
		}

		for _, energy := range a.ring[start:] {
			sum += energy
		}
	}

	if !integratedFinite(sum) {
		return a.failIntegrated(fmt.Errorf("loudness.process: window energy: %w", ErrNumericalOverflow))
	}

	energy := sum / float64(frames)
	if energy > integratedAbsoluteEnergy() {
		a.blocks = append(a.blocks, energy)
		a.absMean += (energy - a.absMean) / float64(len(a.blocks))
	}

	return nil
}

func integratedAbsoluteEnergy() float64 { return math.Pow(10, (-70+0.691)/10) }

func (a *IntegratedAnalyzer) failIntegrated(err error) error {
	a.failure = err
	return err
}

// FinishStep freezes input and examines at most maxBlocks stored energies.
// maxBlocks must be positive; invalid limits leave state unchanged. Repeat until
// done, then call Result. Short selections and all-below-gate input return typed
// errors instead of a fabricated or nonfinite LUFS. Reset permits reuse.
func (a *IntegratedAnalyzer) FinishStep(maxBlocks int) (bool, error) {
	if a == nil || a.maxFrames <= 0 || len(a.ring) == 0 {
		return false, fmt.Errorf("loudness.finish: unconfigured analyzer: %w", ErrState)
	}

	if maxBlocks <= 0 {
		return false, fmt.Errorf("loudness.finish: positive block budget required: %w", ErrInvalid)
	}

	if a.failure != nil {
		return false, a.failure
	}

	if a.finished {
		return true, nil
	}

	if !a.finishing {
		a.finishing = true
		if a.frames < a.firstBlock {
			return false, a.failIntegrated(fmt.Errorf("loudness.finish: %w", ErrTooShort))
		}

		if len(a.blocks) == 0 {
			return false, a.failIntegrated(fmt.Errorf("loudness.finish: %w", ErrBelowGate))
		}

		a.relThreshold = a.absMean * 0.1
	}

	count := min(maxBlocks, len(a.blocks)-a.finishIndex)
	for _, energy := range a.blocks[a.finishIndex : a.finishIndex+count] {
		if energy > a.relThreshold {
			a.relCount++
			a.relMean += (energy - a.relMean) / float64(a.relCount)
		}
	}

	a.finishIndex += count
	a.finished = a.finishIndex == len(a.blocks)

	return a.finished, nil
}

// Result returns the completed, finite integrated measurement. Only actual
// supplied frames are reported; no trailing filter flush or silence is added.
func (a *IntegratedAnalyzer) Result() (IntegratedResult, error) {
	if a == nil {
		return IntegratedResult{}, fmt.Errorf("loudness.result: nil analyzer: %w", ErrState)
	}

	if a.failure != nil {
		return IntegratedResult{}, a.failure
	}

	if !a.finished {
		return IntegratedResult{}, fmt.Errorf("loudness.result: finish is incomplete: %w", ErrState)
	}

	if a.relCount == 0 || a.relMean <= 0 || !integratedFinite(a.relMean) {
		return IntegratedResult{}, fmt.Errorf("loudness.result: %w", ErrBelowGate)
	}

	return IntegratedResult{LUFS: -0.691 + 10*math.Log10(a.relMean), SamplePeak: a.peak, Frames: a.frames}, nil
}

// Reset clears measurement and terminal errors, reusing all reserved storage.
func (a *IntegratedAnalyzer) Reset() {
	if a == nil {
		return
	}

	for i := range a.shelves {
		a.shelves[i].Reset()
		a.highpasses[i].Reset()
	}

	clear(a.ring)
	a.blocks = a.blocks[:0]
	a.frames, a.gateIndex, a.nextBlock, a.write = 0, 0, a.firstBlock, 0
	a.peak, a.absMean, a.relThreshold, a.relMean = 0, 0, 0, 0
	a.relCount, a.finishIndex = 0, 0
	a.finishing, a.finished, a.failure = false, false, nil
}
