//nolint:all // Verbatim reference copy; kept byte-for-byte close to the original.
package align_test

// Verbatim copy (adapted to standalone functions) of stats and compare from
// AudioVisualizer/cmd/verifyrender/main.go. The *audioanalysis.Audio
// arguments are replaced by plain channel slices, the window struct by
// refLagWindow, and identifiers carry a ref prefix. The arithmetic and its
// operation order are unchanged; TestLagParity compares Lag with this copy
// bit for bit. Do not "fix" or reformat the arithmetic below.

import "math"

type refLagWindow struct {
	Start       float64
	End         float64
	LagMS       float64
	Correlation float64
	GainDB      float64
}

func refStats(a, b [][]float64, start, end, lag int) (float64, float64) {
	var ab, aa, bb float64
	for i := start; i < end; i += 16 {
		j := i + lag
		if j < 0 || j >= len(b[0]) {
			continue
		}
		for ch := range a {
			x, y := a[ch][i], b[ch][j]
			ab += x * y
			aa += x * x
			bb += y * y
		}
	}
	return ab / math.Sqrt(aa*bb), 10 * math.Log10(bb/aa)
}

// refCompare works on loaded channels, which Load resamples to the analysis
// rate, not the file's original Source.SampleRate.
func refCompare(a, b [][]float64, start, end float64) refLagWindow {
	rate := float64(SampleRate)
	i, j := int(start*rate), min(int(end*rate), len(a[0]))
	best, lag := -1.0, 0
	for offset := -480; offset <= 480; offset += 16 {
		c, _ := refStats(a, b, i, j, offset)
		if c > best {
			best, lag = c, offset
		}
	}
	coarse := lag
	for offset := coarse - 16; offset <= coarse+16; offset++ {
		c, _ := refStats(a, b, i, j, offset)
		if c > best {
			best, lag = c, offset
		}
	}
	c, gain := refStats(a, b, i, j, lag)
	return refLagWindow{start, end, float64(lag) * 1000 / rate, c, gain}
}
