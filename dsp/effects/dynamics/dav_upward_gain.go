package dynamics

import (
	"fmt"
	"math"
)

// DAVSoftKneeUpwardGain reproduces the DAV Lightweight Upward Compressor's
// transfer curve. The ratio controls which side of the threshold receives
// gain. Detector history and makeup gain belong to the caller.
type DAVSoftKneeUpwardGain struct {
	thresholdLog2 float32
	kneeSquared   float32
	slope         float32
}

// NewDAVSoftKneeUpwardGain prepares the transfer for the given threshold and
// nonnegative knee in dB and a positive ratio. Ratios below one apply gain
// to quiet peaks. Ratios above one apply gain to loud peaks.
func NewDAVSoftKneeUpwardGain(thresholdDB, kneeDB, ratio float64) (DAVSoftKneeUpwardGain, error) {
	threshold, squared, valid := davCompressorCurveInputs(thresholdDB, kneeDB)
	if !(ratio > 0 && ratio <= math.MaxFloat32) || !valid {
		return DAVSoftKneeUpwardGain{}, fmt.Errorf("invalid DAV upward gain: threshold %g dB, knee %g dB, ratio %g",
			thresholdDB, kneeDB, ratio)
	}
	return DAVSoftKneeUpwardGain{
		thresholdLog2: threshold, kneeSquared: squared, slope: 1 - float32(ratio),
	}, nil
}

// Gain maps a nonnegative linear peak to amplification. Nonpositive peaks
// use the smallest normal binary32 input. NaN returns unity. The result
// saturates at the largest finite binary32 gain when the original exponent
// construction would overflow.
func (c DAVSoftKneeUpwardGain) Gain(peak float64) float64 {
	if math.IsNaN(peak) {
		return 1
	}
	logPeak := davLog2Extended(float32(min(max(peak, 0x1p-126), math.MaxFloat32)))
	delta := float64(c.slope) * (float64(c.thresholdLog2) - logPeak)
	root := davCompressorRoot(delta*delta + float64(c.kneeSquared))
	return min(davPower2Extended(delta+root), math.MaxFloat32)
}
