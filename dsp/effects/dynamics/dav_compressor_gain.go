package dynamics

import (
	"fmt"
	"math"
)

// DAVSoftKneeCompressorGain reproduces the x87 transfer curve used by the
// Delphi ASIO & VST compressor. Settings and the detector peak enter the
// curve as binary32 values. The polynomial and gain calculation use wider
// intermediates.
type DAVSoftKneeCompressorGain struct {
	thresholdLog2 float32
	kneeSquared   float32
	slope         float32
}

// NewDAVSoftKneeCompressorGain prepares a threshold and nonnegative knee in
// dB and a positive input/output ratio. Ratios below one attenuate peaks below
// the threshold.
func NewDAVSoftKneeCompressorGain(thresholdDB, kneeDB, ratio float64) (DAVSoftKneeCompressorGain, error) {
	const dbToLog2 = float64(float32(1.6609640474436811739351597147447e-1))

	threshold := float32(float64(float32(thresholdDB)) * dbToLog2)
	knee := float64(float32(kneeDB)) * dbToLog2
	squared := float32(knee * knee)

	slope := float32(.5 * (1/float64(float32(ratio)) - 1))
	if !(ratio > 0 && ratio <= math.MaxFloat32) || !(kneeDB >= 0) ||
		math.IsInf(float64(slope), 0) || math.IsInf(float64(squared), 0) ||
		math.IsNaN(float64(threshold)) || math.IsInf(float64(threshold), 0) {
		return DAVSoftKneeCompressorGain{}, fmt.Errorf("invalid DAV compressor curve: threshold %g dB, knee %g dB, ratio %g",
			thresholdDB, kneeDB, ratio)
	}

	return DAVSoftKneeCompressorGain{thresholdLog2: threshold, kneeSquared: squared, slope: slope}, nil
}

// Gain evaluates the transfer curve for a nonnegative linear peak. Zero and
// infinity use the corresponding limiting gains. Results below the floating
// point range underflow to zero.
func (c DAVSoftKneeCompressorGain) Gain(peak float64) float64 {
	delta := 0.

	if c.slope != 0 {
		switch {
		case !(peak > 0):
			if c.slope > 0 {
				return 0
			}

			return 1
		case math.IsInf(peak, 1):
			if c.slope > 0 {
				return 1
			}

			return 0
		}

		logPeak := davLog2Extended(float32(min(max(peak, 0x1p-126), math.MaxFloat32)))
		delta = float64(c.slope) * (logPeak - float64(c.thresholdLog2))
	}

	v := delta*delta + float64(c.kneeSquared)
	rounded := float32(v)
	root := math.Sqrt(v)

	if rounded >= 0x1p-126 && !math.IsInf(float64(rounded), 1) {
		seed := davSqrtSeed(rounded)
		step := seed + v/seed
		root = .25*step + v/step
	}

	return davPower2Extended(delta - root)
}

// DAVCompressorTiming returns the attack interpolation coefficient and the
// release pole for the compressor's detector loop. Times are in milliseconds
// and sampleRate is the effective detector rate, including oversampling.
func DAVCompressorTiming(attackMS, releaseMS, sampleRate float64) (attack, release float64, err error) {
	if !(attackMS > 0 && attackMS <= math.MaxFloat32 &&
		releaseMS > 0 && releaseMS <= math.MaxFloat32 &&
		sampleRate > 0 && sampleRate <= math.MaxFloat32) {
		return 0, 0, fmt.Errorf("invalid DAV compressor timing: attack %g ms, release %g ms, rate %g Hz",
			attackMS, releaseMS, sampleRate)
	}

	pole := func(ms float64) float64 {
		exponent := float32(-1 / (float64(float32(ms)) * .001 * sampleRate))
		return float64(davPower2(exponent))
	}

	return 1 - pole(attackMS), pole(releaseMS), nil
}
