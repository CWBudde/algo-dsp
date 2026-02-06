package window

import "math"

// Analysis holds numerically computed spectral properties of a window.
type Analysis struct {
	// CoherentGain is sum(w[n]) / N, the DC response of the window.
	CoherentGain float64
	// ENBW is the equivalent noise bandwidth in bins.
	ENBW float64
	// Bandwidth3dB is the 3 dB (half-power) main lobe width in bins.
	Bandwidth3dB float64
	// HighestSidelobedB is the highest sidelobe level relative to DC in dB.
	HighestSidelobedB float64
	// FirstMinimumBins is the first null (minimum) position in bins.
	FirstMinimumBins float64
	// ScallopLossdB is the worst-case amplitude error for an off-bin signal.
	ScallopLossdB float64
}

// Analyze computes spectral properties of the given window coefficients
// using numerical DFT evaluation, matching the approach from the legacy
// MFW Window Test utility.
func Analyze(coeffs []float64) Analysis {
	n := len(coeffs)
	if n == 0 {
		return Analysis{}
	}

	// DC reference: |DFT(0)|^2
	dcRef := dftMagSq(coeffs, 0)
	if dcRef == 0 {
		return Analysis{}
	}

	// Coherent gain and ENBW.
	sum := 0.0
	sumSq := 0.0
	for _, c := range coeffs {
		sum += c
		sumSq += c * c
	}
	coherentGain := sum / float64(n)
	enbw := float64(n) * sumSq / (sum * sum)

	// Scallop loss: evaluate at 0.5 bins offset.
	halfBinFreq := 0.5 / float64(n)
	halfBinMagSq := dftMagSq(coeffs, halfBinFreq)
	scallopLoss := 0.0
	if dcRef > 0 && halfBinMagSq > 0 {
		scallopLoss = 10 * math.Log10(halfBinMagSq/dcRef)
	}

	// Numerical search for 3dB bandwidth (half-power point).
	bw3dB := searchBandwidth(coeffs, dcRef, n)

	// Numerical search for first minimum via coarse scan then refinement.
	firstMin := searchFirstMinimum(coeffs, n)

	// Numerical search for highest sidelobe past the first minimum.
	sidelobe := searchHighestSidelobe(coeffs, dcRef, firstMin, n)

	return Analysis{
		CoherentGain:      coherentGain,
		ENBW:              enbw,
		Bandwidth3dB:      bw3dB,
		HighestSidelobedB: sidelobe,
		FirstMinimumBins:  firstMin,
		ScallopLossdB:     scallopLoss,
	}
}

// dftMagSq evaluates |DFT(freq)|^2 at a normalised frequency [0,1).
func dftMagSq(coeffs []float64, freq float64) float64 {
	re, im := 0.0, 0.0
	w := 2 * math.Pi * freq
	for k, c := range coeffs {
		phase := w * float64(k)
		re += c * math.Cos(phase)
		im -= c * math.Sin(phase)
	}
	return re*re + im*im
}

// searchBandwidth finds the 3dB (half-power) main lobe width in bins
// using binary search on the DFT magnitude response.
func searchBandwidth(coeffs []float64, dcRef float64, n int) float64 {
	nf := float64(n)
	invRef := 1.0 / dcRef

	// The -3dB point is where |H(f)|^2/|H(0)|^2 = 0.5.
	// Use bisection on [0, Nyquist] in normalised frequency.
	lo := 0.0
	hi := 0.5
	for i := 0; i < 80; i++ {
		mid := (lo + hi) / 2
		val := dftMagSq(coeffs, mid) * invRef
		if val > 0.5 {
			lo = mid
		} else {
			hi = mid
		}
	}
	// Bandwidth is two-sided: from -f to +f.
	return 2 * lo * nf
}

// searchFirstMinimum finds the first spectral null position in bins
// by scanning from DC outward for the first local minimum.
func searchFirstMinimum(coeffs []float64, n int) float64 {
	nf := float64(n)
	// Coarse scan: step = 1/8 of a bin in normalised frequency.
	step := 1.0 / (nf * 8)

	dcVal := dftMagSq(coeffs, 0)
	prev := dcVal
	coarseMinFreq := step
	// Require descent to at least 10% of DC before looking for a turn-around,
	// to avoid false positives in flat-top windows where the main lobe
	// has a wide plateau.
	threshold := dcVal * 0.1

	for freq := step; freq < 0.5; freq += step {
		val := dftMagSq(coeffs, freq)
		if prev < threshold && val > prev {
			// We passed a local minimum at the previous sample.
			coarseMinFreq = freq - step
			break
		}
		prev = val
	}

	// Refine with golden-section search around the coarse minimum.
	a := coarseMinFreq - 2*step
	b := coarseMinFreq + 2*step
	if a < 0 {
		a = 0
	}
	if b > 0.5 {
		b = 0.5
	}

	const phi = 0.6180339887498949 // (sqrt(5)-1)/2
	c := b - phi*(b-a)
	d := a + phi*(b-a)
	for i := 0; i < 80; i++ {
		fc := dftMagSq(coeffs, c)
		fd := dftMagSq(coeffs, d)
		if fc < fd {
			b = d
		} else {
			a = c
		}
		c = b - phi*(b-a)
		d = a + phi*(b-a)
	}
	minFreq := (a + b) / 2
	// First minimum position in bins (one-sided, from DC).
	return minFreq * nf
}

// searchHighestSidelobe finds the peak sidelobe level in dB relative to DC.
func searchHighestSidelobe(coeffs []float64, dcRef, firstMinBins float64, n int) float64 {
	nf := float64(n)
	// Start scanning from the first minimum (convert bins to normalised freq).
	startFreq := firstMinBins / nf
	step := 1.0 / (nf * 8)

	peakVal := 0.0
	peakFreq := startFreq

	// Coarse scan to Nyquist.
	for freq := startFreq; freq < 0.5; freq += step {
		val := dftMagSq(coeffs, freq)
		if val > peakVal {
			peakVal = val
			peakFreq = freq
		}
	}

	// Refine around peak with finer step.
	fineStep := step / 32
	refinedPeak := peakVal
	for freq := peakFreq - step; freq <= peakFreq+step; freq += fineStep {
		if freq < 0 {
			continue
		}
		val := dftMagSq(coeffs, freq)
		if val > refinedPeak {
			refinedPeak = val
		}
	}

	if refinedPeak <= 0 || dcRef <= 0 {
		return -math.Inf(1)
	}
	return 10 * math.Log10(refinedPeak/dcRef)
}
