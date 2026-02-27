package resample

import (
	"errors"
	"fmt"
	"math"
)

func designPolyphaseFIR(up, down int, cfg config) ([]float64, [][]float64, int, error) {
	if up <= 0 || down <= 0 {
		return nil, nil, 0, ErrInvalidRatio
	}

	if cfg.tapsPerPhase <= 0 {
		return nil, nil, 0, errors.New("resample: taps per phase must be > 0")
	}

	if cfg.cutoffScale <= 0 || cfg.cutoffScale > 1 {
		return nil, nil, 0, errors.New("resample: cutoff scale must be in (0,1]")
	}

	nTaps := cfg.tapsPerPhase * up

	fc := (0.5 / float64(maxInt(up, down))) * cfg.cutoffScale
	if fc <= 0 || fc >= 0.5 {
		return nil, nil, 0, fmt.Errorf("resample: invalid cutoff %.6f", fc)
	}

	taps := make([]float64, nTaps)

	center := 0.5 * float64(nTaps-1)
	for n := range nTaps {
		t := float64(n) - center
		h := 2 * fc * sinc(2*fc*t) * kaiserWindow(n, nTaps, cfg.kaiserBeta)
		taps[n] = h
	}

	var sum float64
	for _, v := range taps {
		sum += v
	}

	if sum == 0 {
		return nil, nil, 0, errors.New("resample: designed zero-sum filter")
	}

	scale := float64(up) / sum
	for i := range taps {
		taps[i] *= scale
	}

	phases := make([][]float64, up)
	maxPhaseLn := 0

	for p := range up {
		phase := make([]float64, 0, (nTaps-p+up-1)/up)
		for i := p; i < nTaps; i += up {
			phase = append(phase, taps[i])
		}

		if len(phase) > maxPhaseLn {
			maxPhaseLn = len(phase)
		}

		phases[p] = phase
	}

	return taps, phases, maxPhaseLn, nil
}

func approximateRatio(v float64, maxDen int) (num, den int) {
	if maxDen <= 0 {
		maxDen = 4096
	}

	if v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 1, 1
	}

	a0 := math.Floor(v)
	p0, q0 := 1.0, 0.0
	p1, q1 := a0, 1.0
	x := v

	for {
		frac := x - math.Floor(x)
		if frac == 0 {
			break
		}

		x = 1 / frac
		a := math.Floor(x)
		p2 := a*p1 + p0

		q2 := a*q1 + q0
		if q2 > float64(maxDen) {
			break
		}

		p0, q0 = p1, q1
		p1, q1 = p2, q2
	}

	num = int(math.Round(p1))

	den = int(math.Round(q1))
	if den <= 0 {
		return 1, 1
	}

	g := gcd(num, den)

	return num / g, den / g
}

func gcd(a, b int) int {
	if a < 0 {
		a = -a
	}

	if b < 0 {
		b = -b
	}

	for b != 0 {
		a, b = b, a%b
	}

	if a == 0 {
		return 1
	}

	return a
}

func sinc(x float64) float64 {
	if math.Abs(x) < 1e-12 {
		return 1
	}

	pix := math.Pi * x

	return math.Sin(pix) / pix
}

func kaiserWindow(i, n int, beta float64) float64 {
	if n <= 1 || beta == 0 {
		return 1
	}

	t := 2*float64(i)/float64(n-1) - 1
	a := math.Sqrt(math.Max(0, 1-t*t))

	return i0(beta*a) / i0(beta)
}

func i0(x float64) float64 {
	// Power series approximation.
	sum := 1.0
	term := 1.0

	x2 := (x * x) / 4
	for k := 1; k < 64; k++ {
		term *= x2 / float64(k*k)

		sum += term
		if term < 1e-16*sum {
			break
		}
	}

	return sum
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}

	return b
}
