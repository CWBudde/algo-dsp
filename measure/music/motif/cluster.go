package motif

import (
	"cmp"
	"slices"
)

// candidate is one grid window, n-gram or chroma window.
type candidate struct {
	first, length int   // unit range (slots, notes or beats) for the shift overlap
	start, end    int   // slot span, used for overlap
	notes         []int // note indices in sorted order; nil for chroma windows
}

func overlaps(a, b *candidate) bool { return a.start < b.end && b.start < a.end }

// member is a cluster member: a candidate with its similarity and
// transposition relative to the cluster centre.
type member struct {
	index         int
	similarity    float64
	transposition int
}

// group is an accepted cluster: its centre and its non-overlapping
// occurrences in time order.
type group struct {
	centre int
	occs   []member
}

// clusterer groups candidates greedily. sim is the similarity of b relative
// to a, transpose the transposition of b relative to a; both are
// deterministic, so the neighbour similarities are cached.
type clusterer struct {
	cands        []candidate
	sim          func(a, b int) float64
	transpose    func(a, b int) int
	minSim       float64
	minOcc       int
	shiftOverlap float64

	// Neighbours in compressed rows: neighbour b of a is nbr[off[a]+k] with
	// similarity nbrSim[off[a]+k]. nbrTrans, if set, holds the
	// transpositions; else transpose computes them.
	off      []int
	nbr      []int
	nbrSim   []float64
	nbrTrans []int
}

// neighbours fills the neighbour rows from sim: every other candidate that
// does not overlap and has at least the minimum similarity, in index order.
func (c *clusterer) neighbours() {
	cands := c.cands
	c.off = make([]int, len(cands)+1)

	for a := range cands {
		for b := range cands {
			if a != b && !overlaps(&cands[a], &cands[b]) {
				if s := c.sim(a, b); s >= c.minSim {
					c.nbr = append(c.nbr, b)
					c.nbrSim = append(c.nbrSim, s)
				}
			}
		}

		c.off[a+1] = len(c.nbr)
	}
}

// run greedily groups the candidates around the one with the largest summed
// similarity to its available neighbours. Accepted occurrences do not
// overlap; candidates sharing at least shiftOverlap of their units with an
// accepted occurrence become unavailable, which keeps rotations of a
// repeating figure out of later clusters.
func (c *clusterer) run() []group {
	cands := c.cands
	n := len(cands)

	if c.off == nil {
		c.neighbours()
	}

	off, nbr, nbrSim := c.off, c.nbr, c.nbrSim

	available := make([]bool, n)
	for i := range available {
		available[i] = true
	}

	out := []group{}

	var members, accepted []member

	for {
		centre, count, mass := -1, 0, -1.0

		for a := range cands {
			if !available[a] {
				continue
			}

			k, sum := 0, 0.0

			for j := off[a]; j < off[a+1]; j++ {
				if available[nbr[j]] {
					k, sum = k+1, sum+nbrSim[j]
				}
			}

			if sum > mass {
				centre, count, mass = a, k, sum
			}
		}

		if centre < 0 || count+1 < c.minOcc {
			return out
		}

		members = append(members[:0], member{centre, 1, 0})

		for j := off[centre]; j < off[centre+1]; j++ {
			if b := nbr[j]; available[b] {
				var t int
				if c.nbrTrans != nil {
					t = c.nbrTrans[j]
				} else {
					t = c.transpose(centre, b)
				}

				members = append(members, member{b, nbrSim[j], t})
			}
		}

		slices.SortStableFunc(members, func(x, y member) int { return cmp.Compare(y.similarity, x.similarity) })

		accepted = accepted[:0]

		for _, m := range members {
			free := true
			for _, a := range accepted {
				free = free && !overlaps(&cands[a.index], &cands[m.index])
			}

			if free {
				accepted = append(accepted, m)
			}
		}

		for _, m := range members {
			available[m.index] = false
		}

		c.withdrawShifts(available, accepted)

		if len(accepted) >= c.minOcc {
			occs := slices.Clone(accepted)
			slices.SortStableFunc(occs, func(x, y member) int { return cmp.Compare(cands[x.index].first, cands[y.index].first) })
			out = append(out, group{centre, occs})
		}
	}
}

// withdrawShifts marks unavailable every candidate that shares at least
// shiftOverlap of its units with an accepted occurrence.
func (c *clusterer) withdrawShifts(available []bool, accepted []member) {
	for i := range c.cands {
		ci := &c.cands[i]

		for _, a := range accepted {
			ca := &c.cands[a.index]
			lo, hi := max(ci.first, ca.first), min(ci.first+ci.length, ca.first+ca.length)

			if float64(hi-lo) >= c.shiftOverlap*float64(ci.length) {
				available[i] = false
			}
		}
	}
}
