package loudness

import (
	"fmt"
	"math"
)

// FinishMeasurementStep selects measurement-only finalization and performs at
// most maxWork elementary operations: one captured energy or completion of the
// result. It applies the original absolute/relative gates to the already measured
// source; no sorting, normalization solve or sample rescan is performed. This
// verifies an actual float32 candidate using the same optimized streaming scan,
// independently of the source's predicted gain.
//
// Repeat until done, then call MeasurementResult. Positive budgets freeze input;
// invalid budgets leave state unchanged. Measurement-only finalization and
// target-planning finalization are mutually exclusive until Reset. Short input,
// all-below-gate input and arithmetic overflow retain their typed error semantics.
func (a *TargetAnalyzer) FinishMeasurementStep(maxWork int) (bool, error) {
	if a == nil || a.maxFrames <= 0 {
		return false, fmt.Errorf("loudness.target.measure: unconfigured analyzer: %w", ErrState)
	}

	if maxWork <= 0 {
		return false, fmt.Errorf("loudness.target.measure: positive work budget required: %w", ErrInvalid)
	}

	if a.failure != nil {
		return false, a.failure
	}

	if a.phase != targetInput && a.phase != targetMeasurement && a.phase != targetMeasurementDone {
		return false, fmt.Errorf("loudness.target.measure: target-planning mode selected: %w", ErrState)
	}

	if a.phase == targetMeasurementDone {
		return true, nil
	}

	if a.phase == targetInput {
		a.phase, a.index = targetMeasurement, 0

		a.sourceMean, a.sourceCount = 0, 0
		if a.frames < integratedEndpoint(a.sampleRate, 4) {
			return false, a.fail(fmt.Errorf("loudness.target.measure: %w", ErrTooShort))
		}

		if a.absCount == 0 {
			return false, a.fail(fmt.Errorf("loudness.target.measure: %w", ErrBelowGate))
		}
	}

	for work := 0; work < maxWork; work++ {
		if a.index == len(a.energies) {
			if a.sourceCount == 0 || a.sourceMean <= 0 || !integratedFinite(a.sourceMean) {
				return false, a.fail(fmt.Errorf("loudness.target.measure: %w", ErrBelowGate))
			}

			lufs := -0.691 + 10*math.Log10(a.sourceMean)
			if !integratedFinite(lufs) {
				return false, a.fail(fmt.Errorf("loudness.target.measure: result: %w", ErrNumericalOverflow))
			}

			a.measurement = IntegratedResult{LUFS: lufs, SamplePeak: a.peak, Frames: a.frames}
			a.phase = targetMeasurementDone

			return true, nil
		}

		energy := a.energies[a.index]
		if energy > integratedAbsoluteEnergy() && energy > a.absMean*0.1 {
			a.sourceCount++
			a.sourceMean += (energy - a.sourceMean) / float64(a.sourceCount)
		}

		a.index++
	}

	return false, nil
}

// MeasurementResult returns a successfully completed finite, original-gated
// source measurement. It is unavailable in target-planning mode, before finish
// completion, or after terminal failure. Unlike TargetResult, it cannot express
// below-gate source loudness: that case returns ErrBelowGate rather than a
// fabricated LUFS. Four-positive-hop summation can differ by rounding from the
// published IntegratedAnalyzer; its implementation and outputs are untouched.
func (a *TargetAnalyzer) MeasurementResult() (IntegratedResult, error) {
	if a == nil || a.maxFrames <= 0 {
		return IntegratedResult{}, fmt.Errorf("loudness.target.measurement-result: unconfigured analyzer: %w", ErrState)
	}

	if a.failure != nil {
		return IntegratedResult{}, a.failure
	}

	if a.phase != targetMeasurementDone {
		return IntegratedResult{}, fmt.Errorf("loudness.target.measurement-result: measurement incomplete or other mode selected: %w", ErrState)
	}

	return a.measurement, nil
}

// SamplePeak returns the progressive finite source sample peak, including
// zero-weight channels; an unconfigured analyzer or Reset returns zero. Atomic
// preflight rejection leaves it unchanged. On terminal arithmetic failure it
// remains a finite diagnostic, not a valid measurement or a successful plan:
// Result and MeasurementResult continue to report the terminal error. A block
// whose filter state overflowed need not have contributed to this diagnostic.
func (a *TargetAnalyzer) SamplePeak() float64 {
	if a == nil {
		return 0
	}

	return a.peak
}
