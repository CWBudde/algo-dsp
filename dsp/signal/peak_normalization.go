package signal

import (
	"fmt"
	"math"
)

// PeakNormalizationPlan is one linked gain for all supplied audio channels.
// PredictedSamplePeak is not true peak; no clipping or limiting is implied.
type PeakNormalizationPlan struct {
	GainDB, Gain, PredictedSamplePeak float64
}

// PlanPeakNormalization computes a gain from a finite nonnegative input sample
// peak and a finite target in dBFS. Silence returns an identity plan, not an
// invented measurable peak. Nonzero sources require a positive, representable
// target, coefficient and predicted peak; targets are never UI-clamped.
func PlanPeakNormalization(inputPeak, targetDBFS float64) (PeakNormalizationPlan, error) {
	if math.IsNaN(inputPeak) || math.IsInf(inputPeak, 0) || inputPeak < 0 || math.IsNaN(targetDBFS) || math.IsInf(targetDBFS, 0) {
		return PeakNormalizationPlan{}, fmt.Errorf("signal.peak-normalization: finite nonnegative peak and finite target required")
	}

	if inputPeak == 0 {
		return PeakNormalizationPlan{Gain: 1}, nil
	}

	target := math.Pow(10, targetDBFS/20)
	mantissa, exponent := math.Frexp(inputPeak)
	inputDB := 20 * (math.Log10(mantissa) + float64(exponent)*math.Log10(2))
	db := targetDBFS - inputDB
	gain := target / inputPeak

	peak := inputPeak * gain
	if math.IsNaN(db) || math.IsInf(db, 0) || math.IsNaN(gain) || math.IsInf(gain, 0) || gain <= 0 ||
		math.IsNaN(peak) || math.IsInf(peak, 0) || peak <= 0 || math.IsInf(target, 0) || target <= 0 {
		return PeakNormalizationPlan{}, fmt.Errorf("signal.peak-normalization: gain or target peak is unrepresentable")
	}

	return PeakNormalizationPlan{GainDB: db, Gain: gain, PredictedSamplePeak: peak}, nil
}
