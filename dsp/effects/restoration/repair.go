package restoration

import (
	"fmt"
	"math"
)

// RepairGap replaces a bounded corrupt interval using an autoregressive model
// fitted to intact samples on both sides, then solves the unknown residuals
// jointly (Janssen-style interpolation). It changes only [start,end). Short or
// singular contexts fall back to cubic Hermite interpolation. Gaps are limited
// to 256 samples; longer damage must be treated explicitly by the caller.
func RepairGap(samples []float64, start, end int) error {
	if start < 2 || end <= start || end > len(samples)-2 || end-start > 256 {
		return fmt.Errorf("restoration.repair: gap needs two-sided context and <=256 samples")
	}

	for _, x := range samples {
		if !finite(x) {
			return fmt.Errorf("restoration.repair: nonfinite samples")
		}
	}

	lo, hi := max(0, start-256), min(len(samples), end+256)

	order := min(16, (start-lo)/4, (hi-end)/4)
	if order < 2 {
		hermite(samples, start, end)
		return nil
	}

	gram, rhs := make([]float64, order*order), make([]float64, order)
	count := 0

	for t := lo + order; t < hi; t++ {
		if t >= start && t-order < end {
			continue
		}

		count++

		for i := range order {
			xi := samples[t-i-1]

			rhs[i] += xi * samples[t]
			for j := range order {
				gram[i*order+j] += xi * samples[t-j-1]
			}
		}
	}

	if count < order {
		hermite(samples, start, end)
		return nil
	}

	scale := 0.0
	for i := range order {
		scale += gram[i*order+i]
	}

	for i := range order {
		gram[i*order+i] += math.Max(scale*1e-10, 1e-20)
	}

	coefficients, ok := solve(gram, rhs, order)
	if !ok {
		hermite(samples, start, end)
		return nil
	}

	gap := end - start
	system, target := make([]float64, gap*gap), make([]float64, gap)
	terms := make([]float64, order+1)

	terms[0] = 1
	for i, c := range coefficients {
		terms[i+1] = -c
	}

	for t := start; t < min(hi, end+order); t++ {
		known := 0.0

		for j, c := range terms {
			pos := t - j
			if pos < start || pos >= end {
				known += c * samples[pos]
			}
		}

		for j, c := range terms {
			pos := t - j
			if pos < start || pos >= end {
				continue
			}

			row := pos - start
			target[row] -= c * known

			for k, d := range terms {
				q := t - k
				if q >= start && q < end {
					system[row*gap+q-start] += c * d
				}
			}
		}
	}

	restored, ok := solve(system, target, gap)
	if !ok {
		hermite(samples, start, end)
		return nil
	}

	peak := 0.0
	for _, x := range samples[lo:start] {
		peak = math.Max(peak, math.Abs(x))
	}

	for _, x := range samples[end:hi] {
		peak = math.Max(peak, math.Abs(x))
	}

	for _, x := range restored {
		if !finite(x) || math.Abs(x) > math.Max(4*peak, 1e-8) {
			hermite(samples, start, end)
			return nil
		}
	}

	copy(samples[start:end], restored)

	return nil
}

func hermite(x []float64, start, end int) {
	a, b := x[start-1], x[end]
	span := float64(end - start + 1)

	ma, mb := (a-x[start-2])*span, (x[end+1]-b)*span
	for i := start; i < end; i++ {
		t := float64(i-start+1) / span
		t2, t3 := t*t, t*t*t
		x[i] = (2*t3-3*t2+1)*a + (t3-2*t2+t)*ma + (-2*t3+3*t2)*b + (t3-t2)*mb
	}
}

func solve(a, b []float64, n int) ([]float64, bool) {
	for k := range n {
		pivot := k
		for i := k + 1; i < n; i++ {
			if math.Abs(a[i*n+k]) > math.Abs(a[pivot*n+k]) {
				pivot = i
			}
		}

		if math.Abs(a[pivot*n+k]) < 1e-24 {
			return nil, false
		}

		if pivot != k {
			for j := k; j < n; j++ {
				a[k*n+j], a[pivot*n+j] = a[pivot*n+j], a[k*n+j]
			}

			b[k], b[pivot] = b[pivot], b[k]
		}

		for i := k + 1; i < n; i++ {
			factor := a[i*n+k] / a[k*n+k]
			for j := k; j < n; j++ {
				a[i*n+j] -= factor * a[k*n+j]
			}

			b[i] -= factor * b[k]
		}
	}

	for i := n - 1; i >= 0; i-- {
		for j := i + 1; j < n; j++ {
			b[i] -= a[i*n+j] * b[j]
		}

		b[i] /= a[i*n+i]
	}

	return b, true
}

// RepairClicks detects short second-difference outliers relative to the local
// median. sensitivity is in [3,30]; at most maxGap (1..256) samples are repaired
// per event. Two intact samples on both sides are required.
func RepairClicks(samples []float64, sensitivity float64, maxGap int) error {
	if !finite(sensitivity) || sensitivity < 3 || sensitivity > 30 || maxGap < 1 || maxGap > 256 {
		return fmt.Errorf("restoration.clicks: invalid settings")
	}

	for _, x := range samples {
		if !finite(x) {
			return fmt.Errorf("restoration.clicks: nonfinite input")
		}
	}

	marks := make([]bool, len(samples))

	scratch := make([]float64, 0, 31)
	for i := 18; i < len(samples)-18; i++ {
		scratch = scratch[:0]

		for j := i - 16; j <= i+16; j++ {
			if absInt(j-i) > 2 {
				scratch = append(scratch, math.Abs(samples[j-1]-2*samples[j]+samples[j+1]))
			}
		}
		// Bounded insertion sort avoids a callback allocation.
		for j := 1; j < len(scratch); j++ {
			for k := j; k > 0 && scratch[k] < scratch[k-1]; k-- {
				scratch[k], scratch[k-1] = scratch[k-1], scratch[k]
			}
		}

		residual := math.Abs(samples[i-1] - 2*samples[i] + samples[i+1])
		marks[i] = residual > math.Max(1e-5, sensitivity*scratch[len(scratch)/2])
	}

	for i := 2; i < len(samples)-2; {
		if !marks[i] {
			i++
			continue
		}

		start := i
		for i < len(samples)-2 && marks[i] {
			i++
		}

		end := i
		if end-start <= maxGap {
			if err := RepairGap(samples, start, end); err != nil {
				return err
			}
		}
	}

	return nil
}

// Declip interpolates interior saturation runs at threshold [0.1,1]. Intact
// context is retained bit-for-bit. Runs longer than maxGap are left untouched.
func Declip(samples []float64, threshold float64, maxGap int) error {
	if !finite(threshold) || threshold < 0.1 || threshold > 1 || maxGap < 1 || maxGap > 256 {
		return fmt.Errorf("restoration.declip: invalid settings")
	}

	for _, x := range samples {
		if !finite(x) {
			return fmt.Errorf("restoration.declip: nonfinite input")
		}
	}

	for i := 0; i < len(samples); {
		if math.Abs(samples[i]) < threshold {
			i++
			continue
		}

		start := i
		for i < len(samples) && math.Abs(samples[i]) >= threshold {
			i++
		}

		if start >= 2 && i <= len(samples)-2 && i-start <= maxGap {
			if err := RepairGap(samples, start, i); err != nil {
				return err
			}
		}
	}

	return nil
}
