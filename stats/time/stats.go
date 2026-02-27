package time

import "math"

// Stats holds time-domain signal statistics.
//
//nolint:revive
type Stats struct {
	Length         int
	DC             float64 // mean
	DC_dB          float64
	RMS            float64
	RMS_dB         float64
	Max            float64
	MaxPos         int
	Min            float64
	MinPos         int
	Peak           float64 // max(|max|, |min|)
	Peak_dB        float64
	Range          float64 // max - min
	Range_dB       float64
	CrestFactor    float64 // peak / RMS (linear)
	CrestFactor_dB float64
	Energy         float64 // sum of squares
	Power          float64 // energy / length
	ZeroCrossings  int
	Variance       float64
	Skewness       float64
	Kurtosis       float64
}

// ampTodB converts an amplitude value to decibels: 20 * log10(|value|).
// Returns -Inf for zero values.
func ampTodB(value float64) float64 {
	a := math.Abs(value)
	if a == 0 {
		return math.Inf(-1)
	}

	return 20 * math.Log10(a)
}

// ratioTodB converts a linear ratio to decibels: 20 * log10(value).
// Returns -Inf for zero values.
func ratioTodB(value float64) float64 {
	if value == 0 {
		return math.Inf(-1)
	}

	return 20 * math.Log10(value)
}

// emptyStats returns a zero-valued Stats with -Inf for all dB fields.
func emptyStats() Stats {
	return Stats{
		DC_dB:          math.Inf(-1),
		RMS_dB:         math.Inf(-1),
		Peak_dB:        math.Inf(-1),
		Range_dB:       math.Inf(-1),
		CrestFactor_dB: math.Inf(-1),
	}
}

// Calculate computes all time-domain statistics in a single pass using
// Welford's online algorithm for numerical stability on higher-order moments.
func Calculate(signal []float64) Stats {
	n := len(signal)
	if n == 0 {
		return emptyStats()
	}

	// Welford accumulators.
	var (
		mean float64
		m2   float64
		m3   float64
		m4   float64
	)

	// Running aggregates.
	var (
		sumSq         float64
		maxVal        = signal[0]
		maxPos        int
		minVal        = signal[0]
		minPos        int
		zeroCrossings int
	)

	for i, x := range signal {
		// --- Welford update for moments ---
		ni := float64(i + 1) // 1-based count after this sample
		delta := x - mean
		deltaN := delta / ni
		deltaN2 := deltaN * deltaN
		term1 := delta * deltaN * float64(i) // delta * delta_n * (n-1)

		// M4 must be updated before M3, and M3 before M2.
		m4 += term1*deltaN2*(ni*ni-3*ni+3) + 6*deltaN2*m2 - 4*deltaN*m3
		m3 += term1*deltaN*(float64(i)-1) - 3*deltaN*m2
		m2 += term1
		mean += deltaN

		// --- Energy (sum of squares) ---
		sumSq += x * x

		// --- Min / Max ---
		if x > maxVal {
			maxVal = x
			maxPos = i
		}

		if x < minVal {
			minVal = x
			minPos = i
		}

		// --- Zero crossings ---
		if i > 0 && signal[i-1]*x < 0 {
			zeroCrossings++
		}
	}

	nf := float64(n)
	rms := math.Sqrt(sumSq / nf)
	peak := math.Max(math.Abs(maxVal), math.Abs(minVal))
	rangeVal := maxVal - minVal

	var crest, crestdB float64
	if rms == 0 {
		crest = 0
		crestdB = 0
	} else {
		crest = peak / rms
		crestdB = ratioTodB(crest)
	}

	variance := m2 / nf

	var skewness, kurtosis float64
	if variance > 0 {
		skewness = (m3 / nf) / (variance * math.Sqrt(variance))
		kurtosis = (m4/nf)/(variance*variance) - 3
	}

	return Stats{
		Length:         n,
		DC:             mean,
		DC_dB:          ampTodB(mean),
		RMS:            rms,
		RMS_dB:         ampTodB(rms),
		Max:            maxVal,
		MaxPos:         maxPos,
		Min:            minVal,
		MinPos:         minPos,
		Peak:           peak,
		Peak_dB:        ampTodB(peak),
		Range:          rangeVal,
		Range_dB:       ampTodB(rangeVal),
		CrestFactor:    crest,
		CrestFactor_dB: crestdB,
		Energy:         sumSq,
		Power:          sumSq / nf,
		ZeroCrossings:  zeroCrossings,
		Variance:       variance,
		Skewness:       skewness,
		Kurtosis:       kurtosis,
	}
}

// RMS returns the root-mean-square of the signal.
func RMS(signal []float64) float64 {
	if len(signal) == 0 {
		return 0
	}

	var sumSq float64
	for _, x := range signal {
		sumSq += x * x
	}

	return math.Sqrt(sumSq / float64(len(signal)))
}

// DC returns the mean (DC offset) of the signal.
func DC(signal []float64) float64 {
	if len(signal) == 0 {
		return 0
	}
	// Use Kahan summation for numerical stability.
	var sum, c float64
	for _, x := range signal {
		y := x - c
		t := sum + y
		c = (t - sum) - y
		sum = t
	}

	return sum / float64(len(signal))
}

// Peak returns the peak absolute amplitude of the signal.
func Peak(signal []float64) float64 {
	if len(signal) == 0 {
		return 0
	}

	peak := math.Abs(signal[0])
	for _, x := range signal[1:] {
		a := math.Abs(x)
		if a > peak {
			peak = a
		}
	}

	return peak
}

// CrestFactor returns the crest factor (peak / RMS) of the signal.
// Returns 0 if RMS is zero.
func CrestFactor(signal []float64) float64 {
	r := RMS(signal)
	if r == 0 {
		return 0
	}

	return Peak(signal) / r
}

// ZeroCrossings returns the number of zero crossings in the signal.
// A crossing is counted when consecutive samples have opposite signs.
func ZeroCrossings(signal []float64) int {
	if len(signal) < 2 {
		return 0
	}

	var count int

	for i := 1; i < len(signal); i++ {
		if signal[i-1]*signal[i] < 0 {
			count++
		}
	}

	return count
}

// Moments returns the mean, population variance, skewness, and excess kurtosis
// of the signal using Welford's online algorithm for numerical stability.
func Moments(signal []float64) (mean, variance, skewness, kurtosis float64) {
	n := len(signal)
	if n == 0 {
		return 0, 0, 0, 0
	}

	var m2, m3, m4 float64

	for i, x := range signal {
		ni := float64(i + 1)
		delta := x - mean
		deltaN := delta / ni
		deltaN2 := deltaN * deltaN
		term1 := delta * deltaN * float64(i)

		m4 += term1*deltaN2*(ni*ni-3*ni+3) + 6*deltaN2*m2 - 4*deltaN*m3
		m3 += term1*deltaN*(float64(i)-1) - 3*deltaN*m2
		m2 += term1
		mean += deltaN
	}

	nf := float64(n)

	variance = m2 / nf
	if variance > 0 {
		skewness = (m3 / nf) / (variance * math.Sqrt(variance))
		kurtosis = (m4/nf)/(variance*variance) - 3
	}

	return mean, variance, skewness, kurtosis
}

// StreamingStats accumulates time-domain statistics incrementally across
// multiple blocks of samples. It processes each sample individually to
// guarantee bit-for-bit identical results with [Calculate].
type StreamingStats struct {
	n             int
	mean          float64
	m2            float64
	m3            float64
	m4            float64
	sumSq         float64
	maxVal        float64
	maxPos        int
	minVal        float64
	minPos        int
	zeroCrossings int
	hasData       bool
	lastSample    float64
}

// NewStreamingStats creates a new StreamingStats accumulator.
func NewStreamingStats() *StreamingStats {
	return &StreamingStats{}
}

// Update adds a block of samples to the running statistics.
func (s *StreamingStats) Update(samples []float64) {
	for _, x := range samples {
		s.n++
		ni := float64(s.n)

		// Welford update.
		delta := x - s.mean
		deltaN := delta / ni
		deltaN2 := deltaN * deltaN
		term1 := delta * deltaN * float64(s.n-1)

		s.m4 += term1*deltaN2*(ni*ni-3*ni+3) + 6*deltaN2*s.m2 - 4*deltaN*s.m3
		s.m3 += term1*deltaN*(float64(s.n-1)-1) - 3*deltaN*s.m2
		s.m2 += term1
		s.mean += deltaN

		// Energy.
		s.sumSq += x * x

		// Min / Max.
		if !s.hasData {
			s.maxVal = x
			s.maxPos = s.n - 1
			s.minVal = x
			s.minPos = s.n - 1
			s.hasData = true
		} else {
			if x > s.maxVal {
				s.maxVal = x
				s.maxPos = s.n - 1
			}

			if x < s.minVal {
				s.minVal = x
				s.minPos = s.n - 1
			}
		}

		// Zero crossings: check against previous sample.
		if s.n > 1 && s.lastSample*x < 0 {
			s.zeroCrossings++
		}

		s.lastSample = x
	}
}

// Result computes the final statistics from accumulated data.
func (s *StreamingStats) Result() Stats {
	if s.n == 0 {
		return emptyStats()
	}

	nf := float64(s.n)
	rms := math.Sqrt(s.sumSq / nf)
	peak := math.Max(math.Abs(s.maxVal), math.Abs(s.minVal))
	rangeVal := s.maxVal - s.minVal

	var crest, crestdB float64
	if rms == 0 {
		crest = 0
		crestdB = 0
	} else {
		crest = peak / rms
		crestdB = ratioTodB(crest)
	}

	variance := s.m2 / nf

	var skewness, kurtosis float64
	if variance > 0 {
		skewness = (s.m3 / nf) / (variance * math.Sqrt(variance))
		kurtosis = (s.m4/nf)/(variance*variance) - 3
	}

	return Stats{
		Length:         s.n,
		DC:             s.mean,
		DC_dB:          ampTodB(s.mean),
		RMS:            rms,
		RMS_dB:         ampTodB(rms),
		Max:            s.maxVal,
		MaxPos:         s.maxPos,
		Min:            s.minVal,
		MinPos:         s.minPos,
		Peak:           peak,
		Peak_dB:        ampTodB(peak),
		Range:          rangeVal,
		Range_dB:       ampTodB(rangeVal),
		CrestFactor:    crest,
		CrestFactor_dB: crestdB,
		Energy:         s.sumSq,
		Power:          s.sumSq / nf,
		ZeroCrossings:  s.zeroCrossings,
		Variance:       variance,
		Skewness:       skewness,
		Kurtosis:       kurtosis,
	}
}

// Reset clears all accumulated data, allowing the StreamingStats to be reused.
func (s *StreamingStats) Reset() {
	*s = StreamingStats{}
}
