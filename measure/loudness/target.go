package loudness

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// TargetResult describes one linked gain whose predicted, fully re-gated
// integrated loudness reaches the requested target within 0.01 LU. Prediction
// uses linear K-weighting and the captured window energies, not a new source
// scan. Float32 output rounding is a separate effect: if NeedsFloat32Verification
// is true, a caller claiming that tolerance for stored float32 output must
// independently measure that output before publication.
//
// HasMeasuredLUFS distinguishes an input below the original absolute gate from
// a finite source measurement. MeasuredLUFS is zero and has no meaning when that
// flag is false. Positive below-gate sources can be normalized; silence cannot.
// SamplePeak includes every supplied channel, including zero-weight channels.
// Neither SamplePeak nor the plan predicts true peak or implies peak limiting.
type TargetResult struct {
	Plan                     NormalizationPlan
	Frames                   int64
	SamplePeak               float64
	MeasuredLUFS             float64
	HasMeasuredLUFS          bool
	PredictedLUFS            float64
	NeedsFloat32Verification bool
}

type targetFilterState struct {
	s0, s1, h0, h1 float64
}

// TargetAnalyzer measures complete 400 ms windows at precise 100 ms endpoints
// and solves normalization with both absolute and relative gates recalculated.
// It stores every positive window energy, including originally below-gate
// windows, never a complete source. Four positive hop sums replace subtractive
// rolling sums, so a loud-to-quiet transition cannot leave a stale energy tail.
// The published IntegratedAnalyzer's summation and outputs are unchanged.
//
// One caller owns processing/finalization/Reset. Input preflight is atomic and
// successful processing and FinishStep allocate nothing. Invalid input leaves
// streaming state unchanged; arithmetic overflow is terminal until Reset.
// FinishStep is cooperative, with a bound on elementary finalization work, not
// an unbounded sort hidden in its first call. Input blocks are at most 65536
// frames, and total analyzer-owned workspace is at most 64 MiB.
type TargetAnalyzer struct {
	weights                                      []float64
	filters                                      []targetFilterState
	shelf, highpass                              biquad.Coefficients
	energies, workspace                          []float64
	blockEnergy                                  []float64
	sampleRate, target                           float64
	maxFrames, frames                            int64
	hopIndex, nextHop                            int64
	hopSum                                       float64
	hops                                         [4]float64
	peak, absMean                                float64
	absCount                                     int
	phase                                        targetPhase
	index                                        int
	sortWidth, sortLeft                          int
	mergeI, mergeJ                               int
	mergeMid, mergeEnd                           int
	mergeOut                                     int
	merging                                      bool
	searchIndex, relativeIndex                   int
	sourceMean, verifiedAbsMean, verifiedMean    float64
	sourceCount, verifiedAbsCount, verifiedCount int
	cutoff, gainDB                               float64
	result                                       TargetResult
	measurement                                  IntegratedResult
	failure                                      error
}

// NewTargetAnalyzer validates configuration and a finite target strictly above
// -70 LUFS before allocating. Targets at/below the strict absolute gate cannot
// be achieved by any nonempty set of gated windows. Config uses packed channel
// weights exactly as IntegratedAnalyzer; no speaker layout is inferred.
func NewTargetAnalyzer(config IntegratedConfig, targetLUFS float64) (*TargetAnalyzer, error) {
	if !integratedFinite(targetLUFS) || targetLUFS <= -70 {
		return nil, fmt.Errorf("loudness.target.new: finite target above -70 LUFS required: %w", ErrInvalid)
	}

	_, _, capacity, err := validateIntegratedConfig(config)
	if err != nil {
		return nil, fmt.Errorf("loudness.target.new: %w", err)
	}
	// Two window arrays are reused for merge sorting and suffix means. All
	// capacity checks precede integer multiplication/conversion/allocation.
	fixed := int64(MaxIntegratedBlockFrames)*8 + int64(config.Channels)*128 + 4096
	if int64(capacity) > (MaxIntegratedWorkspaceBytes-fixed)/16 {
		return nil, fmt.Errorf("loudness.target.new: workspace exceeds %d bytes: %w", MaxIntegratedWorkspaceBytes, ErrLimit)
	}

	a := &TargetAnalyzer{
		weights: make([]float64, config.Channels), filters: make([]targetFilterState, config.Channels),
		energies: make([]float64, 0, capacity), workspace: make([]float64, capacity),
		blockEnergy: make([]float64, MaxIntegratedBlockFrames), sampleRate: config.SampleRate,
		target: targetLUFS, maxFrames: config.MaxFrames,
	}

	a.shelf, a.highpass = integratedKWeighting(config.SampleRate)
	for channel := range a.weights {
		a.weights[channel] = 1
		if config.ChannelWeights != nil {
			a.weights[channel] = config.ChannelWeights[channel]
		}
	}

	a.Reset()

	return a, nil
}

// ProcessPlanar reads a finite, equal-length planar float64 block without
// retaining or changing caller slices. Invalid preflight is atomic.
func (a *TargetAnalyzer) ProcessPlanar(block [][]float64) error {
	return processTargetPlanar(a, block)
}

// ProcessPlanar32 is ProcessPlanar for float32 storage, without a source-copy
// or intermediate channel conversion buffer.
func (a *TargetAnalyzer) ProcessPlanar32(block [][]float32) error {
	peak, err := preflightTargetPlanar32(a, block)
	if err != nil {
		return err
	}

	return processPreparedTargetPlanar(a, block, peak)
}

func processTargetPlanar[T ~float32 | ~float64](a *TargetAnalyzer, block [][]T) error {
	peak, err := preflightTargetPlanar(a, block)
	if err != nil {
		return err
	}

	return processPreparedTargetPlanar(a, block, peak)
}

func processPreparedTargetPlanar[T ~float32 | ~float64](a *TargetAnalyzer, block [][]T, peak float64) error {
	if len(block) == 1 && a.weights[0] == 1 {
		return processTargetMono(a, block[0], peak)
	}

	if len(block) == 2 && a.weights[0] == 1 && a.weights[1] == 1 {
		return processTargetStereo(a, block[0], block[1], peak)
	}

	return processPreparedTargetPlanarGeneric(a, block, peak)
}

// Retain the channel-major prepared path as the fallback for larger/custom
// layouts and as an independent scalar reference for fused-path tests.
func processPreparedTargetPlanarGeneric[T ~float32 | ~float64](a *TargetAnalyzer, block [][]T, peak float64) error {
	frames := len(block[0])
	energies := a.blockEnergy[:frames]
	clear(energies)

	s, h := a.shelf, a.highpass
	for channel, input := range block {
		weight := a.weights[channel]
		if weight == 0 {
			continue
		}

		state := a.filters[channel]
		s0, s1, h0, h1 := state.s0, state.s1, state.h0, state.h1

		for frame, sample := range input {
			x := float64(sample)
			y := s.B0*x + s0
			s0 = s.B1*x - s.A1*y + s1
			s1 = s.B2*x - s.A2*y
			z := h.B0*y + h0
			h0 = h.B1*y - h.A1*z + h1
			h1 = h.B2*y - h.A2*z
			energies[frame] += weight * z * z
		}

		a.filters[channel] = targetFilterState{s0: s0, s1: s1, h0: h0, h1: h1}
		if !integratedFinite(s0) || !integratedFinite(s1) || !integratedFinite(h0) || !integratedFinite(h1) {
			return a.fail(fmt.Errorf("loudness.target.process: filter state: %w", ErrNumericalOverflow))
		}
	}

	a.peak = peak
	for start := 0; start < frames; {
		count := min(frames-start, int(a.nextHop-a.frames))
		sum := a.hopSum
		// Preserve the original left-to-right addition order. Every energy is
		// nonnegative or nonfinite, so an overflowing sum cannot recover through
		// cancellation. Checking each endpoint/partial segment detects failure
		// in the same bounded call without a finite/state check on every frame.
		for _, energy := range energies[start : start+count] {
			sum += energy
		}

		if !integratedFinite(sum) {
			return a.fail(fmt.Errorf("loudness.target.process: hop energy: %w", ErrNumericalOverflow))
		}

		a.hopSum = sum
		a.frames += int64(count)
		start += count

		if a.frames == a.nextHop {
			if err := a.recordHop(); err != nil {
				return err
			}
		}
	}

	return nil
}

func preflightTargetPlanar[T ~float32 | ~float64](a *TargetAnalyzer, block [][]T) (float64, error) {
	if a == nil || a.maxFrames <= 0 || len(a.blockEnergy) == 0 {
		return 0, fmt.Errorf("loudness.target.process: unconfigured analyzer: %w", ErrState)
	}

	if a.failure != nil {
		return 0, a.failure
	}

	if a.phase != targetInput {
		return 0, fmt.Errorf("loudness.target.process: input finalized: %w", ErrState)
	}

	if len(block) != len(a.weights) || len(block) == 0 {
		return 0, fmt.Errorf("loudness.target.process: channel count: %w", ErrInvalid)
	}

	frames := len(block[0])
	if frames > MaxIntegratedBlockFrames || int64(frames) > a.maxFrames-a.frames {
		return 0, fmt.Errorf("loudness.target.process: frame count: %w", ErrLimit)
	}

	peak := a.peak

	for _, channel := range block {
		if len(channel) != frames {
			return 0, fmt.Errorf("loudness.target.process: unequal channel lengths: %w", ErrInvalid)
		}

		for _, sample := range channel {
			value := float64(sample)
			if !integratedFinite(value) {
				return 0, fmt.Errorf("loudness.target.process: sample: %w", ErrNonFinite)
			}

			if value < 0 {
				value = -value
			}

			if value > peak {
				peak = value
			}
		}
	}

	return peak, nil
}

func (a *TargetAnalyzer) recordHop() error {
	a.hops[a.hopIndex%4] = a.hopSum
	a.hopSum = 0

	a.hopIndex++
	if a.hopIndex >= 4 {
		sum := 0.0
		for offset := int64(0); offset < 4; offset++ {
			sum += a.hops[(a.hopIndex-4+offset)%4]
		}

		if !integratedFinite(sum) {
			return a.fail(fmt.Errorf("loudness.target.process: window energy: %w", ErrNumericalOverflow))
		}

		frames := a.frames - integratedEndpoint(a.sampleRate, a.hopIndex-4)

		energy := sum / float64(frames)
		if energy > 0 {
			a.energies = append(a.energies, energy)
			if energy > integratedAbsoluteEnergy() {
				a.absCount++
				a.absMean += (energy - a.absMean) / float64(a.absCount)
			}
		}
	}

	a.nextHop = integratedEndpoint(a.sampleRate, a.hopIndex+1)

	return nil
}

func (a *TargetAnalyzer) fail(err error) error {
	a.failure = err
	return err
}

// Result returns only a successfully finished plan. It never fabricates a
// source LUFS for material that originally passed no absolute-gated window.
func (a *TargetAnalyzer) Result() (TargetResult, error) {
	if a == nil || a.maxFrames <= 0 {
		return TargetResult{}, fmt.Errorf("loudness.target.result: unconfigured analyzer: %w", ErrState)
	}

	if a.failure != nil {
		return TargetResult{}, a.failure
	}

	if a.phase != targetDone {
		return TargetResult{}, fmt.Errorf("loudness.target.result: finish incomplete: %w", ErrState)
	}

	return a.result, nil
}

// Reset clears all source, sorting, planning and terminal-error state while
// reusing the constructor's reserved workspace and retaining config/target.
func (a *TargetAnalyzer) Reset() {
	if a == nil {
		return
	}

	clear(a.filters)
	a.energies = a.energies[:0]
	a.workspace = a.workspace[:cap(a.workspace)]
	a.frames, a.hopIndex, a.nextHop = 0, 0, integratedEndpoint(a.sampleRate, 1)
	a.hopSum, a.hops, a.peak, a.absMean, a.absCount = 0, [4]float64{}, 0, 0, 0
	a.phase, a.index, a.sortWidth, a.sortLeft, a.merging = targetInput, 0, 1, 0, false
	a.mergeI, a.mergeJ, a.mergeMid, a.mergeEnd, a.mergeOut = 0, 0, 0, 0, 0
	a.searchIndex, a.relativeIndex = 0, 0
	a.sourceMean, a.verifiedAbsMean, a.verifiedMean = 0, 0, 0
	a.sourceCount, a.verifiedAbsCount, a.verifiedCount = 0, 0, 0
	a.cutoff, a.gainDB, a.result, a.failure = 0, 0, TargetResult{}, nil
	a.measurement = IntegratedResult{}
}
