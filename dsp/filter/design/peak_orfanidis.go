package design

import (
	"errors"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// ErrInvalidPeakParams is returned when Orfanidis peaking parameters are
// invalid (negative gains, out-of-range frequencies, etc.).
var ErrInvalidPeakParams = errors.New("design: invalid peaking parameters")

// PeakRaw designs an Orfanidis-style peaking EQ from low-level parameters.
//
// Inputs are linear gains and digital rad/sample frequencies:
//
//	G0 = DC gain (linear)
//	G1 = Nyquist gain (linear)
//	G  = peak gain at center (linear)
//	GB = gain at band edges (linear) near w0 ± dw/2
//	w0 = center frequency (rad/sample)
//	dw = bandwidth (rad/sample)
//
// Returns biquad.Coefficients in the DF-II-T sign convention with a0
// normalized to 1. On error the returned coefficients are [biquad.Identity],
// so a caller that ignores the error passes audio through unchanged rather
// than muting it.
func PeakRaw(G0, G1, G, GB, w0, dw float64) (biquad.Coefficients, error) {
	if !(G0 > 0 && G1 > 0 && G > 0 && GB > 0) {
		return biquad.Identity(), ErrInvalidPeakParams
	}

	if !(w0 > 0 && w0 < math.Pi) {
		return biquad.Identity(), ErrInvalidPeakParams
	}

	if !(dw > 0 && dw < math.Pi) {
		return biquad.Identity(), ErrInvalidPeakParams
	}

	if hasInvalidFloat(G0, G1, G, GB, w0, dw) {
		return biquad.Identity(), ErrInvalidPeakParams
	}

	Omega0 := math.Tan(w0 / 2)
	if Omega0 == 0 || math.IsNaN(Omega0) || math.IsInf(Omega0, 0) {
		return biquad.Identity(), ErrInvalidPeakParams
	}

	gb2, g02, g12, g2 := GB*GB, G0*G0, G1*G1, G*G

	den1 := gb2 - g12
	den2 := g2 - g02
	num1 := gb2 - g02

	num2 := g2 - g12
	if den1 == 0 || den2 == 0 || num1 == 0 || num2 == 0 {
		return biquad.Identity(), ErrInvalidPeakParams
	}

	radicand := (num1 / den1) * (num2 / den2) * (Omega0 * Omega0)
	if radicand <= 0 || math.IsNaN(radicand) || math.IsInf(radicand, 0) {
		return biquad.Identity(), ErrInvalidPeakParams
	}

	DeltaOmega := (1 + math.Sqrt(radicand)) * math.Tan(dw/2)
	if DeltaOmega <= 0 || math.IsNaN(DeltaOmega) || math.IsInf(DeltaOmega, 0) {
		return biquad.Identity(), ErrInvalidPeakParams
	}

	W2 := (num2 / den2) * (Omega0 * Omega0)
	if W2 <= 0 || math.IsNaN(W2) || math.IsInf(W2, 0) {
		return biquad.Identity(), ErrInvalidPeakParams
	}

	q := 1.0
	if G < 1 {
		q = -1.0
	}

	abs := math.Abs
	C := (DeltaOmega * DeltaOmega * abs(gb2-g12)) - 2*W2*(abs(gb2-G0*G1)-q*(gb2-g02)*(gb2-g12))
	D := 2 * W2 * (abs(g2-G0*G1) - q*(g2-g02)*(g2-g12))

	denAB := abs(g2 - gb2)
	if denAB == 0 || (C+D) <= 0 {
		return biquad.Identity(), ErrInvalidPeakParams
	}

	A := math.Sqrt((C + D) / denAB)

	B := math.Sqrt((g2*C + gb2*D) / denAB)
	if math.IsNaN(A) || math.IsInf(A, 0) || math.IsNaN(B) || math.IsInf(B, 0) {
		return biquad.Identity(), ErrInvalidPeakParams
	}

	den := 1 + W2 + A
	if den == 0 || math.IsNaN(den) || math.IsInf(den, 0) {
		return biquad.Identity(), ErrInvalidPeakParams
	}

	b0 := (G1 + G0*W2 + B) / den
	b1 := -2 * (G1 - G0*W2) / den
	b2 := (G1 + G0*W2 - B) / den
	a1 := -2 * (1 - W2) / den
	a2 := (1 + W2 - A) / den

	if hasInvalidFloat(b0, b1, b2, a1, a2) {
		return biquad.Identity(), ErrInvalidPeakParams
	}

	return biquad.Coefficients{B0: b0, B1: b1, B2: b2, A1: a1, A2: a2}, nil
}

// PeakCascade builds an N-section cascade approximating a higher-order peaking
// EQ. Each section uses a reduced per-section gain so that total gain
// multiplies to the target gain.
//
// When options (WithDCGain, WithNyquistGain, etc.) are supplied, each section
// uses the Orfanidis algorithm; otherwise the standard RBJ formula is used.
func PeakCascade(sampleRate, f0Hz, Q, gainDB float64, sections int, opts ...PeakOption) ([]biquad.Coefficients, error) {
	if sections <= 0 {
		return nil, ErrInvalidPeakParams
	}

	if sampleRate <= 0 || f0Hz <= 0 || f0Hz >= sampleRate/2 || Q <= 0 {
		return nil, ErrInvalidPeakParams
	}

	if hasInvalidFloat(sampleRate, f0Hz, Q, gainDB) {
		return nil, ErrInvalidPeakParams
	}

	G := math.Pow(10, gainDB/20.0)
	Gs := math.Pow(G, 1.0/float64(sections))
	gainPerSectionDB := 20 * math.Log10(Gs)

	out := make([]biquad.Coefficients, sections)
	for i := range out {
		c, ok := peakWithOpts(f0Hz, gainPerSectionDB, Q, sampleRate, opts)
		if !ok {
			return nil, ErrInvalidPeakParams
		}

		out[i] = c
	}

	return out, nil
}

// peakOrfanidisFromAudio maps audio-style parameters to the Orfanidis
// algorithm. The second result is false if the constraints cannot be met, in
// which case the coefficients are the pass-through section.
func peakOrfanidisFromAudio(freq, gainDB, q, sampleRate float64, cfg peakConfig) (biquad.Coefficients, bool) {
	w0 := 2 * math.Pi * freq / sampleRate

	G0 := 1.0
	if cfg.hasDCGain {
		G0 = cfg.dcGain
	}

	G1 := 1.0
	if cfg.hasNyqGain {
		G1 = cfg.nyquistGain
	}

	// Orfanidis uses inverted dB mapping for the peak gain.
	G := math.Pow(10, -gainDB/20.0)

	// GB is the gain at the band edges near w0 ± dw/2.
	GB := math.Pow(10, -gainDB/40.0) // default: half-gain
	if cfg.hasBEGain {
		GB = cfg.bandEdgeGain
	}

	dw := 2 * w0 * math.Sinh((math.Sin(w0)/w0)*math.Asinh(1/(2*q)))
	if !(dw > 0 && dw < math.Pi) {
		return biquad.Identity(), false
	}

	c, err := PeakRaw(G0, G1, G, GB, w0, dw)
	if err != nil {
		return biquad.Identity(), false
	}

	// Validate that the designed filter hits the requested center gain.
	want := math.Pow(10, gainDB/20.0)

	gotSq := c.MagnitudeSquared(freq, sampleRate)
	if gotSq > 0 && !math.IsNaN(gotSq) && !math.IsInf(gotSq, 0) {
		got := math.Sqrt(gotSq)
		if closeRel(got, want, 1e-2) {
			return c, true
		}
	}

	return biquad.Identity(), false
}

// peakWithOpts routes to either the Orfanidis or the RBJ algorithm based on
// whether options were supplied. If the Orfanidis path fails, it falls back
// to the RBJ formula.
//
// The second result reports whether a filter could be designed at all. It is
// false only when the RBJ fallback is also undesignable, in which case the
// coefficients are the pass-through section.
func peakWithOpts(freq, gainDB, q, sampleRate float64, opts []PeakOption) (biquad.Coefficients, bool) {
	cfg := applyPeakOpts(opts)
	useOrfanidis := cfg.hasDCGain || cfg.hasNyqGain || cfg.hasBEGain

	if useOrfanidis {
		if c, ok := peakOrfanidisFromAudio(freq, gainDB, q, sampleRate, cfg); ok {
			return c, true
		}
		// Fall back to RBJ.
	}

	return peakRBJ(freq, gainDB, q, sampleRate)
}

func closeRel(got, want, rel float64) bool {
	if want == 0 {
		return got == 0
	}

	d := math.Abs(got - want)

	return d <= rel*math.Abs(want)
}

func hasInvalidFloat(vals ...float64) bool {
	for _, v := range vals {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return true
		}
	}

	return false
}
