package motif

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"sort"

	"github.com/cwbudde/algo-dsp/measure/music/melody"
	"github.com/cwbudde/algo-dsp/measure/music/rhythm"
)

// maxGridWindows bounds the number of grid windows scanned per width, so a
// note far from the grid origin cannot make the scan run away.
const maxGridWindows = 1 << 22

// noteSearch holds the notes in time order and the state shared by the grid
// and n-gram passes.
type noteSearch struct {
	cfg     config
	grid    rhythm.Grid
	source  string
	notes   []melody.CleanNote // sorted
	orig    []int              // orig[k] is the caller's index of notes[k]
	seq     []int              // 0, 1, …, len(notes)-1
	covered []bool
	motifs  []Motif
}

// FindNoteMotifs finds recurring figures in a monophonic note line, such as
// the output of [melody.Clean], longest first. The figures are found
// regardless of transposition, and tolerate dropped notes and octave errors.
// source names the line in [Motif].Source; it must not be [SourceChroma].
//
// Two passes run in order, and each skips candidates whose notes are mostly
// covered by motifs found before ([WithCoveredShare]):
//
//  1. Grid windows ([WithGridWindows]: a bar and half a bar) start every half
//     window from the start of bar 0 and hold the notes whose slot falls in
//     them ([WithGridMinNotes] at least). Two windows match by the Dice
//     overlap of their notes paired by slot position within the window and
//     pitch class, under the transposition that pairs most notes (a
//     transposition other than whole octaves costs
//     [WithGridTranspositionPenalty]). The reported transposition is the
//     median semitone difference of the paired notes.
//  2. N-grams of consecutive notes ([WithLengths]) spanning at most
//     [WithMaxSpanSlots] with rests of at most [WithMaxGapSlots] match by
//     1 − distance, where the distance averages, over their successive
//     intervals, the interval difference (octave-tolerant, capped at 3
//     semitones, divided by 3) and the absolute log2 ratio of the
//     inter-onset intervals (capped at 1), weighted by
//     [WithDistanceWeights]. The transposition is the median semitone
//     difference of the paired notes.
//
// Each width or length clusters its candidates greedily: the candidate with
// the largest summed similarity to its available non-overlapping
// neighbours of at least [WithGridMinSimilarity] or [WithMinSimilarity]
// becomes a centre, its neighbours are accepted most similar first as long
// as they do not overlap, and the cluster is a motif if it has
// [WithMinOccurrences]. Candidates that share [WithShiftOverlap] of their
// units with an accepted occurrence are withdrawn, which suppresses the
// rotations of a repeating figure.
//
// The centre gives the motif's shape (intervals and inter-onset intervals);
// its pitches are reported at the first occurrence's level and register, and
// transpositions are relative to the first occurrence, folded into
// [-6, 5] semitones.
//
// The notes are processed in time order (by slot, then by the remaining
// fields), so the result does not depend on their order in the slice;
// [Occurrence].NoteIndices refer to the slice as passed. The search reads
// Slot, Slots, MIDI, Start and End; the other fields only order otherwise
// equal notes. Notes from [melody.Clean] (one per slot, in
// time order) reproduce AudioVisualizer's FindNoteMotifs bit for bit with
// the default options.
//
// Options read: [WithGridWindows], [WithGridMinNotes],
// [WithGridMinSimilarity], [WithGridTranspositionPenalty], [WithLengths],
// [WithMaxSpanSlots], [WithMaxGapSlots], [WithMinSimilarity],
// [WithMinOccurrences], [WithDistanceWeights], [WithShiftOverlap] and
// [WithCoveredShare].
func FindNoteMotifs(notes []melody.CleanNote, grid rhythm.Grid, source string, opts ...Option) ([]Motif, error) {
	cfg, err := newConfig(opts)
	if err != nil {
		return nil, err
	}

	err = validGrid(grid)
	if err != nil {
		return nil, err
	}

	if source == SourceChroma {
		return nil, fmt.Errorf("%w: source %q is reserved for chroma motifs", ErrInvalidArgument, source)
	}

	s := newNoteSearch(cfg, grid, source, notes)

	for _, width := range cfg.gridWindows {
		err = s.gridPass(width)
		if err != nil {
			return nil, err
		}
	}

	for _, length := range cfg.lengths {
		s.ngramPass(length)
	}

	return s.motifs, nil
}

func newNoteSearch(cfg config, grid rhythm.Grid, source string, notes []melody.CleanNote) *noteSearch {
	orig := make([]int, len(notes))
	for i := range orig {
		orig[i] = i
	}

	slices.SortStableFunc(orig, func(i, j int) int { return compareNotes(&notes[i], &notes[j]) })

	sorted := make([]melody.CleanNote, len(notes))
	for k, i := range orig {
		sorted[k] = notes[i]
	}

	seq := make([]int, len(notes))
	for i := range seq {
		seq[i] = i
	}

	return &noteSearch{
		cfg: cfg, grid: grid, source: source, notes: sorted, orig: orig, seq: seq,
		covered: make([]bool, len(notes)), motifs: []Motif{},
	}
}

// compareNotes orders notes by slot, then by every other field, so the
// processing order is independent of the input order.
func compareNotes(a, b *melody.CleanNote) int {
	return cmp.Or(
		cmp.Compare(a.Slot, b.Slot),
		cmp.Compare(a.Start, b.Start),
		cmp.Compare(a.End, b.End),
		cmp.Compare(a.Slots, b.Slots),
		cmp.Compare(a.MIDI, b.MIDI),
		cmp.Compare(a.Strength, b.Strength),
		cmp.Compare(a.RawMIDI, b.RawMIDI),
		cmp.Compare(a.OctaveShift, b.OctaveShift),
		cmp.Compare(a.Voice, b.Voice),
		cmp.Compare(a.OffsetMS, b.OffsetMS),
		compareBool(a.OffGrid, b.OffGrid),
		cmp.Compare(a.RawIndex, b.RawIndex),
	)
}

func compareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case b:
		return -1
	}

	return 1
}

func (s *noteSearch) coveredShare(idx []int) float64 {
	c := 0

	for _, k := range idx {
		if s.covered[k] {
			c++
		}
	}

	return float64(c) / float64(len(idx))
}

// shape returns the successive intervals and inter-onset intervals (at
// least 1 slot) of the notes idx.
func (s *noteSearch) shape(idx []int) (iv, ioi []int) {
	for k := 1; k < len(idx); k++ {
		iv = append(iv, s.notes[idx[k]].MIDI-s.notes[idx[k-1]].MIDI)
		ioi = append(ioi, max(1, s.notes[idx[k]].Slot-s.notes[idx[k-1]].Slot))
	}

	return iv, ioi
}

// gridPass searches the grid windows of width slots.
func (s *noteSearch) gridPass(width int) error {
	notes := s.notes
	cands := []candidate{}
	stride, origin := width/2, s.grid.Slot(s.grid.BarStart(0))

	if len(notes) > 0 {
		last := notes[len(notes)-1].Slot
		if last >= origin && (last-origin)/stride >= maxGridWindows {
			return fmt.Errorf("%w: notes span %d slots from the grid origin, more than %d windows", ErrInvalidArgument, last-origin, maxGridWindows)
		}
	}

	for start := origin; len(notes) > 0 && start <= notes[len(notes)-1].Slot; start += stride {
		c := candidate{first: start, length: width, start: start, end: start + width}
		lo := sort.Search(len(notes), func(k int) bool { return notes[k].Slot >= c.start })
		hi := sort.Search(len(notes), func(k int) bool { return notes[k].Slot >= c.end })
		c.notes = s.seq[lo:hi:hi]

		if len(c.notes) >= s.cfg.gridMinNotes && s.coveredShare(c.notes) < s.cfg.coveredShare {
			cands = append(cands, c)
		}
	}

	gs := gridSim{notes: notes, at: make([]int, width), has: make([]bool, width)}
	cl := clusterer{
		cands: cands,
		sim: func(a, b int) float64 {
			v, t := gs.similarity(&cands[a], &cands[b])
			if t != 0 {
				v -= s.cfg.gridTransposeCost
			}

			return v
		},
		transpose:    func(a, b int) int { return gs.transposition(&cands[a], &cands[b]) },
		minSim:       s.cfg.gridMinSimilarity,
		minOcc:       s.cfg.minOccurrences,
		shiftOverlap: s.cfg.shiftOverlap,
	}

	s.emit(cands, cl.run(), func(c *candidate) (float64, float64) {
		return s.grid.SlotTime(c.start), s.grid.SlotTime(c.end)
	})

	return nil
}

// ngramPass searches the n-grams of length notes.
func (s *noteSearch) ngramPass(length int) {
	notes, cfg := s.notes, s.cfg
	cands := []candidate{}
	iv, ioi := []int{}, []int{} // flat, length-1 values per candidate
	maxIOI := 1

	for i := 0; i+length <= len(notes); i++ {
		last := notes[i+length-1]
		ok := last.Slot+last.Slots-notes[i].Slot <= cfg.maxSpanSlots

		for k := i + 1; k < i+length; k++ {
			ok = ok && notes[k].Slot-(notes[k-1].Slot+notes[k-1].Slots) <= cfg.maxGapSlots
		}

		c := candidate{first: i, length: length, start: notes[i].Slot, end: last.Slot + last.Slots, notes: s.seq[i : i+length : i+length]}
		if !ok || s.coveredShare(c.notes) >= cfg.coveredShare {
			continue
		}

		for k := i + 1; k < i+length; k++ {
			d := max(1, notes[k].Slot-notes[k-1].Slot)
			iv, ioi = append(iv, notes[k].MIDI-notes[k-1].MIDI), append(ioi, d)
			maxIOI = max(maxIOI, d)
		}

		cands = append(cands, c)
	}

	dist := newNoteDistance(cfg, maxIOI)
	w := length - 1

	var diffs []int

	cl := clusterer{
		cands: cands,
		sim: func(a, b int) float64 {
			return dist.similarity(iv[a*w:(a+1)*w], iv[b*w:(b+1)*w], ioi[a*w:(a+1)*w], ioi[b*w:(b+1)*w], cfg.minSimilarity)
		},
		transpose: func(a, b int) int {
			diffs = diffs[:0]
			for k := range length {
				diffs = append(diffs, notes[cands[b].first+k].MIDI-notes[cands[a].first+k].MIDI)
			}

			slices.Sort(diffs)

			return diffs[(len(diffs)-1)/2]
		},
		minSim:       cfg.minSimilarity,
		minOcc:       cfg.minOccurrences,
		shiftOverlap: cfg.shiftOverlap,
	}

	s.emit(cands, cl.run(), func(c *candidate) (float64, float64) {
		return notes[c.notes[0]].Start, notes[c.notes[len(c.notes)-1]].End
	})
}

// emit turns the groups into motifs and marks their notes covered.
func (s *noteSearch) emit(cands []candidate, groups []group, span func(c *candidate) (float64, float64)) {
	notes := s.notes
	slotsPerBeat := float64(s.grid.Subdivisions())

	for _, gr := range groups {
		// The cluster centre defines the shape; transpositions are
		// re-referenced to the earliest occurrence so the opening statement
		// reads as 0.
		proto := &cands[gr.centre]
		t0 := gr.occs[0].transposition
		m := Motif{Source: s.source, SpanBeats: r3(float64(proto.end-proto.start) / slotsPerBeat)}
		m.Intervals, m.IOI = s.shape(proto.notes)

		// Keep the prototype's pitch classes at the first occurrence's level
		// and register (t0 may carry octave errors).
		first, mean := &cands[gr.occs[0].index], 0.0
		for _, k := range first.notes {
			mean += float64(notes[k].MIDI) / float64(len(first.notes))
		}

		for _, k := range proto.notes {
			mean -= float64(notes[k].MIDI+fold(t0)) / float64(len(proto.notes))
		}

		shift := fold(t0) + 12*int(math.Round(mean/12))
		for _, k := range proto.notes {
			m.PrototypeMIDI = append(m.PrototypeMIDI, notes[k].MIDI+shift)
			m.Notes = append(m.Notes, noteName(notes[k].MIDI+shift))
		}

		protoOffset := notes[proto.notes[0]].Slot - proto.start

		for _, o := range gr.occs {
			c := &cands[o.index]
			iv, ioi := s.shape(c.notes)
			exact := slices.Equal(iv, m.Intervals) && slices.Equal(ioi, m.IOI) && notes[c.notes[0]].Slot-c.start == protoOffset
			start, end := span(c)
			t := fold(o.transposition - t0)

			occ := Occurrence{
				Start: r6(start), End: r6(end), Bar: s.grid.Bar(s.grid.SlotTime(c.start)), Slot: c.start,
				Transposition: t, Similarity: r3(o.similarity), Variant: variant(exact, t),
				NoteIndices: make([]int, len(c.notes)),
			}

			for i, k := range c.notes {
				occ.NoteIndices[i] = s.orig[k]
				s.covered[k] = true
			}

			m.Occurrences = append(m.Occurrences, occ)
		}

		s.motifs = append(s.motifs, m)
	}
}

// gridSim compares grid windows. at and has are scratch buffers indexed by
// the slot offset within a window.
type gridSim struct {
	notes []melody.CleanNote
	at    []int
	has   []bool
}

// match pairs the notes of b with the notes of a at the same slot offset and
// returns, per pitch-class difference, the number of paired notes, plus the
// best pitch-class difference: most pairs, ties to the smallest
// transposition.
func (g *gridSim) match(a, b *candidate) (best, bestT int) {
	clear(g.has)

	for _, k := range a.notes {
		off := g.notes[k].Slot - a.start
		g.at[off], g.has[off] = g.notes[k].MIDI, true
	}

	var count [12]int

	for _, k := range b.notes {
		if off := g.notes[k].Slot - b.start; g.has[off] {
			count[((g.notes[k].MIDI-g.at[off])%12+12)%12]++
		}
	}

	best, bestT = -1, 0
	for t, n := range count {
		if n > best || (n == best && min(t, 12-t) < min(bestT, 12-bestT)) {
			best, bestT = n, t
		}
	}

	return best, bestT
}

// similarity is the Dice overlap of the two windows' notes paired by slot
// offset and pitch class under the best transposition, and that
// transposition as a pitch-class difference in [0, 11].
func (g *gridSim) similarity(a, b *candidate) (float64, int) {
	best, bestT := g.match(a, b)
	if best <= 0 {
		return 0, 0
	}

	return 2 * float64(best) / float64(len(a.notes)+len(b.notes)), bestT
}

// transposition is the median semitone difference of the notes paired under
// the best transposition, so octave errors do not leak into it.
func (g *gridSim) transposition(a, b *candidate) int {
	best, bestT := g.match(a, b)
	if best <= 0 {
		return 0
	}

	diffs := make([]int, 0, best)

	for _, k := range b.notes {
		if off := g.notes[k].Slot - b.start; g.has[off] {
			if d := g.notes[k].MIDI - g.at[off]; (d%12+12)%12 == bestT {
				diffs = append(diffs, d)
			}
		}
	}

	slices.Sort(diffs)

	return diffs[(len(diffs)-1)/2]
}

// noteDistance compares n-gram shapes. The interval term depends only on
// the absolute interval difference and the inter-onset term only on the two
// inter-onset intervals, so both are tabulated with the exact arithmetic of
// the direct formula.
type noteDistance struct {
	intervalWeight, ioiWeight float64
	intervalTerm              [16]float64 // |difference| 0..15; 1 beyond
	ioiTerm                   []float64   // (maxIOI+1)² entries, nil above maxUnitTable
	stride                    int
}

func newNoteDistance(cfg config, maxIOI int) *noteDistance {
	d := &noteDistance{intervalWeight: cfg.intervalWeight, ioiWeight: cfg.ioiWeight}

	for k := range d.intervalTerm {
		d.intervalTerm[k] = intervalTerm(k)
	}

	if maxIOI <= maxUnitTable {
		d.stride = maxIOI + 1
		d.ioiTerm = make([]float64, d.stride*d.stride)

		for a := 1; a <= maxIOI; a++ {
			for b := 1; b <= maxIOI; b++ {
				d.ioiTerm[a*d.stride+b] = ioiTerm(a, b)
			}
		}
	}

	return d
}

// intervalTerm is the octave-tolerant interval distance, capped at 3
// semitones and divided by 3, of an absolute interval difference.
func intervalTerm(diff int) float64 {
	di := float64(diff)

	return math.Min(math.Min(di, math.Abs(di-12)), 3) / 3
}

// ioiTerm is the absolute log2 ratio of two inter-onset intervals, capped at
// one octave of tempo.
func ioiTerm(a, b int) float64 {
	return math.Min(math.Abs(math.Log2(float64(a)/float64(b))), 1)
}

// distance is the mean weighted interval and inter-onset distance of two
// shapes of equal length.
func (d *noteDistance) distance(ia, ib, oa, ob []int) float64 {
	sum := 0.0
	for k := range ia {
		sum += d.term(ia[k]-ib[k], oa[k], ob[k])
	}

	return sum / float64(len(ia))
}

// similarity is 1 - distance, or -Inf as soon as the partial sum shows it
// will be below minSim. The terms are non-negative and floating-point
// addition, division and subtraction are monotone, so a partial sum that
// already fails the threshold proves the full sum fails it too; similarities
// at or above minSim are exactly 1 - distance.
func (d *noteDistance) similarity(ia, ib, oa, ob []int, minSim float64) float64 {
	sum, n := 0.0, float64(len(ia))

	for k := range ia {
		sum += d.term(ia[k]-ib[k], oa[k], ob[k])
		if 1-sum/n < minSim {
			return math.Inf(-1)
		}
	}

	return 1 - sum/n
}

// term is the weighted distance of one interval pair: diff is the interval
// difference, a and b the inter-onset intervals.
func (d *noteDistance) term(diff, a, b int) float64 {
	if diff < 0 {
		diff = -diff
	}

	di := 1.0
	if diff < len(d.intervalTerm) {
		di = d.intervalTerm[diff]
	}

	var dt float64
	if d.ioiTerm != nil {
		dt = d.ioiTerm[a*d.stride+b]
	} else {
		dt = ioiTerm(a, b)
	}

	return d.intervalWeight*di + d.ioiWeight*dt
}
