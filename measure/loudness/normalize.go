package loudness

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/core"
	"github.com/cwbudde/algo-vecmath"
)

// NormalizationPlan applies one linked gain to every supplied channel. Peak is
// a sample-peak prediction, not true peak; no limiter or clipping is implied.
type NormalizationPlan struct {
	GainDB, Gain, PredictedSamplePeak float64
}

// PlanNormalization computes targetLUFS minus the original measured LUFS.
// Targets may be any finite value; unrepresentable/zero gain or a nonfinite
// predicted sample peak is rejected, never clamped to a UI gain range.
//
// IMPORTANT: this input-derived adjustment is not an unconditional guarantee
// of the post-gain integrated LUFS. Scaling can change membership of the absolute
// -70 LUFS gate, especially for material close to it. Verification requires a
// fresh measurement of the scaled result. A valid measurement is required;
// silence/below-gate or short selections must not be represented by invented LUFS.
func PlanNormalization(measured IntegratedResult, targetLUFS float64) (NormalizationPlan, error) {
	if !integratedFinite(targetLUFS) || !integratedFinite(measured.LUFS) ||
		!integratedFinite(measured.SamplePeak) || measured.SamplePeak <= 0 || measured.Frames <= 0 {
		return NormalizationPlan{}, fmt.Errorf("loudness.plan: finite target and valid measurement required: %w", ErrInvalid)
	}

	db := targetLUFS - measured.LUFS
	gain := core.DBToLinear(db)

	peak := measured.SamplePeak * gain
	if !integratedFinite(db) || !integratedFinite(gain) || gain <= 0 || !integratedFinite(peak) || peak <= 0 {
		return NormalizationPlan{}, fmt.Errorf("loudness.plan: gain or predicted peak is unrepresentable: %w", ErrNumericalOverflow)
	}

	return NormalizationPlan{GainDB: db, Gain: gain, PredictedSamplePeak: peak}, nil
}

// NormalizeLoudness measures planar input and returns fresh channel slices with
// one linked, input-derived gain. It delegates to IntegratedAnalyzer and
// PlanNormalization; see the latter's absolute-gate qualification. It never
// mutates input, clips, limits true peak, or infers a surround layout. Zero-weight
// channels are still scaled by the same gain. MaxFrames must cover the input.
//
// All source validation and planning precede output allocation. This one-shot
// convenience deliberately allocates its result; bounded streaming applications
// should instead measure blocks, plan once, and apply the gain to output blocks.
func NormalizeLoudness(planar [][]float64, targetLUFS float64, config IntegratedConfig) ([][]float64, error) {
	if !integratedFinite(targetLUFS) {
		return nil, fmt.Errorf("loudness.normalize: finite target required: %w", ErrInvalid)
	}

	if len(planar) != config.Channels || len(planar) == 0 {
		return nil, fmt.Errorf("loudness.normalize: channel count: %w", ErrInvalid)
	}

	frames := len(planar[0])
	for _, channel := range planar {
		if len(channel) != frames {
			return nil, fmt.Errorf("loudness.normalize: unequal channel lengths: %w", ErrInvalid)
		}
	}

	if int64(frames) > config.MaxFrames {
		return nil, fmt.Errorf("loudness.normalize: frame count: %w", ErrLimit)
	}

	a, err := NewIntegratedAnalyzer(config)
	if err != nil {
		return nil, fmt.Errorf("loudness.normalize: %w", err)
	}

	block := make([][]float64, len(planar))

	for start := 0; start < frames; {
		end := start + min(MaxIntegratedBlockFrames, frames-start)
		for i := range block {
			block[i] = planar[i][start:end]
		}

		if err := a.ProcessPlanar(block); err != nil {
			return nil, fmt.Errorf("loudness.normalize: %w", err)
		}

		start = end
	}

	for {
		done, err := a.FinishStep(MaxIntegratedBlockFrames)
		if err != nil {
			return nil, fmt.Errorf("loudness.normalize: %w", err)
		}

		if done {
			break
		}
	}

	measured, err := a.Result()
	if err != nil {
		return nil, fmt.Errorf("loudness.normalize: %w", err)
	}

	plan, err := PlanNormalization(measured, targetLUFS)
	if err != nil {
		return nil, fmt.Errorf("loudness.normalize: %w", err)
	}

	output := make([][]float64, len(planar))
	for i := range output {
		output[i] = make([]float64, frames)
		vecmath.ScaleBlock(output[i], planar[i], plan.Gain)
		// Prediction covers overflow; preserve explicit errors for unexpected
		// arithmetic failure rather than returning a partly invalid result.
		for _, value := range output[i] {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return nil, fmt.Errorf("loudness.normalize: output sample: %w", ErrNumericalOverflow)
			}
		}
	}

	return output, nil
}
