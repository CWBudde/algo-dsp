package design

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design/pass"
)

const defaultQ = 1 / math.Sqrt2

// BilinearTransform converts an analog second-order polynomial
// c0*s^2 + c1*s + c2 into the digital z^-1-domain polynomial
// d0 + d1*z^-1 + d2*z^-2 using the bilinear transform.
//
// The returned coefficients are normalized such that d0 = 1.
func BilinearTransform(sCoeffs [3]float64, sampleRate float64) [3]float64 {
	if sampleRate <= 0 {
		return [3]float64{1, 0, 0}
	}

	k := 2 * sampleRate
	c0, c1, c2 := sCoeffs[0], sCoeffs[1], sCoeffs[2]

	d0 := c0*k*k + c1*k + c2
	d1 := -2*c0*k*k + 2*c2
	d2 := c0*k*k - c1*k + c2

	if d0 == 0 || math.IsNaN(d0) || math.IsInf(d0, 0) {
		return [3]float64{1, 0, 0}
	}

	return [3]float64{1, d1 / d0, d2 / d0}
}

// Lowpass designs a lowpass biquad at freq (Hz) with quality factor q.
func Lowpass(freq, q, sampleRate float64) biquad.Coefficients {
	return pass.LowpassRBJ(freq, q, sampleRate)
}

// Highpass designs a highpass biquad at freq (Hz) with quality factor q.
func Highpass(freq, q, sampleRate float64) biquad.Coefficients {
	return pass.HighpassRBJ(freq, q, sampleRate)
}

// Bandpass designs a constant-skirt-gain bandpass biquad.
func Bandpass(freq, q, sampleRate float64) biquad.Coefficients {
	w0, ok := normalizedW0(freq, sampleRate)
	if !ok {
		return biquad.Coefficients{}
	}

	q = normalizedQ(q)
	cw := math.Cos(w0)
	sw := math.Sin(w0)
	alpha := sw / (2 * q)

	b0 := sw / 2
	b1 := 0.0
	b2 := -sw / 2
	a0 := 1 + alpha
	a1 := -2 * cw
	a2 := 1 - alpha

	return normalizeBiquad(b0, b1, b2, a0, a1, a2)
}

// Notch designs a notch biquad centered at freq (Hz).
func Notch(freq, q, sampleRate float64) biquad.Coefficients {
	w0, ok := normalizedW0(freq, sampleRate)
	if !ok {
		return biquad.Coefficients{}
	}

	q = normalizedQ(q)
	cw := math.Cos(w0)
	sw := math.Sin(w0)
	alpha := sw / (2 * q)

	b0 := 1.0
	b1 := -2 * cw
	b2 := 1.0
	a0 := 1 + alpha
	a1 := -2 * cw
	a2 := 1 - alpha

	return normalizeBiquad(b0, b1, b2, a0, a1, a2)
}

// Allpass designs an allpass biquad centered at freq (Hz).
func Allpass(freq, q, sampleRate float64) biquad.Coefficients {
	w0, ok := normalizedW0(freq, sampleRate)
	if !ok {
		return biquad.Coefficients{}
	}

	q = normalizedQ(q)
	cosW := math.Cos(w0)
	sinW := math.Sin(w0)
	alpha := sinW / (2 * q)

	b0 := 1 - alpha
	b1 := -2 * cosW
	b2 := 1 + alpha
	a0 := 1 + alpha
	a1 := -2 * cosW
	a2 := 1 - alpha

	return normalizeBiquad(b0, b1, b2, a0, a1, a2)
}

// Peak designs a peaking-EQ biquad with gain in dB.
//
// Without options, it uses the standard RBJ formula. Supplying WithDCGain
// and/or WithNyquistGain activates the Orfanidis algorithm which supports
// prescribed gain at DC and Nyquist. If the Orfanidis constraints cannot be
// met, it silently falls back to the RBJ formula.
func Peak(freq, gainDB, q, sampleRate float64, opts ...PeakOption) biquad.Coefficients {
	return peakWithOpts(freq, gainDB, q, sampleRate, opts)
}

func peakRBJ(freq, gainDB, q, sampleRate float64) biquad.Coefficients {
	w0, ok := normalizedW0(freq, sampleRate)
	if !ok {
		return biquad.Coefficients{}
	}

	q = normalizedQ(q)
	cw := math.Cos(w0)
	sw := math.Sin(w0)
	alpha := sw / (2 * q)
	a := math.Pow(10, gainDB/40)

	b0 := 1 + alpha*a
	b1 := -2 * cw
	b2 := 1 - alpha*a
	a0 := 1 + alpha/a
	a1 := -2 * cw
	a2 := 1 - alpha/a

	return normalizeBiquad(b0, b1, b2, a0, a1, a2)
}

// LowShelf designs a low-shelf biquad with gain in dB.
func LowShelf(freq, gainDB, q, sampleRate float64) biquad.Coefficients {
	w0, ok := normalizedW0(freq, sampleRate)
	if !ok {
		return biquad.Coefficients{}
	}

	q = normalizedQ(q)
	cw := math.Cos(w0)
	sw := math.Sin(w0)
	alpha := sw / (2 * q)
	a := math.Pow(10, gainDB/40)
	beta := 2 * math.Sqrt(a) * alpha

	b0 := a * ((a + 1) - (a-1)*cw + beta)
	b1 := 2 * a * ((a - 1) - (a+1)*cw)
	b2 := a * ((a + 1) - (a-1)*cw - beta)
	a0 := (a + 1) + (a-1)*cw + beta
	a1 := -2 * ((a - 1) + (a+1)*cw)
	a2 := (a + 1) + (a-1)*cw - beta

	return normalizeBiquad(b0, b1, b2, a0, a1, a2)
}

// HighShelf designs a high-shelf biquad with gain in dB.
func HighShelf(freq, gainDB, q, sampleRate float64) biquad.Coefficients {
	w0, ok := normalizedW0(freq, sampleRate)
	if !ok {
		return biquad.Coefficients{}
	}

	q = normalizedQ(q)
	cw := math.Cos(w0)
	sw := math.Sin(w0)
	alpha := sw / (2 * q)
	a := math.Pow(10, gainDB/40)
	beta := 2 * math.Sqrt(a) * alpha

	b0 := a * ((a + 1) + (a-1)*cw + beta)
	b1 := -2 * a * ((a - 1) + (a+1)*cw)
	b2 := a * ((a + 1) + (a-1)*cw - beta)
	a0 := (a + 1) - (a-1)*cw + beta
	a1 := 2 * ((a - 1) - (a+1)*cw)
	a2 := (a + 1) - (a-1)*cw - beta

	return normalizeBiquad(b0, b1, b2, a0, a1, a2)
}

func normalizedW0(freq, sampleRate float64) (float64, bool) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return 0, false
	}

	nyquist := sampleRate / 2
	if freq <= 0 || freq >= nyquist || math.IsNaN(freq) || math.IsInf(freq, 0) {
		return 0, false
	}

	return 2 * math.Pi * freq / sampleRate, true
}

func normalizedQ(q float64) float64 {
	if q <= 0 || math.IsNaN(q) || math.IsInf(q, 0) {
		return defaultQ
	}

	return q
}

func normalizeBiquad(b0, b1, b2, a0, a1, a2 float64) biquad.Coefficients {
	if a0 == 0 || math.IsNaN(a0) || math.IsInf(a0, 0) {
		return biquad.Coefficients{}
	}

	return biquad.Coefficients{
		B0: b0 / a0,
		B1: b1 / a0,
		B2: b2 / a0,
		A1: a1 / a0,
		A2: a2 / a0,
	}
}
