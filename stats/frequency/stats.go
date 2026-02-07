package frequency

import (
	"math"
	"math/cmplx"
)

// Stats holds frequency-domain statistics computed from a magnitude spectrum.
type Stats struct {
	BinCount   int
	DC         float64 // bin 0 magnitude
	DC_dB      float64
	Sum        float64 // sum of magnitudes
	Sum_dB     float64
	Max        float64
	MaxBin     int
	Min        float64
	MinBin     int
	Average    float64
	Average_dB float64
	Range      float64
	Range_dB   float64
	Energy     float64 // sum of squared magnitudes
	Power      float64
	// Spectral shape descriptors
	Centroid  float64 // spectral centroid (Hz)
	Spread    float64 // spectral spread (Hz)
	Flatness  float64 // spectral flatness (Wiener entropy), 0..1
	Rolloff   float64 // frequency below which 85% energy (Hz)
	Bandwidth float64 // 3 dB bandwidth around peak (Hz)
}

// toDB converts a linear magnitude to decibels.
// Returns -Inf for zero values.
func toDB(v float64) float64 {
	if v <= 0 {
		return math.Inf(-1)
	}
	return 20 * math.Log10(v)
}

// binFreq returns the frequency in Hz of a given bin index.
// fftSize = 2 * (len(magnitude) - 1).
func binFreq(i int, sampleRate float64, binCount int) float64 {
	return float64(i) * sampleRate / float64(2*(binCount-1))
}

// Calculate computes all frequency-domain statistics from a magnitude spectrum
// (linear scale, NOT dB).
//
// The magnitude slice represents bins from 0 (DC) to Nyquist (one-sided
// spectrum, length = FFTSize/2 + 1). The frequency of bin i is:
//
//	f_i = i * sampleRate / (2 * (len(magnitude) - 1))
func Calculate(magnitude []float64, sampleRate float64) Stats {
	n := len(magnitude)
	if n == 0 {
		return Stats{
			DC_dB:      math.Inf(-1),
			Sum_dB:     math.Inf(-1),
			Average_dB: math.Inf(-1),
			Range_dB:   math.Inf(-1),
		}
	}
	if n == 1 {
		// DC-only spectrum (single bin).
		v := magnitude[0]
		return Stats{
			BinCount:   1,
			DC:         v,
			DC_dB:      toDB(v),
			Sum:        v,
			Sum_dB:     toDB(v),
			Max:        v,
			MaxBin:     0,
			Min:        v,
			MinBin:     0,
			Average:    v,
			Average_dB: toDB(v),
			Range:      0,
			Range_dB:   toDB(0),
			Energy:     v * v,
			Power:      v * v,
		}
	}

	var s Stats
	s.BinCount = n
	s.DC = magnitude[0]
	s.DC_dB = toDB(s.DC)

	// First pass: basic statistics.
	s.Min = magnitude[0]
	s.Max = magnitude[0]
	for i, v := range magnitude {
		s.Sum += v
		s.Energy += v * v
		if v > s.Max {
			s.Max = v
			s.MaxBin = i
		}
		if v < s.Min {
			s.Min = v
			s.MinBin = i
		}
	}
	s.Sum_dB = toDB(s.Sum)
	s.Average = s.Sum / float64(n)
	s.Average_dB = toDB(s.Average)
	s.Range = s.Max - s.Min
	s.Range_dB = toDB(s.Range)
	s.Power = s.Energy / float64(n)

	// Spectral shape descriptors.
	s.Centroid = centroid(magnitude, sampleRate, s.Sum)
	s.Spread = spread(magnitude, sampleRate, s.Centroid, s.Sum)
	s.Flatness = flatness(magnitude)
	s.Rolloff = rolloff(magnitude, sampleRate, 0.85, s.Energy)
	s.Bandwidth = bandwidth(magnitude, sampleRate)

	return s
}

// CalculateFromComplex converts a complex spectrum to magnitude (absolute value)
// and delegates to [Calculate].
func CalculateFromComplex(spectrum []complex128, sampleRate float64) Stats {
	mag := make([]float64, len(spectrum))
	for i, c := range spectrum {
		mag[i] = cmplx.Abs(c)
	}
	return Calculate(mag, sampleRate)
}

// Centroid returns the spectral centroid in Hz.
//
//	centroid = sum(f_i * |X_i|) / sum(|X_i|)
func Centroid(magnitude []float64, sampleRate float64) float64 {
	if len(magnitude) < 2 {
		return 0
	}
	sum := 0.0
	for _, v := range magnitude {
		sum += v
	}
	return centroid(magnitude, sampleRate, sum)
}

func centroid(magnitude []float64, sampleRate float64, sumMag float64) float64 {
	n := len(magnitude)
	if n < 2 || sumMag == 0 {
		return 0
	}
	weightedSum := 0.0
	for i, v := range magnitude {
		weightedSum += binFreq(i, sampleRate, n) * v
	}
	return weightedSum / sumMag
}

// spread computes spectral spread (standard deviation of the spectrum around the centroid).
func spread(magnitude []float64, sampleRate float64, cent float64, sumMag float64) float64 {
	n := len(magnitude)
	if n < 2 || sumMag == 0 {
		return 0
	}
	weightedSqSum := 0.0
	for i, v := range magnitude {
		diff := binFreq(i, sampleRate, n) - cent
		weightedSqSum += diff * diff * v
	}
	return math.Sqrt(weightedSqSum / sumMag)
}

// Flatness returns the spectral flatness (Wiener entropy) in the range 0..1.
//
// Flatness = exp(mean(log(|X_i|))) / mean(|X_i|)
//
// DC bin (index 0) is excluded from the computation. If all considered bins
// are zero, 0 is returned.
func Flatness(magnitude []float64) float64 {
	return flatness(magnitude)
}

func flatness(magnitude []float64) float64 {
	n := len(magnitude)
	if n < 2 {
		return 0
	}

	// Operate on bins 1..N-1 (skip DC bin 0).
	nBins := n - 1
	sumLin := 0.0
	sumLog := 0.0
	hasZero := false

	for i := 1; i < n; i++ {
		v := magnitude[i]
		sumLin += v
		if v > 0 {
			sumLog += math.Log(v)
		} else {
			hasZero = true
		}
	}

	meanLin := sumLin / float64(nBins)
	if meanLin == 0 {
		return 0
	}

	// If any bin is zero the geometric mean is zero, so flatness is zero.
	if hasZero {
		return 0
	}

	meanLog := sumLog / float64(nBins)
	geoMean := math.Exp(meanLog)

	return geoMean / meanLin
}

// Rolloff returns the frequency below which the specified fraction (0..1) of
// spectral energy lies.
//
// Energy is defined as the sum of squared magnitudes. A typical value for
// percent is 0.85.
func Rolloff(magnitude []float64, sampleRate float64, percent float64) float64 {
	if len(magnitude) < 2 {
		return 0
	}
	energy := 0.0
	for _, v := range magnitude {
		energy += v * v
	}
	return rolloff(magnitude, sampleRate, percent, energy)
}

func rolloff(magnitude []float64, sampleRate float64, percent float64, totalEnergy float64) float64 {
	n := len(magnitude)
	if n < 2 || totalEnergy == 0 {
		return 0
	}
	threshold := percent * totalEnergy
	cumEnergy := 0.0
	for i, v := range magnitude {
		cumEnergy += v * v
		if cumEnergy >= threshold {
			return binFreq(i, sampleRate, n)
		}
	}
	return binFreq(n-1, sampleRate, n)
}

// Bandwidth returns the 3 dB bandwidth around the spectral peak in Hz.
//
// The peak bin is found, and then the -3 dB points (where magnitude drops to
// peak/sqrt(2)) are located on both sides. Linear interpolation between bins
// is used for more precise estimation.
func Bandwidth(magnitude []float64, sampleRate float64) float64 {
	return bandwidth(magnitude, sampleRate)
}

func bandwidth(magnitude []float64, sampleRate float64) float64 {
	n := len(magnitude)
	if n < 2 {
		return 0
	}

	// Find peak.
	peakBin := 0
	peakVal := magnitude[0]
	for i, v := range magnitude {
		if v > peakVal {
			peakVal = v
			peakBin = i
		}
	}
	if peakVal == 0 {
		return 0
	}

	threshold := peakVal / math.Sqrt2

	// Find lower -3 dB point (search left from peak).
	lowerFreq := binFreq(0, sampleRate, n)
	for i := peakBin; i >= 1; i-- {
		if magnitude[i-1] <= threshold && magnitude[i] > threshold {
			// Interpolate between bins i-1 and i.
			lowerFreq = interpFreq(i-1, i, magnitude[i-1], magnitude[i], threshold, sampleRate, n)
			break
		}
	}

	// Find upper -3 dB point (search right from peak).
	upperFreq := binFreq(n-1, sampleRate, n)
	for i := peakBin; i < n-1; i++ {
		if magnitude[i+1] <= threshold && magnitude[i] > threshold {
			// Interpolate between bins i and i+1.
			upperFreq = interpFreq(i, i+1, magnitude[i], magnitude[i+1], threshold, sampleRate, n)
			break
		}
	}

	bw := upperFreq - lowerFreq
	if bw < 0 {
		return 0
	}
	return bw
}

// interpFreq linearly interpolates between two bins to find the frequency
// where the magnitude crosses the given threshold.
func interpFreq(binLow, binHigh int, magLow, magHigh, threshold, sampleRate float64, binCount int) float64 {
	fLow := binFreq(binLow, sampleRate, binCount)
	fHigh := binFreq(binHigh, sampleRate, binCount)

	denom := magHigh - magLow
	if denom == 0 {
		return (fLow + fHigh) / 2
	}
	t := (threshold - magLow) / denom
	return fLow + t*(fHigh-fLow)
}
