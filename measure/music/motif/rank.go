package motif

import (
	"cmp"
	"fmt"
	"math"
	"slices"

	"github.com/cwbudde/algo-dsp/measure/music/melody"
	"github.com/cwbudde/algo-dsp/measure/music/rhythm"
)

// Salience constants of [Score].
const (
	// spanSaturationBeats is the motif span at which the span term reaches 1.
	spanSaturationBeats = 8
	// lowEntropyScale scales the distinctiveness of motifs with less than one
	// bit of interval (or pitch-class) entropy.
	lowEntropyScale = 0.3
	// confirmedBonus is the confirmation term of a motif confirmed by chroma.
	confirmedBonus = 1.2
)

// Corroborate sets m.ConfirmedByChroma when at least [WithConfirmShare] of
// m's occurrences start within one beat of the grid of an occurrence of a
// single chroma motif, at a consistent relative transposition (mod 12). For
// each occurrence of m the first such chroma occurrence counts. A motif
// already confirmed stays confirmed.
//
// chroma are typically the motifs of [FindChromaMotifs]; m is a motif of
// [FindNoteMotifs]. Options read: [WithConfirmShare].
func Corroborate(m *Motif, chroma []Motif, grid rhythm.Grid, opts ...Option) error {
	cfg, err := newConfig(opts)
	if err != nil {
		return err
	}

	err = validGrid(grid)
	if err != nil {
		return err
	}

	if m == nil {
		return fmt.Errorf("%w: nil motif", ErrInvalidArgument)
	}

	beat := grid.BeatSeconds()

	for _, c := range chroma {
		var offsets [12]int

		for _, o := range m.Occurrences {
			for _, co := range c.Occurrences {
				if math.Abs(co.Start-o.Start) <= beat {
					offsets[((co.Transposition-o.Transposition)%12+12)%12]++

					break
				}
			}
		}

		for _, n := range offsets {
			if n > 0 && float64(n) >= cfg.confirmShare*float64(len(m.Occurrences)) {
				m.ConfirmedByChroma = true

				return nil
			}
		}
	}

	return nil
}

// Score sets m's salience, salience terms and role.
//
// The salience is the product of the [SalienceTerms]:
//   - Count: ln(1 + occurrences).
//   - Sections: ln(1 + the number of distinct sections, by name, that
//     contain an occurrence start).
//   - Prominence: the mean of energy(Start, End) over the occurrences, times
//     the mean occurrence similarity (so loose clusters do not outrank tight
//     ones), times, when the occurrences have notes, their mean strength
//     relative to the median strength of notes (voicing shares of a clean
//     bass line and a polyphonic lead are not comparable). The strength
//     factor is left out when the median strength is not positive.
//   - Distinctiveness: the entropy in bits of the intervals (or, for chroma
//     motifs, the note names), capped at 1, times 0.3 below 1 bit.
//   - Span: SpanBeats capped at 8 beats, divided by 8.
//   - Confirmed: 1.2 if m.ConfirmedByChroma, else 1.
//
// The role is [RoleOstinato] if more than [WithOstinatoShare] of the
// occurrences lie within a bar plus half a slot of another occurrence, else
// [RoleTheme].
//
// notes is the note slice the motif was found in ([Occurrence].NoteIndices
// index it; nil for chroma motifs). energy returns the level of the motif's
// source over a time range in seconds, for example the mean frame energy of
// the source's stem; it is called once per occurrence. sections are the
// song sections.
//
// Score returns an error wrapping [ErrInvalidArgument], and leaves m
// unchanged, when energy returns a non-finite value or the salience is not
// finite (for example from non-finite note strengths).
//
// The defaults reproduce AudioVisualizer's Score bit for bit. Options read:
// [WithOstinatoShare].
func Score(m *Motif, notes []melody.CleanNote, sections []Span, energy func(start, end float64) float64,
	grid rhythm.Grid, opts ...Option,
) error {
	cfg, err := newConfig(opts)
	if err != nil {
		return err
	}

	err = validGrid(grid)
	if err != nil {
		return err
	}

	switch {
	case m == nil:
		return fmt.Errorf("%w: nil motif", ErrInvalidArgument)
	case energy == nil:
		return fmt.Errorf("%w: nil energy function", ErrInvalidArgument)
	}

	for i, o := range m.Occurrences {
		for _, k := range o.NoteIndices {
			if k < 0 || k >= len(notes) {
				return fmt.Errorf("%w: occurrence %d has note index %d, have %d notes", ErrInvalidArgument, i, k, len(notes))
			}
		}
	}

	t, err := salienceTerms(m, notes, sections, energy)
	if err != nil {
		return err
	}

	salience := t.Count * t.Sections * t.Prominence * t.Distinctiveness * t.Span * t.Confirmed
	if !finite(salience) {
		return fmt.Errorf("%w: salience is %g (prominence %g)", ErrInvalidArgument, salience, t.Prominence)
	}

	m.Salience = r3(salience)
	m.SalienceTerms = SalienceTerms{r3(t.Count), r3(t.Sections), r3(t.Prominence), r3(t.Distinctiveness), r3(t.Span), t.Confirmed}
	m.Role = role(m.Occurrences, grid, cfg.ostinatoShare)

	return nil
}

// salienceTerms returns the unrounded salience terms of m, or an error for a
// non-finite energy.
func salienceTerms(m *Motif, notes []melody.CleanNote, sections []Span,
	energy func(start, end float64) float64,
) (SalienceTerms, error) {
	visited := map[string]bool{}
	prominence, strength, n := 0.0, 0.0, 0.0

	for _, o := range m.Occurrences {
		for _, s := range sections {
			if o.Start >= s.Start && o.Start < s.End {
				visited[s.Name] = true
			}
		}

		e := energy(o.Start, o.End)
		if !finite(e) {
			return SalienceTerms{}, fmt.Errorf("%w: energy of %g..%g s is %g", ErrInvalidArgument, o.Start, o.End, e)
		}

		prominence += e / float64(len(m.Occurrences))

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
		all := make([]float64, len(notes))
		for i, note := range notes {
			all[i] = note.Strength
		}

		slices.Sort(all)

		// A median of zero (or NaN) would make the salience infinite or NaN;
		// cleaned notes always have positive strengths.
		if median := all[len(all)/2]; median > 0 {
			prominence *= strength / n / median
		}
	}

	entropy := symbolEntropy(m)
	t := SalienceTerms{
		Count: math.Log(1 + float64(len(m.Occurrences))), Sections: math.Log(1 + float64(len(visited))), Prominence: prominence,
		Distinctiveness: math.Min(entropy, 1), Span: math.Min(m.SpanBeats, spanSaturationBeats) / spanSaturationBeats, Confirmed: 1,
	}

	if entropy < 1 {
		t.Distinctiveness *= lowEntropyScale
	}

	if m.ConfirmedByChroma {
		t.Confirmed = confirmedBonus
	}

	return t, nil
}

// symbolEntropy is the Shannon entropy in bits of m's intervals, or of its
// note names when it has none. The terms are summed in order of first
// appearance.
func symbolEntropy(m *Motif) float64 {
	var counts []float64

	index := map[string]int{}
	add := func(symbol string) {
		i, ok := index[symbol]
		if !ok {
			i = len(counts)
			index[symbol] = i

			counts = append(counts, 0)
		}

		counts[i]++
	}

	if len(m.Intervals) > 0 {
		for _, v := range m.Intervals {
			add(fmt.Sprint(v))
		}
	} else {
		for _, v := range m.Notes {
			add(v)
		}
	}

	entropy, total := 0.0, 0.0
	for _, c := range counts {
		total += c
	}

	for _, c := range counts {
		entropy -= c / total * math.Log2(c/total)
	}

	return entropy
}

// role is RoleOstinato when more than share of the occurrences lie within a
// bar (plus half a slot) of another one. Requiring every occurrence to
// follow its predecessor would let one missed bar break the chain.
func role(occs []Occurrence, grid rhythm.Grid, share float64) Role {
	limit := grid.BarSeconds() + grid.SlotSeconds()/2
	adjacent := 0

	for i, o := range occs {
		near := false
		for j, q := range occs {
			near = near || (j != i && math.Max(o.Start-q.End, q.Start-o.End) <= limit)
		}

		if near {
			adjacent++
		}
	}

	if float64(adjacent) > share*float64(len(occs)) {
		return RoleOstinato
	}

	return RoleTheme
}

// Rank orders motifs and picks the leitmotifs, in place, and returns
// motifs.
//
// Note motifs come first, then chroma motifs ([SourceChroma]), each by
// descending salience (stable). IDs are "M1", "M2", … for note motifs and
// "C1", … for chroma motifs in that order, and are copied into the
// occurrences.
//
// Up to [WithLeitmotifs] note motifs with at least [WithLeitmotifMinRatio]
// of the best note motif's salience become leitmotifs: the best
// [RoleTheme] motif first, then the best [RoleOstinato] motif, then the
// rest by salience. A motif is skipped when it shares at least
// [WithLeitmotifOverlap] of its occurrence time with the leitmotifs picked
// before it, or when its source already has [WithLeitmotifsPerSource]
// leitmotifs. Leitmotifs are ranked 1, 2, … by salience.
//
// Call [Score] on every motif first. Rank clears Leitmotif and Rank of all
// motifs before picking, so calling it again gives the same result. A
// non-finite salience returns an error wrapping [ErrInvalidArgument] and
// leaves motifs unchanged. The defaults reproduce AudioVisualizer's Rank bit
// for bit. Options read: [WithLeitmotifs], [WithLeitmotifOverlap],
// [WithLeitmotifMinRatio] and [WithLeitmotifsPerSource].
func Rank(motifs []Motif, opts ...Option) ([]Motif, error) {
	cfg, err := newConfig(opts)
	if err != nil {
		return nil, err
	}

	for i := range motifs {
		if !finite(motifs[i].Salience) {
			return nil, fmt.Errorf("%w: motif %d has salience %g", ErrInvalidArgument, i, motifs[i].Salience)
		}
	}

	for i := range motifs {
		motifs[i].Leitmotif, motifs[i].Rank = false, 0
	}

	slices.SortStableFunc(motifs, func(a, b Motif) int {
		if ac, bc := a.Source == SourceChroma, b.Source == SourceChroma; ac != bc {
			if bc {
				return -1
			}

			return 1
		}

		return cmp.Compare(b.Salience, a.Salience)
	})

	nm, nc := 0, 0

	for i := range motifs {
		if motifs[i].Source == SourceChroma {
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

	pickLeitmotifs(motifs, cfg)

	return motifs, nil
}

// pickLeitmotifs marks and ranks the leitmotifs of the sorted motifs.
func pickLeitmotifs(motifs []Motif, cfg config) {
	best := 0.0

	for _, m := range motifs {
		if m.Source != SourceChroma {
			best = math.Max(best, m.Salience)
		}
	}

	picked, perSource := []int{}, map[string]int{}
	try := func(i int) bool {
		m := &motifs[i]
		if m.Source == SourceChroma || m.Leitmotif || len(picked) >= cfg.leitmotifs ||
			m.Salience < cfg.leitmotifMinRatio*best || perSource[m.Source] >= cfg.leitmotifsPerSource {
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

		// shared > 0 keeps disjoint motifs eligible when the limit is 0.
		if total > 0 && shared > 0 && shared/total >= cfg.leitmotifOverlap {
			return false
		}

		picked = append(picked, i)
		perSource[m.Source]++
		m.Leitmotif = true

		return true
	}

	// The best theme, then the best ostinato, then everything else.
	for _, r := range []Role{RoleTheme, RoleOstinato} {
		for i := range motifs {
			if motifs[i].Role == r && try(i) {
				break
			}
		}
	}

	for i := range motifs {
		try(i)
	}

	slices.Sort(picked)

	for r, i := range picked {
		motifs[i].Rank = r + 1
	}
}
