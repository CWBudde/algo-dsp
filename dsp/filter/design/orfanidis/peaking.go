package orfanidis

import (
	"errors"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
)

var ErrInvalidParams = errors.New("orfanidis: invalid parameters")

// Peaking designs an Orfanidis-style peaking EQ (prescribed Nyquist gain).
//
// Inputs are linear gains and digital rad/sample frequencies:
//
//	G0 = DC gain (linear)
//	G1 = Nyquist gain (linear)
//	G  = peak gain at center (linear)
//	GB = gain at band edges (linear) near w0 +- dw/2
//	w0 = center frequency (rad/sample)
//	dw = bandwidth (rad/sample)
//
// Returns biquad.Coefficients in the DF-II-T sign convention with a0 normalized to 1.
func Peaking(G0, G1, G, GB, w0, dw float64) (biquad.Coefficients, error) {
	if !(G0 > 0 && G1 > 0 && G > 0 && GB > 0) {
		return biquad.Coefficients{}, ErrInvalidParams
	}
	if !(w0 > 0 && w0 < math.Pi) {
		return biquad.Coefficients{}, ErrInvalidParams
	}
	if !(dw > 0 && dw < math.Pi) {
		return biquad.Coefficients{}, ErrInvalidParams
	}
	if hasInvalidFloat(G0, G1, G, GB, w0, dw) {
		return biquad.Coefficients{}, ErrInvalidParams
	}

	Omega0 := math.Tan(w0 / 2)
	if Omega0 == 0 || math.IsNaN(Omega0) || math.IsInf(Omega0, 0) {
		return biquad.Coefficients{}, ErrInvalidParams
	}

	gb2, g02, g12, g2 := GB*GB, G0*G0, G1*G1, G*G

	den1 := gb2 - g12
	den2 := g2 - g02
	num1 := gb2 - g02
	num2 := g2 - g12
	if den1 == 0 || den2 == 0 || num1 == 0 || num2 == 0 {
		return biquad.Coefficients{}, ErrInvalidParams
	}

	radicand := (num1 / den1) * (num2 / den2) * (Omega0 * Omega0)
	if radicand <= 0 || math.IsNaN(radicand) || math.IsInf(radicand, 0) {
		return biquad.Coefficients{}, ErrInvalidParams
	}
	DeltaOmega := (1 + math.Sqrt(radicand)) * math.Tan(dw/2)
	if DeltaOmega <= 0 || math.IsNaN(DeltaOmega) || math.IsInf(DeltaOmega, 0) {
		return biquad.Coefficients{}, ErrInvalidParams
	}

	W2 := (num2 / den2) * (Omega0 * Omega0)
	if W2 <= 0 || math.IsNaN(W2) || math.IsInf(W2, 0) {
		return biquad.Coefficients{}, ErrInvalidParams
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
		return biquad.Coefficients{}, ErrInvalidParams
	}

	A := math.Sqrt((C + D) / denAB)
	B := math.Sqrt((g2*C + gb2*D) / denAB)
	if math.IsNaN(A) || math.IsInf(A, 0) || math.IsNaN(B) || math.IsInf(B, 0) {
		return biquad.Coefficients{}, ErrInvalidParams
	}

	den := 1 + W2 + A
	if den == 0 || math.IsNaN(den) || math.IsInf(den, 0) {
		return biquad.Coefficients{}, ErrInvalidParams
	}

	b0 := (G1 + G0*W2 + B) / den
	b1 := -2 * (G1 - G0*W2) / den
	b2 := (G1 + G0*W2 - B) / den
	a1 := -2 * (1 - W2) / den
	a2 := (1 + W2 - A) / den

	if hasInvalidFloat(b0, b1, b2, a1, a2) {
		return biquad.Coefficients{}, ErrInvalidParams
	}

	return biquad.Coefficients{B0: b0, B1: b1, B2: b2, A1: a1, A2: a2}, nil
}

// PeakingFromFreqQGain is a convenience wrapper for audio-style parameters.
//
// - f0Hz in Hz, Q as usual (center/bandwidth), gainDB in dB
// - uses G0 = 1
// - sets GB = sqrt(G) (classic "half gain" band-edge convention)
// - maps Q -> dw using a common Orfanidis mapping
// - uses G1 = 1 (unity at Nyquist) as the default policy
func PeakingFromFreqQGain(sampleRate, f0Hz, Q, gainDB float64) (biquad.Coefficients, error) {
	if sampleRate <= 0 || f0Hz <= 0 || f0Hz >= sampleRate/2 || Q <= 0 {
		return biquad.Coefficients{}, ErrInvalidParams
	}
	if hasInvalidFloat(sampleRate, f0Hz, Q, gainDB) {
		return biquad.Coefficients{}, ErrInvalidParams
	}

	w0 := 2 * math.Pi * f0Hz / sampleRate
	// Many Orfanidis reference implementations (e.g. the historical peqio wrapper)
	// define these gains with an inverted dB mapping. The designed filter's
	// center gain ends up approximately 1/G.
	G := math.Pow(10, -gainDB/20.0)
	G0 := 1.0
	G1 := 1.0
	GB := math.Pow(10, -gainDB/40.0)

	dw := 2 * w0 * math.Sinh((math.Sin(w0)/w0)*math.Asinh(1/(2*Q)))
	if !(dw > 0 && dw < math.Pi) {
		return biquad.Coefficients{}, ErrInvalidParams
	}

	// The Orfanidis closed-form can be numerically/parametrically invalid for
	// some (f0, Q, gain) combinations (especially at high w0 and wide bands).
	// For audio use-cases we prefer a robust designer, so we fall back to the
	// existing RBJ-style peaking EQ if the Orfanidis constraints cannot be met
	// or if it does not realize the requested center gain.
	if c, err := Peaking(G0, G1, G, GB, w0, dw); err == nil {
		want := math.Pow(10, gainDB/20.0)
		gotSq := c.MagnitudeSquared(f0Hz, sampleRate)
		if gotSq > 0 && !math.IsNaN(gotSq) && !math.IsInf(gotSq, 0) {
			got := math.Sqrt(gotSq)
			if closeRel(got, want, 1e-2) {
				return c, nil
			}
		}
	}
	return design.Peak(f0Hz, gainDB, Q, sampleRate), nil
}

func closeRel(got, want, rel float64) bool {
	if want == 0 {
		return got == 0
	}
	d := math.Abs(got - want)
	return d <= rel*math.Abs(want)
}

// PeakingCascade builds an N-section cascade approximating a higher-order peaking EQ.
// Each section uses a reduced per-section gain so that total gain multiplies
// to the target gain.
func PeakingCascade(sampleRate, f0Hz, Q, gainDB float64, sections int) ([]biquad.Coefficients, error) {
	if sections <= 0 {
		return nil, ErrInvalidParams
	}
	if sections == 1 {
		c, err := PeakingFromFreqQGain(sampleRate, f0Hz, Q, gainDB)
		if err != nil {
			return nil, err
		}
		return []biquad.Coefficients{c}, nil
	}

	G := math.Pow(10, gainDB/20.0)
	Gs := math.Pow(G, 1.0/float64(sections))
	gainPerSectionDB := 20 * math.Log10(Gs)

	out := make([]biquad.Coefficients, sections)
	for i := 0; i < sections; i++ {
		c, err := PeakingFromFreqQGain(sampleRate, f0Hz, Q, gainPerSectionDB)
		if err != nil {
			return nil, err
		}
		out[i] = c
	}
	return out, nil
}

func hasInvalidFloat(vals ...float64) bool {
	for _, v := range vals {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return true
		}
	}
	return false
}
