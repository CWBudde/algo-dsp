package design

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// PeakOrfanidisOctaves designs a second-order Orfanidis-style peaking filter
// with unity DC gain, a frequency-dependent Nyquist gain, and bandwidth in
// octaves. gain is the linear amplitude ratio at the peak, or at the dip for
// gains below one. Unity gain returns a transparent section.
//
// sampleRate is the rate at which the section will run, including oversampling.
// The octave-bandwidth equations evaluate the supplied center directly, which
// preserves the trigonometric frequency folding when the center exceeds
// Nyquist. Invalid inputs and singular designs return [biquad.Identity] with
// [ErrInvalidPeakParams].
func PeakOrfanidisOctaves(frequency, gain, bandwidth, sampleRate float64) (biquad.Coefficients, error) {
	if frequency <= 0 || gain <= 0 || bandwidth <= 0 || sampleRate <= 0 ||
		hasInvalidFloat(frequency, gain, bandwidth, sampleRate) {
		return biquad.Identity(), ErrInvalidPeakParams
	}

	w := 2 * math.Pi * (frequency / sampleRate)
	if nearMultipleOfPi(w) {
		return biquad.Identity(), ErrInvalidPeakParams
	}

	if gain == 1 {
		return biquad.Identity(), nil
	}

	const piSquared = math.Pi * math.Pi

	// The bandwidth edges sit halfway between unity and the peak in dB.
	edgeGain := math.Sqrt(gain)
	gainSquared := gain * gain
	edgeSquared := edgeGain * edgeGain
	peakToEdge := math.Abs(gainSquared - edgeSquared)
	peakToDC := math.Abs(gainSquared - 1)
	edgeToDC := math.Abs(edgeSquared - 1)

	halfOctaveWidth := bandwidth * (math.Ln2 / 2)
	angularWidth := 2 * w * math.Sinh(halfOctaveWidth)
	nyquistDistance := w*w - piSquared
	nyquistDistanceSquared := nyquistDistance * nyquistDistance
	correction := edgeToDC * piSquared * angularWidth * angularWidth / peakToEdge
	nyquistGain := math.Sqrt((nyquistDistanceSquared + gainSquared*correction) /
		(nyquistDistanceSquared + correction))

	nyquistSquared := nyquistGain * nyquistGain
	edgeToNyquist := math.Abs(edgeSquared - nyquistSquared)
	peakToNyquist := math.Abs(gainSquared - nyquistSquared)

	// The bilinear transform warps both the center and the octave bandwidth.
	tangent := math.Tan(w / 2)
	centerSquared := math.Sqrt(peakToNyquist/peakToDC) * tangent * tangent
	warpedWidth := 2 * tangent * math.Sinh(halfOctaveWidth*w/math.Sin(w))
	c := edgeToNyquist*warpedWidth*warpedWidth -
		2*centerSquared*(math.Abs(edgeSquared-nyquistGain)-math.Sqrt(edgeToDC*edgeToNyquist))
	d := 2 * centerSquared * (math.Abs(gainSquared-nyquistGain) - math.Sqrt(peakToDC*peakToNyquist))

	// Negative radicands mean the requested peak gain cannot be met; taking
	// their magnitude would silently design a section with the wrong gain.
	poleRadicand := (c + d) / peakToEdge
	zeroRadicand := (gainSquared*c + edgeSquared*d) / peakToEdge

	if !(poleRadicand >= 0 && zeroRadicand >= 0) || hasInvalidFloat(poleRadicand, zeroRadicand) {
		return biquad.Identity(), ErrInvalidPeakParams
	}

	a := math.Sqrt(poleRadicand)
	b := math.Sqrt(zeroRadicand)

	coefficients, ok := normalizeBiquadOK(
		nyquistGain+centerSquared+b,
		-2*(nyquistGain-centerSquared),
		nyquistGain+centerSquared-b,
		1+centerSquared+a,
		-2*(1-centerSquared),
		1+centerSquared-a,
	)
	if !ok || !polesInsideUnitCircle(coefficients) {
		return biquad.Identity(), ErrInvalidPeakParams
	}

	return coefficients, nil
}

// nearMultipleOfPi reports whether w lies within rounding distance of a
// multiple of pi, where the octave-bandwidth design is singular. The tolerance
// covers the error of forming w from frequency/sampleRate, so folded centers
// such as 5.5 times the sample rate are rejected even though their computed
// remainder is not exactly zero.
func nearMultipleOfPi(w float64) bool {
	const tolerance = 16 * 0x1p-52

	return math.Abs(math.Remainder(w, math.Pi)) <= tolerance*math.Max(1, math.Abs(w))
}

// polesInsideUnitCircle reports whether both poles of the normalized section
// lie strictly inside the unit circle (the stability triangle).
func polesInsideUnitCircle(c biquad.Coefficients) bool {
	return math.Abs(c.A2) < 1 && math.Abs(c.A1) < 1+c.A2
}
