package loudness

import (
	"fmt"
	"math"
)

func targetLog10Positive(value float64) float64 {
	// Frexp also normalizes subnormals before the logarithm. This keeps
	// positive, below-gate energies usable across native and WASM math paths.
	if value >= 0x1p-1022 {
		return math.Log10(value)
	}

	mantissa, exponent := math.Frexp(value)

	return math.Log10(mantissa) + float64(exponent)*math.Log10(2)
}

type targetPhase uint8

const (
	targetInput targetPhase = iota
	targetSort
	targetMeans
	targetSearch
	targetVerifyAbsolute
	targetVerifyRelative
	targetDone
	targetMeasurement
	targetMeasurementDone
)

// FinishStep freezes input and performs at most maxWork elementary operations:
// one merge-sort move/setup, one suffix-mean element, one search comparison,
// or one gate-verification element. Repeat until done, then call Result.
// Nonpositive budgets leave streaming state unchanged. A completed result is
// idempotent; silence, short input and unrepresentable plans are typed errors.
func (a *TargetAnalyzer) FinishStep(maxWork int) (bool, error) {
	if a == nil || a.maxFrames <= 0 {
		return false, fmt.Errorf("loudness.target.finish: unconfigured analyzer: %w", ErrState)
	}

	if maxWork <= 0 {
		return false, fmt.Errorf("loudness.target.finish: positive work budget required: %w", ErrInvalid)
	}

	if a.failure != nil {
		return false, a.failure
	}

	if a.phase == targetMeasurement || a.phase == targetMeasurementDone {
		return false, fmt.Errorf("loudness.target.finish: measurement-only mode selected: %w", ErrState)
	}

	if a.phase == targetDone {
		return true, nil
	}

	if a.phase == targetInput {
		if a.frames < integratedEndpoint(a.sampleRate, 4) {
			return false, a.fail(fmt.Errorf("loudness.target.finish: %w", ErrTooShort))
		}

		if len(a.energies) == 0 {
			return false, a.fail(fmt.Errorf("loudness.target.finish: zero complete-window energy: %w", ErrBelowGate))
		}

		a.workspace = a.workspace[:len(a.energies)]
		a.phase = targetSort
	}

	for work := 0; work < maxWork && a.phase != targetDone; work++ {
		if err := a.finishUnit(); err != nil {
			return false, a.fail(err)
		}
	}

	return a.phase == targetDone, nil
}

func (a *TargetAnalyzer) finishUnit() error {
	switch a.phase {
	case targetSort:
		a.sortUnit()
	case targetMeans:
		a.meanUnit()
	case targetSearch:
		return a.searchUnit()
	case targetVerifyAbsolute:
		a.verifyAbsoluteUnit()
	case targetVerifyRelative:
		a.verifyRelativeUnit()
	}

	return nil
}

func (a *TargetAnalyzer) sortUnit() {
	n := len(a.energies)
	if a.sortWidth >= n {
		a.phase, a.index = targetMeans, n-1
		return
	}

	if !a.merging {
		if a.sortLeft >= n {
			a.energies, a.workspace = a.workspace, a.energies
			a.sortWidth *= 2
			a.sortLeft = 0

			return
		}

		a.mergeI = a.sortLeft
		a.mergeMid = min(a.sortLeft+a.sortWidth, n)
		a.mergeJ = a.mergeMid
		a.mergeEnd = min(a.mergeMid+a.sortWidth, n)
		a.mergeOut = a.sortLeft
		a.merging = true

		return
	}

	if a.mergeOut == a.mergeEnd {
		a.sortLeft, a.merging = a.mergeEnd, false
		return
	}

	if a.mergeI < a.mergeMid && (a.mergeJ == a.mergeEnd || a.energies[a.mergeI] <= a.energies[a.mergeJ]) {
		a.workspace[a.mergeOut] = a.energies[a.mergeI]
		a.mergeI++
	} else {
		a.workspace[a.mergeOut] = a.energies[a.mergeJ]
		a.mergeJ++
	}

	a.mergeOut++
}

func (a *TargetAnalyzer) meanUnit() {
	if a.index < 0 {
		a.phase = targetSearch
		a.searchIndex, a.relativeIndex = len(a.energies)-1, len(a.energies)

		return
	}

	i := a.index
	energy := a.energies[i]

	mean := energy
	if i+1 < len(a.energies) {
		// Descending incremental means never form a potentially overflowing
		// sum, and adding smaller positive values cannot overflow subtraction.
		mean = a.workspace[i+1] + (energy-a.workspace[i+1])/float64(len(a.energies)-i)
	}

	a.workspace[i] = mean
	if energy > integratedAbsoluteEnergy() && energy > a.absMean*0.1 {
		a.sourceCount++
		a.sourceMean += (energy - a.sourceMean) / float64(a.sourceCount)
	}

	a.index--
}

func (a *TargetAnalyzer) searchUnit() error {
	i := a.searchIndex
	if i < 0 {
		return fmt.Errorf("loudness.target.finish: no representable consistent gain interval: %w", ErrNumericalOverflow)
	}
	// Equal energies enter the strict absolute gate together. Only the first
	// index of each group describes an attainable absolute-gated suffix.
	if i > 0 && a.energies[i] == a.energies[i-1] {
		a.searchIndex--
		return nil
	}

	threshold := a.workspace[i] * 0.1
	if a.relativeIndex > i && a.energies[a.relativeIndex-1] > threshold {
		a.relativeIndex--
		return nil
	}
	// Within this absolute suffix, gain² cancels from the relative gate.
	// Therefore the selected original mean is fixed and gain is analytic.
	mean := a.workspace[a.relativeIndex]
	db := a.target - (-0.691 + 10*targetLog10Positive(mean))
	gain := math.Pow(10, db/20)
	peak := a.peak * gain
	a.searchIndex--

	if !integratedFinite(db) || !integratedFinite(gain) || gain <= 0 || !integratedFinite(peak) || peak <= 0 {
		return nil
	}

	actualDB := 20 * math.Log10(gain)

	cutoff := math.Exp(math.Log(integratedAbsoluteEnergy()) - 2*math.Log(gain))
	if !(a.energies[i] > cutoff) || (i > 0 && a.energies[i-1] > cutoff) {
		return nil
	}
	// Enumerating suffixes from highest energy down chooses the smallest
	// representable gain, reducing unnecessary peak growth when there are
	// several solutions. Ordinary gain bisection is invalid: new quiet blocks
	// can cause downward jumps in gated loudness as gain increases.
	a.result = TargetResult{
		Plan:   NormalizationPlan{GainDB: db, Gain: gain, PredictedSamplePeak: peak},
		Frames: a.frames, SamplePeak: a.peak, NeedsFloat32Verification: true,
	}
	if a.sourceCount > 0 {
		a.result.HasMeasuredLUFS = true
		a.result.MeasuredLUFS = -0.691 + 10*math.Log10(a.sourceMean)
	}

	a.cutoff, a.gainDB = cutoff, actualDB
	a.verifiedAbsMean, a.verifiedMean = 0, 0
	a.verifiedAbsCount, a.verifiedCount = 0, 0
	a.phase, a.index = targetVerifyAbsolute, 0

	return nil
}

func (a *TargetAnalyzer) verifyAbsoluteUnit() {
	if a.index == len(a.energies) {
		a.phase, a.index = targetVerifyRelative, 0
		return
	}

	energy := a.energies[a.index]
	if energy > a.cutoff {
		a.verifiedAbsCount++
		a.verifiedAbsMean += (energy - a.verifiedAbsMean) / float64(a.verifiedAbsCount)
	}

	a.index++
}

func (a *TargetAnalyzer) verifyRelativeUnit() {
	if a.index == len(a.energies) {
		predicted := -0.691 + 10*targetLog10Positive(a.verifiedMean) + a.gainDB
		if a.verifiedCount == 0 || !integratedFinite(predicted) || math.Abs(predicted-a.target) > 0.01 {
			// Near a gate tie, finite mean arithmetic can change membership.
			// Try the next interval rather than publishing a fictitious target.
			a.phase = targetSearch
			return
		}

		a.result.PredictedLUFS, a.phase = predicted, targetDone

		return
	}

	energy := a.energies[a.index]
	if energy > a.cutoff && energy > a.verifiedAbsMean*0.1 {
		a.verifiedCount++
		a.verifiedMean += (energy - a.verifiedMean) / float64(a.verifiedCount)
	}

	a.index++
}
