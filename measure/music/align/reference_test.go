//nolint:all // Verbatim reference copy; kept byte-for-byte close to the original.
package align_test

// Verbatim copy (adapted to a standalone function) of CheckAlignment from
// AudioVisualizer/internal/audioanalysis/rhythm.go. The duration check on
// the source WAV metadata and the four-stem requirement stay in the app.

import "math"

const SampleRate = 24000

func DB(v float64) float64 { return 20 * math.Log10(math.Max(v, 1e-6)) }

type Alignment struct {
	Correlation   float64
	ResidualRMSDB float64
	BestLagMS     float64
}

func refCheckAlignment(mix [][]float64, stems [][][]float64) *Alignment {
	n := len(mix[0])
	sum := make([]float64, n)
	reference := make([]float64, n)
	result := &Alignment{}
	for _, s := range stems {
		for _, ch := range s {
			for i := 0; i < min(n, len(ch)); i++ {
				sum[i] += ch[i] / float64(len(s))
			}
		}
	}
	for _, ch := range mix {
		for i, v := range ch {
			reference[i] += v / float64(len(mix))
		}
	}
	best := -1.0
	residual := 0.0
	for lag := -48; lag <= 48; lag++ {
		dot, aa, bb := 0.0, 0.0, 0.0
		for i := max(0, -lag); i < min(n, n-lag); i += 4 {
			a, b := reference[i], sum[i+lag]
			dot += a * b
			aa += a * a
			bb += b * b
		}
		corr := 0.0
		if aa*bb > 0 {
			corr = dot / math.Sqrt(aa*bb)
		}
		if lag == 0 {
			result.Correlation = corr
		}
		if corr > best {
			best = corr
			result.BestLagMS = float64(lag) / SampleRate * 1000
		}
	}
	for i, v := range reference {
		d := v - sum[i]
		residual += d * d
	}
	result.ResidualRMSDB = DB(math.Sqrt(residual / float64(n)))
	return result
}
