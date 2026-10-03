package motif

import (
	"cmp"
	"fmt"
	"math"
	"slices"

	"github.com/cwbudde/algo-dsp/measure/music/rhythm"
)

// FindChromaMotifs finds recurring harmonic material in beat-level chroma:
// the fallback for material without reliable notes. beats[i] is the 12-bin
// chroma (C, C♯, …, B) of grid beat i, from [rhythm.Grid.BeatStart] of i to
// i+1; an all-zero vector marks a silent or gated beat. The values must be
// finite; their scale does not matter.
//
// For each window width in beats ([WithChromaWindows], longest first),
// windows start at every beat, unless half or more of their beats are silent
// or [WithCoveredShare] of their beats are covered by longer chroma motifs.
// Two windows are compared under every rotation of the first window's
// chroma by the mean per-beat cosine similarity; the best rotation is the
// optimal transposition index and gives the transposition (folded into
// [-5, 6] semitones before re-referencing). Windows match at
// [WithChromaMinSimilarity] and are clustered like the windows of
// [FindNoteMotifs] ([WithMinOccurrences], [WithShiftOverlap]).
//
// Each motif has [SourceChroma] as its source, the window width as
// SpanBeats, and as Notes the strongest pitch class of each beat of the
// prototype at the first occurrence's level. An occurrence is
// [VariantExact] or [VariantTransposed] when its similarity is at least
// 0.999, else varied. Occurrence slots are the beats' slots (beat × the
// grid's subdivisions) and NoteIndices is nil.
//
// The defaults reproduce AudioVisualizer's FindChromaMotifs bit for bit.
//
// Options read: [WithChromaWindows], [WithChromaMinSimilarity],
// [WithMinOccurrences], [WithShiftOverlap] and [WithCoveredShare].
func FindChromaMotifs(beats [][12]float64, grid rhythm.Grid, opts ...Option) ([]Motif, error) {
	cfg, err := newConfig(opts)
	if err != nil {
		return nil, err
	}

	err = validGrid(grid)
	if err != nil {
		return nil, err
	}

	for i, c := range beats {
		for pc, v := range c {
			if !finite(v) {
				return nil, fmt.Errorf("%w: chroma of beat %d, class %d is %v", ErrInvalidArgument, i, pc, v)
			}
		}
	}

	subdivisions := grid.Subdivisions()
	coveredBeats := make([]bool, len(beats))
	motifs := []Motif{}

	for _, width := range cfg.chromaWindows {
		cands := []candidate{}

		for i := 0; i+width <= len(beats); i++ {
			c, silent := 0, 0

			for k := i; k < i+width; k++ {
				if coveredBeats[k] {
					c++
				}

				if beats[k] == ([12]float64{}) {
					silent++
				}
			}

			if float64(c) < cfg.coveredShare*float64(width) && silent*2 < width {
				cands = append(cands, candidate{first: i, length: width, start: subdivisions * i, end: subdivisions * (i + width)})
			}
		}

		cl := clusterer{
			cands:        cands,
			minSim:       cfg.chromaMinSimilarity,
			minOcc:       cfg.minOccurrences,
			shiftOverlap: cfg.shiftOverlap,
		}
		chromaNeighbours(&cl, beats, width)

		for _, gr := range cl.run() {
			m := Motif{Source: SourceChroma, SpanBeats: float64(width)}
			proto := &cands[gr.centre]
			t0 := gr.occs[0].transposition

			for k := range width {
				m.Notes = append(m.Notes, pitchClassName(argmax(beats[proto.first+k])+t0+12))
			}

			for _, o := range gr.occs {
				c := &cands[o.index]
				start, t := grid.BeatStart(c.first), fold(o.transposition-t0)
				m.Occurrences = append(m.Occurrences, Occurrence{
					Start: r6(start), End: r6(grid.BeatStart(c.first + width)), Bar: grid.Bar(start + 1e-6), Slot: c.start,
					Transposition: t, Similarity: r3(o.similarity), Variant: variant(o.similarity >= 0.999, t),
				})

				for k := c.first; k < c.first+width; k++ {
					coveredBeats[k] = true
				}
			}

			motifs = append(motifs, m)
		}
	}

	return motifs, nil
}

// chromaNeighbours fills the neighbour rows of the chroma windows of width
// beats. The similarity of window b to window a is the best mean per-beat
// cosine of b against a rotated by t semitones, over t in 0..11, and the
// transposition is that t folded into [-5, 6].
//
// Windows a and b at a beat distance d share the per-beat cosines of the
// diagonal d, so these are computed once per diagonal and rotation and then
// summed per window in beat order, the same arithmetic as comparing the
// windows directly.
func chromaNeighbours(cl *clusterer, beats [][12]float64, width int) {
	cands, n := cl.cands, len(beats)

	// byFirst maps a first beat to its candidate, or -1.
	byFirst := make([]int, n)
	for i := range byFirst {
		byFirst[i] = -1
	}

	for a, c := range cands {
		byFirst[c.first] = a
	}

	rotated := make([][12][12]float64, n)

	for i := range beats {
		for t := range 12 {
			rotated[i][t] = rotate(beats[i], t)
		}
	}

	type neighbour struct {
		b, t int
		sim  float64
	}

	rows := make([][]neighbour, len(cands))
	cosines := make([][]float64, 12)

	for t := range cosines {
		cosines[t] = make([]float64, n)
	}

	for d := -(n - 1); d < n; d++ {
		if d > -width && d < width {
			continue // overlapping windows
		}

		lo, hi := max(0, -d), min(n, n-d) // beats i with i+d in range
		for t := range 12 {
			for i := lo; i < hi; i++ {
				cosines[t][i] = cosine(&rotated[i][t], &beats[i+d])
			}
		}

		for a, c := range cands {
			bf := c.first + d
			if bf < 0 || bf >= n || byFirst[bf] < 0 {
				continue
			}

			best, bestT := math.Inf(-1), 0

			for t := range 12 {
				v := 0.0
				for k := range width {
					v += cosines[t][c.first+k] / float64(width)
				}

				if v > best+1e-12 {
					best, bestT = v, t
				}
			}

			if bestT > 6 {
				bestT -= 12
			}

			if best >= cl.minSim {
				rows[a] = append(rows[a], neighbour{byFirst[bf], bestT, best})
			}
		}
	}

	// Rows were filled by distance; the clusterer expects index order, which
	// is the order of the first beats.
	cl.off = make([]int, len(cands)+1)

	for a, row := range rows {
		slices.SortFunc(row, func(x, y neighbour) int { return cmp.Compare(x.b, y.b) })

		for _, nb := range row {
			cl.nbr = append(cl.nbr, nb.b)
			cl.nbrSim = append(cl.nbrSim, nb.sim)
			cl.nbrTrans = append(cl.nbrTrans, nb.t)
		}

		cl.off[a+1] = len(cl.nbr)
	}

	if cl.nbrTrans == nil {
		cl.nbrTrans = []int{}
	}
}

func rotate(c [12]float64, t int) [12]float64 {
	var out [12]float64
	for pc := range 12 {
		out[(pc+t+12)%12] = c[pc]
	}

	return out
}

func cosine(a, b *[12]float64) float64 {
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

func argmax(c [12]float64) int {
	best := 0

	for i, v := range c {
		if v > c[best] {
			best = i
		}
	}

	return best
}
