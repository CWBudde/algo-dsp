package structure

// This file holds a verbatim copy of AudioVisualizer's
// internal/story/structure.go (ZScore, assemble, SelfSimilarity,
// FooteNovelty, Boundaries, Label, letterName), cosine from
// internal/story/harmony.go and r3/r6 from internal/story/grid.go, adapted
// to standalone test code: the Grid is replaced by beat-start and
// bar-position functions, the audioanalysis.Cue by a name and a start time,
// identifiers carry a ref prefix, and blank lines were added for the linter.
// The arithmetic and its operation order are unchanged; the parity tests
// compare this copy with the package bit for bit.

import (
	"math"
	"strings"
)

type refCue struct {
	Name  string
	Start float64
}

type refBoundary struct {
	Time        float64
	Beat        int
	BarPosition float64
	Novelty     float64
	NearestCue  string
	CueDelta    float64
}

type refBlock struct {
	values [][]float64 // [unit][dim]
	weight float64
}

func refR6(v float64) float64 { return math.Round(v*1e6) / 1e6 }
func refR3(v float64) float64 { return math.Round(v*1e3) / 1e3 }

func refZScore(rows [][]float64) {
	if len(rows) == 0 {
		return
	}

	for d := range rows[0] {
		mean, sq := 0.0, 0.0
		for _, r := range rows {
			mean += r[d] / float64(len(rows))
		}

		for _, r := range rows {
			sq += (r[d] - mean) * (r[d] - mean)
		}

		sd := math.Sqrt(sq / float64(len(rows)))
		for _, r := range rows {
			if sd > 1e-12 {
				r[d] = (r[d] - mean) / sd
			} else {
				r[d] = 0
			}
		}
	}
}

func refAssemble(blocks []refBlock) [][]float64 {
	if len(blocks) == 0 {
		return nil
	}

	out := make([][]float64, len(blocks[0].values))
	for _, b := range blocks {
		refZScore(b.values)

		for u, v := range b.values {
			norm := 0.0
			for _, x := range v {
				norm += x * x
			}

			norm = math.Sqrt(norm)
			for _, x := range v {
				if norm > 0 {
					x *= b.weight / norm
				}

				out[u] = append(out[u], x)
			}
		}
	}

	return out
}

func refCosine(a, b []float64) float64 {
	dot, na, nb := 0.0, 0.0, 0.0
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}

	if na == 0 || nb == 0 {
		return 0
	}

	return dot / math.Sqrt(na*nb)
}

func refSelfSimilarity(rows [][]float64) [][]float64 {
	s := make([][]float64, len(rows))
	for i := range rows {
		s[i] = make([]float64, len(rows))
		for j := range rows {
			s[i][j] = refCosine(rows[i], rows[j])
		}
	}

	return s
}

func refFooteNovelty(s [][]float64, halfWidth int) []float64 {
	n := len(s)
	out := make([]float64, n)
	sigma := float64(halfWidth) / 2
	top := 0.0

	for t := range n {
		v := 0.0

		for a := -halfWidth; a < halfWidth; a++ {
			for b := -halfWidth; b < halfWidth; b++ {
				i, j := t+a, t+b
				if i < 0 || j < 0 || i >= n || j >= n {
					continue
				}

				sign := 1.0
				if (a < 0) != (b < 0) {
					sign = -1
				}

				x, y := float64(a)+0.5, float64(b)+0.5
				v += sign * math.Exp(-(x*x+y*y)/(2*sigma*sigma)) * s[i][j]
			}
		}

		out[t] = math.Max(0, v)
		top = math.Max(top, out[t])
	}

	for t := range out {
		if top > 0 {
			out[t] /= top
		}
	}

	return out
}

func refBoundaries(novelty []float64, beatStart func(int) float64, barPosition func(float64) float64, cues []refCue, radius int, sigma float64) []refBoundary {
	mean, sq := 0.0, 0.0
	for _, v := range novelty {
		mean += v / float64(len(novelty))
	}

	for _, v := range novelty {
		sq += (v - mean) * (v - mean) / float64(len(novelty))
	}

	threshold := mean + sigma*math.Sqrt(sq)
	out := []refBoundary{}

	for t, v := range novelty {
		if v <= threshold {
			continue
		}

		peak := true

		for k := max(0, t-radius); k <= min(len(novelty)-1, t+radius); k++ {
			if novelty[k] > v || (novelty[k] == v && k < t) {
				peak = false
			}
		}

		if !peak {
			continue
		}

		time := beatStart(t)
		b := refBoundary{Time: refR6(time), Beat: t, BarPosition: refR3(barPosition(time)), Novelty: refR3(v)}
		best := math.Inf(1)

		for _, c := range cues {
			if d := time - c.Start; math.Abs(d) < math.Abs(best) {
				best, b.NearestCue = d, c.Name
			}
		}

		b.CueDelta = refR3(best)
		out = append(out, b)
	}

	return out
}

func refLabel(s [][]float64, size int, same, variant float64, lower bool) ([]string, []float64) {
	units := (len(s) + size - 1) / size
	sim := func(a, b int) float64 {
		v, n := 0.0, 0

		for k := range size {
			i, j := a*size+k, b*size+k
			if i < len(s) && j < len(s) {
				v += s[i][j]
				n++
			}
		}

		if n == 0 {
			return 0
		}

		return v / float64(n)
	}
	labels, first := make([]string, units), []int{}
	toFirst := make([]float64, units)
	base := func(l string) string { return strings.TrimRight(l, "'") }

	for u := range units {
		bestLetter, bestSim := -1, math.Inf(-1)

		for letter := range first {
			for v := range u {
				if base(labels[v]) == refLetterName(letter, lower) && sim(u, v) > bestSim {
					bestLetter, bestSim = letter, sim(u, v)
				}
			}
		}

		switch {
		case bestLetter >= 0 && bestSim >= same:
			labels[u] = refLetterName(bestLetter, lower)
		case bestLetter >= 0 && bestSim >= variant:
			labels[u] = refLetterName(bestLetter, lower) + "'"
		default:
			bestLetter = len(first)
			first = append(first, u)
			labels[u] = refLetterName(bestLetter, lower)
		}

		toFirst[u] = refR3(sim(u, first[bestLetter]))
	}

	return labels, toFirst
}

func refLetterName(i int, lower bool) string {
	a := 'A'
	if lower {
		a = 'a'
	}

	if i < 26 {
		return string(a + rune(i))
	}

	return string(a+rune(i%26)) + string(rune('0'+i/26))
}
