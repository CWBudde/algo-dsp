//nolint:all // Verbatim reference copy; kept byte-for-byte close to the original.
package motif

// This file holds a verbatim copy of AudioVisualizer's internal/story/motif.go
// (MotifParams, DefaultMotifParams, MotifOccurrence, SalienceTerms, Motif,
// candidate, cluster, noteDistance, FindNoteMotifs, gridSimilarity, fold,
// variant, rotate, FindChromaMotifs, argmax, Corroborate, Span, Score, Rank)
// and the parts of internal/story/{grid,harmony,notes}.go it uses (Grid,
// NewGrid and its methods, StoryNote, PitchNames, NoteName, cosine, abs, r3,
// r6). Adapted to standalone test code: audioanalysis.Rhythm is replaced by
// its plain fields (refRhythm), and identifiers carry a ref prefix. The
// arithmetic and its operation order are unchanged; the parity tests compare
// the package with this copy bit for bit.

import (
	"fmt"
	"math"
	"sort"
)

type refRhythm struct {
	BPM        float64
	BeatOrigin float64
	Beats      []float64
	Downbeat   int
}

type refGrid struct {
	BPM              float64   `json:"bpm"`
	OriginSeconds    float64   `json:"originSeconds"`
	BeatSeconds      float64   `json:"beatSeconds"`
	SixteenthSeconds float64   `json:"sixteenthSeconds"`
	BarSeconds       float64   `json:"barSeconds"`
	BeatsPerBar      int       `json:"beatsPerBar"`
	Downbeat         int       `json:"downbeatBeatIndex"`
	Duration         float64   `json:"durationSeconds"`
	Beats            []float64 `json:"beatsSeconds"`
	BarStarts        []float64 `json:"barStartsSeconds"`
	barOrigin        float64
}

func refNewGrid(r refRhythm, duration float64) refGrid {
	beat := 60 / r.BPM
	g := refGrid{BPM: r.BPM, OriginSeconds: r.BeatOrigin, BeatSeconds: beat, SixteenthSeconds: beat / 4, BarSeconds: 4 * beat, BeatsPerBar: 4, Downbeat: r.Downbeat, Duration: duration}
	g.barOrigin = g.OriginSeconds + float64(r.Downbeat%4)*beat
	if r.Downbeat%4 != 0 {
		g.barOrigin -= g.BarSeconds
	}
	g.Beats = append([]float64(nil), r.Beats...)
	for i := 0; len(r.Beats) == 0 && g.BeatStart(i) < duration; i++ {
		g.Beats = append(g.Beats, g.BeatStart(i))
	}
	for i := 0; g.BarStart(i) < duration; i++ {
		g.BarStarts = append(g.BarStarts, g.BarStart(i))
	}
	return g
}

func (g refGrid) Slot(t float64) int {
	return int(math.Round((t - g.OriginSeconds) / g.SixteenthSeconds))
}

func (g refGrid) SlotTime(slot int) float64 {
	return g.OriginSeconds + float64(slot)*g.SixteenthSeconds
}

func (g refGrid) Bar(t float64) int {
	return max(0, int(math.Floor((t-g.barOrigin)/g.BarSeconds+1e-9)))
}
func (g refGrid) Bars() int                     { return len(g.BarStarts) }
func (g refGrid) BarStart(n int) float64        { return g.barOrigin + float64(n)*g.BarSeconds }
func (g refGrid) BeatStart(n int) float64       { return g.OriginSeconds + float64(n)*g.BeatSeconds }
func (g refGrid) BarPosition(t float64) float64 { return (t - g.barOrigin) / g.BarSeconds }

func refR6(v float64) float64 { return math.Round(v*1e6) / 1e6 }
func refR3(v float64) float64 { return math.Round(v*1e3) / 1e3 }

type refStoryNote struct {
	Start       float64 `json:"startSeconds"`
	End         float64 `json:"endSeconds"`
	Slot        int     `json:"startSixteenth"`
	Slots       int     `json:"sixteenths"`
	MIDI        int     `json:"midi"`
	RawMIDI     int     `json:"rawMidi"`
	OctaveShift int     `json:"octaveShift,omitempty"`
	Strength    float64 `json:"strength"`
	Voice       string  `json:"voice"`
	OffsetMS    float64 `json:"gridOffsetMS"`
	OffGrid     bool    `json:"offGrid,omitempty"`
	RawIndex    int     `json:"rawIndex"`
}

var refPitchNames = [12]string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

func refNoteName(midi int) string {
	return fmt.Sprintf("%s%d", refPitchNames[(midi%12+12)%12], midi/12-1)
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

func refAbs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

type refMotifParams struct {
	GridWindows         []int   `json:"gridWindowSixteenths"`
	GridMinNotes        int     `json:"gridMinNotes"`
	GridMinSimilarity   float64 `json:"gridMinSimilarity"`
	GridTransposeCost   float64 `json:"gridTranspositionPenalty"`
	Lengths             []int   `json:"lengths"`
	MaxSpanSlots        int     `json:"maxSpanSixteenths"`
	MaxGapSlots         int     `json:"maxGapSixteenths"`
	MinSimilarity       float64 `json:"minSimilarity"`
	MinOccurrences      int     `json:"minOccurrences"`
	IntervalWeight      float64 `json:"intervalWeight"`
	IOIWeight           float64 `json:"ioiWeight"`
	ShiftOverlap        float64 `json:"sameLengthShiftOverlap"`
	CoveredShare        float64 `json:"shorterCoveredShare"`
	ChromaWindowBeats   []int   `json:"chromaWindowBeats"`
	ChromaMinSimilarity float64 `json:"chromaMinSimilarity"`
	ConfirmShare        float64 `json:"chromaConfirmShare"`
	OstinatoShare       float64 `json:"ostinatoShare"`
	Leitmotifs          int     `json:"leitmotifs"`
	LeitmotifOverlap    float64 `json:"leitmotifMaxOverlap"`
	LeitmotifMinRatio   float64 `json:"leitmotifMinSalienceRatio"`
	LeitmotifsPerSource int     `json:"leitmotifsPerSource"`
}

func refDefaultMotifParams() refMotifParams {
	return refMotifParams{[]int{16, 8}, 4, 0.5, 0.1, []int{8, 6, 4}, 32, 4, 0.8, 3, 0.7, 0.3, 0.5, 0.75, []int{8, 4}, 0.85, 0.6, 0.6, 4, 0.5, 0.5, 2}
}

type refMotifOccurrence struct {
	MotifID       string  `json:"motifId"`
	Start         float64 `json:"startSeconds"`
	End           float64 `json:"endSeconds"`
	Bar           int     `json:"bar"`
	Slot          int     `json:"startSixteenth"`
	Transposition int     `json:"transposition"`
	Similarity    float64 `json:"similarity"`
	Variant       string  `json:"variant"`
	NoteIndices   []int   `json:"noteIndices,omitempty"`
}

type refSalienceTerms struct {
	Count           float64 `json:"count"`
	Sections        float64 `json:"sections"`
	Prominence      float64 `json:"prominence"`
	Distinctiveness float64 `json:"distinctiveness"`
	Span            float64 `json:"span"`
	Confirmed       float64 `json:"confirmed"`
}

type refMotif struct {
	ID                string               `json:"id"`
	Source            string               `json:"source"`
	Role              string               `json:"role"`
	Notes             []string             `json:"notes"`
	SpanBeats         float64              `json:"spanBeats"`
	Intervals         []int                `json:"intervals"`
	IOI               []int                `json:"ioiSixteenths"`
	PrototypeMIDI     []int                `json:"prototypeMidi"`
	Occurrences       []refMotifOccurrence `json:"occurrences"`
	Salience          float64              `json:"salience"`
	SalienceTerms     refSalienceTerms     `json:"salienceTerms"`
	ConfirmedByChroma bool                 `json:"confirmedByChroma"`
	Leitmotif         bool                 `json:"leitmotif"`
	Rank              int                  `json:"rank,omitempty"`
}

// refCandidate is one n-gram (notes) or beat window (chroma) of a source.
type refCandidate struct {
	first, length int   // unit range (notes, slots or beats) for shift overlap
	start, end    int   // sixteenth span, used for overlap
	notes         []int // note indices, empty for chroma windows
}

func refOverlaps(a, b refCandidate) bool { return a.start < b.end && b.start < a.end }

// cluster greedily groups candidates around the one with the largest summed
// similarity to its available neighbours. sim returns similarity and transposition of b relative to a.
// Accepted occurrences are non-overlapping; candidates sharing at least
// shiftOverlap of their units with an accepted occurrence become unavailable,
// which keeps rotations of a repeating figure out of later clusters.
func refCluster(cands []refCandidate, available []bool, sim func(a, b int) (float64, int), minSim float64, minOcc int, shiftOverlap float64) []refGroup {
	neighbours := make([][]int, len(cands))
	for a := range cands {
		for b := range cands {
			if a != b && !refOverlaps(cands[a], cands[b]) {
				if s, _ := sim(a, b); s >= minSim {
					neighbours[a] = append(neighbours[a], b)
				}
			}
		}
	}
	out := []refGroup{}
	for {
		centre, count, mass := -1, 0, -1.0
		for a := range cands {
			if !available[a] {
				continue
			}
			n, sum := 0, 0.0
			for _, b := range neighbours[a] {
				if available[b] {
					s, _ := sim(a, b)
					n, sum = n+1, sum+s
				}
			}
			if sum > mass {
				centre, count, mass = a, n, sum
			}
		}
		if centre < 0 || count+1 < minOcc {
			return out
		}
		members := []refOcc{{centre, 1, 0}}
		for _, b := range neighbours[centre] {
			if available[b] {
				s, t := sim(centre, b)
				members = append(members, refOcc{b, s, t})
			}
		}
		sort.SliceStable(members, func(i, j int) bool { return members[i].similarity > members[j].similarity })
		accepted := []refOcc{}
		for _, m := range members {
			free := true
			for _, a := range accepted {
				free = free && !refOverlaps(cands[a.index], cands[m.index])
			}
			if free {
				accepted = append(accepted, m)
			}
		}
		for _, m := range members {
			available[m.index] = false
		}
		for c := range cands {
			for _, a := range accepted {
				lo, hi := max(cands[c].first, cands[a.index].first), min(cands[c].first+cands[c].length, cands[a.index].first+cands[a.index].length)
				if float64(hi-lo) >= shiftOverlap*float64(cands[c].length) {
					available[c] = false
				}
			}
		}
		if len(accepted) >= minOcc {
			sort.SliceStable(accepted, func(i, j int) bool { return cands[accepted[i].index].first < cands[accepted[j].index].first })
			out = append(out, refGroup{centre, accepted})
		}
	}
}

type refGroup struct {
	centre int
	occs   []refOcc // in time order
}

type refOcc struct {
	index         int
	similarity    float64
	transposition int
}

// refNoteDistance compares two n-grams' intervals (octave-tolerant, capped at 3
// semitones) and inter-onset ratios (capped at one octave of tempo).
func refNoteDistance(ia, ib, oa, ob []int, p refMotifParams) float64 {
	d := 0.0
	for k := range ia {
		di := float64(refAbs(ia[k] - ib[k]))
		di = math.Min(math.Min(di, math.Abs(di-12)), 3) / 3
		dt := math.Min(math.Abs(math.Log2(float64(oa[k])/float64(ob[k]))), 1)
		d += p.IntervalWeight*di + p.IOIWeight*dt
	}
	return d / float64(len(ia))
}

// refFindNoteMotifs finds recurring note material, transposition-invariant,
// longest first. refGrid windows (one bar, half a bar) compare notes by their
// sixteenth position and pitch class, which tolerates the tracker's dropped
// notes and octave errors. Windows start every half window from the first
// downbeat (bars and half bars, half bars and beats). N-grams then look for
// shorter figures among notes not yet mostly covered.
func refFindNoteMotifs(notes []refStoryNote, g refGrid, source string, p refMotifParams) []refMotif {
	covered := make([]bool, len(notes))
	motifs := []refMotif{}
	coveredShare := func(idx []int) float64 {
		c := 0
		for _, k := range idx {
			if covered[k] {
				c++
			}
		}
		return float64(c) / float64(len(idx))
	}
	shape := func(idx []int) (iv, ioi []int) {
		for k := 1; k < len(idx); k++ {
			iv = append(iv, notes[idx[k]].MIDI-notes[idx[k-1]].MIDI)
			ioi = append(ioi, max(1, notes[idx[k]].Slot-notes[idx[k-1]].Slot))
		}
		return
	}
	emit := func(cands []refCandidate, groups []refGroup, span func(c refCandidate) (float64, float64)) {
		for _, gr := range groups {
			// The cluster centre defines the shape; transpositions are
			// re-referenced to the earliest occurrence so the opening
			// statement reads as T0.
			proto := cands[gr.centre]
			t0 := gr.occs[0].transposition
			m := refMotif{Source: source, SpanBeats: refR3(float64(proto.end-proto.start) / 4)}
			m.Intervals, m.IOI = shape(proto.notes)
			// Keep the prototype's pitch classes at the first occurrence's
			// level and register (t0 may carry octave errors).
			first, mean := gr.occs[0], 0.0
			for _, k := range cands[first.index].notes {
				mean += float64(notes[k].MIDI) / float64(len(cands[first.index].notes))
			}
			for _, k := range proto.notes {
				mean -= float64(notes[k].MIDI+refFold(t0)) / float64(len(proto.notes))
			}
			shift := refFold(t0) + 12*int(math.Round(mean/12))
			for _, k := range proto.notes {
				m.PrototypeMIDI = append(m.PrototypeMIDI, notes[k].MIDI+shift)
				m.Notes = append(m.Notes, refNoteName(notes[k].MIDI+shift))
			}
			for _, o := range gr.occs {
				c := cands[o.index]
				iv, ioi := shape(c.notes)
				exact := fmt.Sprint(iv, ioi, notes[c.notes[0]].Slot-c.start) == fmt.Sprint(m.Intervals, m.IOI, notes[proto.notes[0]].Slot-proto.start)
				start, end := span(c)
				t := refFold(o.transposition - t0)
				refOcc := refMotifOccurrence{
					Start: refR6(start), End: refR6(end), Bar: g.Bar(g.SlotTime(c.start)), Slot: c.start,
					Transposition: t, Similarity: refR3(o.similarity), Variant: refVariant(exact && (o.transposition-t0)%12 == 0 == (t == 0), t), NoteIndices: c.notes,
				}
				for _, k := range c.notes {
					covered[k] = true
				}
				m.Occurrences = append(m.Occurrences, refOcc)
			}
			motifs = append(motifs, m)
		}
	}
	all := func(n int) []bool {
		v := make([]bool, n)
		for i := range v {
			v[i] = true
		}
		return v
	}
	for _, width := range p.GridWindows {
		cands := []refCandidate{}
		stride, origin := width/2, g.Slot(g.BarStart(0))
		for start := origin; len(notes) > 0 && start <= notes[len(notes)-1].Slot; start += stride {
			c := refCandidate{first: start, length: width, start: start, end: start + width}
			for k, n := range notes {
				if n.Slot >= c.start && n.Slot < c.end {
					c.notes = append(c.notes, k)
				}
			}
			if len(c.notes) >= p.GridMinNotes && coveredShare(c.notes) < p.CoveredShare {
				cands = append(cands, c)
			}
		}
		sim := func(a, b int) (float64, int) {
			v, t := refGridSimilarity(notes, cands[a], cands[b])
			if t%12 != 0 {
				v -= p.GridTransposeCost
			}
			return v, t
		}
		groups := refCluster(cands, all(len(cands)), sim, p.GridMinSimilarity, p.MinOccurrences, p.ShiftOverlap)
		emit(cands, groups, func(c refCandidate) (float64, float64) { return g.SlotTime(c.start), g.SlotTime(c.end) })
	}
	for _, length := range p.Lengths {
		cands, iv, ioi := []refCandidate{}, [][]int{}, [][]int{}
		for i := 0; i+length <= len(notes); i++ {
			last := notes[i+length-1]
			ok := last.Slot+last.Slots-notes[i].Slot <= p.MaxSpanSlots
			c := refCandidate{first: i, length: length, start: notes[i].Slot, end: last.Slot + last.Slots}
			for k := i; k < i+length; k++ {
				c.notes = append(c.notes, k)
				if k > i {
					ok = ok && notes[k].Slot-(notes[k-1].Slot+notes[k-1].Slots) <= p.MaxGapSlots
				}
			}
			if !ok || coveredShare(c.notes) >= p.CoveredShare {
				continue
			}
			a, b := shape(c.notes)
			cands, iv, ioi = append(cands, c), append(iv, a), append(ioi, b)
		}
		sim := func(a, b int) (float64, int) {
			d := []int{}
			for k := range length {
				d = append(d, notes[cands[b].first+k].MIDI-notes[cands[a].first+k].MIDI)
			}
			sort.Ints(d)
			return 1 - refNoteDistance(iv[a], iv[b], ioi[a], ioi[b], p), d[(len(d)-1)/2]
		}
		groups := refCluster(cands, all(len(cands)), sim, p.MinSimilarity, p.MinOccurrences, p.ShiftOverlap)
		emit(cands, groups, func(c refCandidate) (float64, float64) {
			return notes[c.notes[0]].Start, notes[c.notes[len(c.notes)-1]].End
		})
	}
	return motifs
}

// refGridSimilarity is the Dice overlap of two windows' notes matched by
// position within the window and pitch class, under the best transposition.
// The transposition reported is the median semitone difference of the matched
// notes, so octave errors do not leak into it.
func refGridSimilarity(notes []refStoryNote, a, b refCandidate) (float64, int) {
	at := map[int]int{}
	for _, k := range a.notes {
		at[notes[k].Slot-a.start] = notes[k].MIDI
	}
	best, bestT, bestDiffs := -1, 0, []int(nil)
	for t := range 12 {
		count, diffs := 0, []int{}
		for _, k := range b.notes {
			if m, ok := at[notes[k].Slot-b.start]; ok && ((notes[k].MIDI-m-t)%12+12)%12 == 0 {
				count++
				diffs = append(diffs, notes[k].MIDI-m)
			}
		}
		if count > best || (count == best && min(t, 12-t) < min(bestT, 12-bestT)) {
			best, bestT, bestDiffs = count, t, diffs
		}
	}
	if best <= 0 {
		return 0, 0
	}
	sort.Ints(bestDiffs)
	return 2 * float64(best) / float64(len(a.notes)+len(b.notes)), bestDiffs[(len(bestDiffs)-1)/2]
}

// refFold maps a transposition to [-6, 6] semitones: the tracker's octave
// errors make octave and direction unreliable.
func refFold(t int) int { return ((t%12)+18)%12 - 6 }

func refVariant(exact bool, t int) string {
	switch {
	case exact && t == 0:
		return "exact"
	case exact:
		return "transposed"
	case t == 0:
		return "varied"
	}
	return "transposed+varied"
}

func refRotate(c [12]float64, t int) [12]float64 {
	var out [12]float64
	for pc := range 12 {
		out[(pc+t+12)%12] = c[pc]
	}
	return out
}

// refFindChromaMotifs groups beat-chroma windows by optimal transposition index
// (the rotation maximising the mean per-beat refCosine), longest window first.
func refFindChromaMotifs(beats [][12]float64, g refGrid, p refMotifParams) []refMotif {
	coveredBeats := make([]bool, len(beats))
	motifs := []refMotif{}
	for _, width := range p.ChromaWindowBeats {
		cands := []refCandidate{}
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
			if float64(c) < p.CoveredShare*float64(width) && silent*2 < width {
				cands = append(cands, refCandidate{first: i, length: width, start: 4 * i, end: 4 * (i + width)})
			}
		}
		sim := func(a, b int) (float64, int) {
			best, bestT := math.Inf(-1), 0
			for t := range 12 {
				v := 0.0
				for k := range width {
					ra := refRotate(beats[cands[a].first+k], t)
					v += refCosine(ra[:], beats[cands[b].first+k][:]) / float64(width)
				}
				if v > best+1e-12 {
					best, bestT = v, t
				}
			}
			if bestT > 6 {
				bestT -= 12
			}
			return best, bestT
		}
		available := make([]bool, len(cands))
		for i := range available {
			available[i] = true
		}
		for _, gr := range refCluster(cands, available, sim, p.ChromaMinSimilarity, p.MinOccurrences, p.ShiftOverlap) {
			m := refMotif{Source: "chroma", SpanBeats: float64(width)}
			proto := cands[gr.centre]
			t0 := gr.occs[0].transposition
			for k := range width {
				m.Notes = append(m.Notes, refPitchNames[(refArgmax(beats[proto.first+k])+t0+12)%12])
			}
			for _, o := range gr.occs {
				c := cands[o.index]
				start, t := g.BeatStart(c.first), refFold(o.transposition-t0)
				m.Occurrences = append(m.Occurrences, refMotifOccurrence{
					Start: refR6(start), End: refR6(g.BeatStart(c.first + width)), Bar: g.Bar(start + 1e-6), Slot: c.start,
					Transposition: t, Similarity: refR3(o.similarity), Variant: refVariant(o.similarity >= 0.999, t),
				})
				for k := c.first; k < c.first+width; k++ {
					coveredBeats[k] = true
				}
			}
			motifs = append(motifs, m)
		}
	}
	return motifs
}

func refArgmax(c [12]float64) int {
	best := 0
	for i, v := range c {
		if v > c[best] {
			best = i
		}
	}
	return best
}

// refCorroborate marks a note motif confirmed when at least share of its
// occurrences start within one beat of an occurrence of a single chroma motif
// at a consistent relative transposition (mod 12).
func refCorroborate(m *refMotif, chroma []refMotif, beat, share float64) {
	for _, c := range chroma {
		offsets := map[int]int{}
		for _, o := range m.Occurrences {
			for _, co := range c.Occurrences {
				if math.Abs(co.Start-o.Start) <= beat {
					offsets[((co.Transposition-o.Transposition)%12+12)%12]++
					break
				}
			}
		}
		for _, n := range offsets {
			if float64(n) >= share*float64(len(m.Occurrences)) {
				m.ConfirmedByChroma = true
				return
			}
		}
	}
}

// Span is a named time range, used to count the sections a motif visits.
type refSpan struct {
	Name       string
	Start, End float64
}

// refScore computes salience and role. Prominence is mean note strength
// relative to the source median (note motifs) × mean source energy control
// over the occurrences × mean occurrence similarity; the similarity factor
// keeps loose clusters from outranking tight ones.
func refScore(m *refMotif, notes []refStoryNote, sections []refSpan, energy func(t0, t1 float64) float64, g refGrid, p refMotifParams) {
	visited := map[string]bool{}
	prominence, strength, n := 0.0, 0.0, 0.0
	for _, o := range m.Occurrences {
		for _, s := range sections {
			if o.Start >= s.Start && o.Start < s.End {
				visited[s.Name] = true
			}
		}
		prominence += energy(o.Start, o.End) / float64(len(m.Occurrences))
		for _, k := range o.NoteIndices {
			strength += notes[k].Strength
			n++
		}
	}
	similarity := 0.0
	for _, o := range m.Occurrences {
		similarity += o.Similarity / float64(len(m.Occurrences))
	}
	prominence *= similarity
	if n > 0 {
		// Relative to the source's median note strength: voicing shares of a
		// clean bass line and a polyphonic lead stem are not comparable.
		all := []float64{}
		for _, note := range notes {
			all = append(all, note.Strength)
		}
		sort.Float64s(all)
		prominence *= strength / n / all[len(all)/2]
	}
	symbols := map[string]float64{}
	if len(m.Intervals) > 0 {
		for _, v := range m.Intervals {
			symbols[fmt.Sprint(v)]++
		}
	} else {
		for _, v := range m.Notes {
			symbols[v]++
		}
	}
	entropy, total := 0.0, 0.0
	for _, c := range symbols {
		total += c
	}
	for _, c := range symbols {
		entropy -= c / total * math.Log2(c/total)
	}
	t := refSalienceTerms{
		Count: math.Log(1 + float64(len(m.Occurrences))), Sections: math.Log(1 + float64(len(visited))), Prominence: prominence,
		Distinctiveness: math.Min(entropy, 1), Span: math.Min(m.SpanBeats, 8) / 8, Confirmed: 1,
	}
	if entropy < 1 {
		t.Distinctiveness *= 0.3
	}
	if m.ConfirmedByChroma {
		t.Confirmed = 1.2
	}
	m.Salience = refR3(t.Count * t.Sections * t.Prominence * t.Distinctiveness * t.Span * t.Confirmed)
	m.SalienceTerms = refSalienceTerms{refR3(t.Count), refR3(t.Sections), refR3(t.Prominence), refR3(t.Distinctiveness), refR3(t.Span), t.Confirmed}
	// Ostinato: most occurrences sit within a bar of another one. (Requiring
	// every occurrence to follow its predecessor made one undetected bar
	// break the chain.)
	adjacent := 0
	for i, o := range m.Occurrences {
		near := false
		for j, q := range m.Occurrences {
			near = near || (j != i && math.Max(o.Start-q.End, q.Start-o.End) <= g.BarSeconds+g.SixteenthSeconds/2)
		}
		if near {
			adjacent++
		}
	}
	m.Role = "theme"
	if float64(adjacent) > p.OstinatoShare*float64(len(m.Occurrences)) {
		m.Role = "ostinato"
	}
}

// Rank assigns ids by descending salience (note motifs M1…, chroma motifs
// C1…) and picks up to p.Leitmotifs note motifs with at least
// p.LeitmotifMinRatio of the best salience: the best theme first, then the
// best ostinato, then the rest by salience, each overlapping already picked
// ones for less than p.LeitmotifOverlap of its occurrence time and at most
// p.LeitmotifsPerSource per source. Ranks follow salience.
func refRank(motifs []refMotif, p refMotifParams) []refMotif {
	sort.SliceStable(motifs, func(i, j int) bool {
		if (motifs[i].Source == "chroma") != (motifs[j].Source == "chroma") {
			return motifs[j].Source == "chroma"
		}
		return motifs[i].Salience > motifs[j].Salience
	})
	nm, nc := 0, 0
	for i := range motifs {
		if motifs[i].Source == "chroma" {
			nc++
			motifs[i].ID = fmt.Sprintf("C%d", nc)
		} else {
			nm++
			motifs[i].ID = fmt.Sprintf("M%d", nm)
		}
		for k := range motifs[i].Occurrences {
			motifs[i].Occurrences[k].MotifID = motifs[i].ID
		}
	}
	best := 0.0
	for _, m := range motifs {
		if m.Source != "chroma" {
			best = math.Max(best, m.Salience)
		}
	}
	picked, perSource := []int{}, map[string]int{}
	try := func(i int) bool {
		m := motifs[i]
		if m.Source == "chroma" || m.Leitmotif || len(picked) >= p.Leitmotifs || m.Salience < p.LeitmotifMinRatio*best || perSource[m.Source] >= p.LeitmotifsPerSource {
			return false
		}
		total, shared := 0.0, 0.0
		for _, o := range m.Occurrences {
			total += o.End - o.Start
			for _, j := range picked {
				for _, q := range motifs[j].Occurrences {
					shared += math.Max(0, math.Min(o.End, q.End)-math.Max(o.Start, q.Start))
				}
			}
		}
		if total > 0 && shared/total >= p.LeitmotifOverlap {
			return false
		}
		picked = append(picked, i)
		perSource[m.Source]++
		motifs[i].Leitmotif = true
		return true
	}
	for _, role := range []string{"theme", "ostinato", ""} {
		for i := range motifs {
			if (role == "" || motifs[i].Role == role) && try(i) && role != "" {
				break
			}
		}
	}
	sort.Ints(picked)
	for r, i := range picked {
		motifs[i].Rank = r + 1
	}
	return motifs
}
