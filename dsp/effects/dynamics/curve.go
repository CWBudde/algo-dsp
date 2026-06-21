package dynamics

import (
	"fmt"
	"math"
)

// StaticCurveProcessor is implemented by dynamics processors that can report
// their steady-state output level for a given input magnitude without running
// the envelope follower. Compressor, Expander, and Gate satisfy it directly; an
// individual band of a MultibandCompressor is reachable through Band(i) or via
// MultibandCompressor.BandStaticCurve.
type StaticCurveProcessor interface {
	// CalculateOutputLevel returns the steady-state output magnitude for the
	// supplied input magnitude, applying the gain computer (and makeup gain
	// where relevant) but no time-domain smoothing.
	CalculateOutputLevel(inputMagnitude float64) float64
}

// CurvePoint is one sample of a static dynamics characteristic curve.
type CurvePoint struct {
	// InputDB is the probe input level in dBFS.
	InputDB float64
	// OutputDB is the steady-state output level in dBFS.
	OutputDB float64
	// GainReductionDB is the signed transfer gain, OutputDB - InputDB: zero at
	// unity, negative for attenuation (compression/gating), positive where
	// makeup gain dominates.
	GainReductionDB float64
}

// StaticCurve samples the steady-state characteristic curve of a dynamics
// processor over [minInputDB, maxInputDB] (inclusive of both endpoints) in
// stepDB increments.
//
// It evaluates the gain-computer path through CalculateOutputLevel only, so it
// neither consumes nor mutates detector/envelope state and is safe to call on a
// live processor. This is the dB-domain analogue of the legacy
// CharacteristicCurve_dB sampling in DAV_DspDynamics.pas, where
// CharacteristicCurve(in) = TranslatePeakToGain(|in|) * in.
func StaticCurve(p StaticCurveProcessor, minInputDB, maxInputDB, stepDB float64) ([]CurvePoint, error) {
	if p == nil {
		return nil, fmt.Errorf("dynamics: nil static-curve processor")
	}

	if !isFinite(minInputDB) || !isFinite(maxInputDB) || !isFinite(stepDB) {
		return nil, fmt.Errorf("dynamics: curve bounds must be finite: min=%f max=%f step=%f", minInputDB, maxInputDB, stepDB)
	}

	if stepDB <= 0 {
		return nil, fmt.Errorf("dynamics: curve step must be positive: %f", stepDB)
	}

	if minInputDB > maxInputDB {
		return nil, fmt.Errorf("dynamics: curve min must not exceed max: min=%f max=%f", minInputDB, maxInputDB)
	}

	n := int(math.Round((maxInputDB-minInputDB)/stepDB)) + 1

	points := make([]CurvePoint, 0, n+1)
	for i := range n {
		inDB := minInputDB + float64(i)*stepDB
		if inDB > maxInputDB {
			inDB = maxInputDB
		}

		points = append(points, curvePointAt(p, inDB))
	}

	// Guarantee the final endpoint is sampled exactly even when stepDB does not
	// divide the range evenly.
	if points[len(points)-1].InputDB != maxInputDB {
		points = append(points, curvePointAt(p, maxInputDB))
	}

	return points, nil
}

func curvePointAt(p StaticCurveProcessor, inDB float64) CurvePoint {
	inputMag := mathPower10(inDB / 20.0)
	outputMag := math.Abs(p.CalculateOutputLevel(inputMag))
	outDB := ampToDB(outputMag)

	return CurvePoint{
		InputDB:         inDB,
		OutputDB:        outDB,
		GainReductionDB: outDB - inDB,
	}
}

// BandStaticCurve samples the static characteristic curve of a single band of a
// multiband compressor. It is shorthand for StaticCurve(mc.Band(band), ...) with
// band-index validation.
func (mc *MultibandCompressor) BandStaticCurve(band int, minInputDB, maxInputDB, stepDB float64) ([]CurvePoint, error) {
	if err := mc.checkBand(band); err != nil {
		return nil, err
	}

	return StaticCurve(mc.compressors[band], minInputDB, maxInputDB, stepDB)
}

// ampToDB converts a linear magnitude to dBFS, mapping non-positive input to
// negative infinity.
func ampToDB(amp float64) float64 {
	if amp <= 0 {
		return math.Inf(-1)
	}

	return 20.0 * math.Log10(amp)
}
