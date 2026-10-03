package harmony

import (
	"fmt"
	"math"
	"slices"

	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
)

// Default chord detection parameters. They reproduce the AudioVisualizer
// chord chart exactly.
const (
	// DefaultBassRootBonus is the default weight of the bass evidence for a
	// chord: up to this much is added to the chroma similarity when the
	// strongest bass pitch class is the chord root. A larger value (0.3) let
	// a bass on the third outvote the chroma and read B7/D♯ as D♯m.
	DefaultBassRootBonus = 0.2
	// DefaultInversionShare is the default share of the bass bonus granted
	// for a bass on a chord tone other than the root (inversions).
	DefaultInversionShare = 0.5
	// DefaultSeventhPenalty is the default score subtracted from templates
	// with more than three tones, so that a triad wins over its seventh
	// chord unless the seventh is heard.
	DefaultSeventhPenalty = 0.03
	// DefaultInKeyBonus is the default score added to chords whose tones all
	// belong to the key's scale.
	DefaultInKeyBonus = 0.05
	// DefaultMinPeakRatio is the default lowest ratio of the strongest to the
	// mean chroma value for a window to carry a chord; flatter windows are
	// "N".
	DefaultMinPeakRatio = 1.5
	// DefaultGateDB is the default level gate in dBFS; quieter windows are
	// "N".
	DefaultGateDB = -50.0
	// DefaultSwitchPenalty is the default score cost of a chord change in the
	// Viterbi smoothing.
	DefaultSwitchPenalty = 0.10
	// DefaultScoreDecimals is the default number of decimals [Chord.Score]
	// and [Chord.Margin] are rounded to (see [WithScoreDecimals]).
	DefaultScoreDecimals = 3

	// VoicingBase is the MIDI note of the C an octave below middle C (C3).
	// [Chord.Voicing] is the template stacked on the root in the octave
	// starting here.
	VoicingBase = 48

	// maxScoreDecimals is the largest accepted [WithScoreDecimals] value.
	maxScoreDecimals = 15
)

// noChord is the quality and symbol of a window without a chord.
const noChord = "N"

// ChordTemplate is a chord quality: a name, a symbol suffix and the chord
// tones as semitones above the root.
type ChordTemplate struct {
	// Name is the quality name reported in [Chord.Quality], for example
	// "maj" or "m7".
	Name string
	// Suffix follows the root in [Chord.Symbol], for example "" (C) or "m7"
	// (Cm7).
	Suffix string
	// Intervals are the chord tones in semitones above the root, root (0)
	// first, each in [0, 11] and distinct.
	Intervals []int
}

// DefaultTemplates returns the default template set: major and minor triads
// and the dominant, major and minor seventh chords. The result is a fresh
// copy.
func DefaultTemplates() []ChordTemplate {
	return []ChordTemplate{
		{Name: "maj", Suffix: "", Intervals: []int{0, 4, 7}},
		{Name: "min", Suffix: "m", Intervals: []int{0, 3, 7}},
		{Name: "7", Suffix: "7", Intervals: []int{0, 4, 7, 10}},
		{Name: "maj7", Suffix: "maj7", Intervals: []int{0, 4, 7, 11}},
		{Name: "m7", Suffix: "m7", Intervals: []int{0, 3, 7, 10}},
	}
}

// Chord is one entry of the chord chart: a run of windows with the same
// chord symbol.
type Chord struct {
	// Start is the start of the first window and End the end of the last
	// window of the run, in seconds.
	Start, End float64
	// NoChord marks a run of "N" windows (gated or flat). Root and Bass are
	// then zero and meaningless, and Voicing is empty.
	NoChord bool
	// Root is the chord root.
	Root pitch.PitchClass
	// Quality is the [ChordTemplate] name, or "N".
	Quality string
	// Bass is the bass pitch class: the root, or for a slash chord the
	// chord tone that carries the strongest bass evidence.
	Bass pitch.PitchClass
	// Symbol is the chord symbol, for example "Am", "G7" or "C/E"; "N" for
	// no chord.
	Symbol string
	// Score is the mean per-window score of the chosen chord (1 for "N"):
	// chroma cosine similarity plus bass and key bonuses minus the seventh
	// penalty. With score rounding, the running mean is rounded after every
	// merged window.
	Score float64
	// Margin is the smallest per-window gap between the best and the second
	// best chord score over the run; 0 when a window had only one finite
	// score (an "N" window).
	Margin float64
	// Voicing is the chord as MIDI notes: root + interval above
	// [VoicingBase], in template order.
	Voicing []int
}

// ChordOption configures [Chords] and [NewChorder]. Options return an error
// wrapping [ErrInvalidOption] for out-of-range values.
type ChordOption func(*chordConfig) error

type chordConfig struct {
	templates      []ChordTemplate
	bassWeight     float64
	inversionShare float64
	seventhCost    float64
	inKeyBonus     float64
	minPeakRatio   float64
	gateDB         float64
	switchCost     float64
	decimals       int
}

func defaultChordConfig() chordConfig {
	return chordConfig{
		templates:      DefaultTemplates(),
		bassWeight:     DefaultBassRootBonus,
		inversionShare: DefaultInversionShare,
		seventhCost:    DefaultSeventhPenalty,
		inKeyBonus:     DefaultInKeyBonus,
		minPeakRatio:   DefaultMinPeakRatio,
		gateDB:         DefaultGateDB,
		switchCost:     DefaultSwitchPenalty,
		decimals:       DefaultScoreDecimals,
	}
}

// WithTemplates sets the chord templates (default [DefaultTemplates]). The
// order matters only for exact score ties, where the earlier template wins.
// Names and suffixes must be distinct, names non-empty and not "N", and every
// template needs at least two intervals, the first 0. The templates are
// copied.
func WithTemplates(templates ...ChordTemplate) ChordOption {
	copied := make([]ChordTemplate, len(templates))
	for i, t := range templates {
		copied[i] = ChordTemplate{Name: t.Name, Suffix: t.Suffix, Intervals: slices.Clone(t.Intervals)}
	}

	return func(cfg *chordConfig) error {
		err := validateTemplates(copied)
		if err != nil {
			return err
		}

		cfg.templates = copied

		return nil
	}
}

func validateTemplates(templates []ChordTemplate) error {
	if len(templates) == 0 {
		return fmt.Errorf("%w: at least one chord template is required", ErrInvalidOption)
	}

	for i, t := range templates {
		if t.Name == "" || t.Name == noChord {
			return fmt.Errorf("%w: template %d has name %q", ErrInvalidOption, i, t.Name)
		}

		if len(t.Intervals) < 2 || t.Intervals[0] != 0 {
			return fmt.Errorf("%w: template %q needs the root 0 first and at least two intervals, got %v",
				ErrInvalidOption, t.Name, t.Intervals)
		}

		var seen [pitchClasses]bool

		for _, iv := range t.Intervals {
			if iv < 0 || iv >= pitchClasses || seen[iv] {
				return fmt.Errorf("%w: template %q has invalid or repeated interval %d", ErrInvalidOption, t.Name, iv)
			}

			seen[iv] = true
		}

		for _, u := range templates[:i] {
			if u.Name == t.Name || u.Suffix == t.Suffix {
				return fmt.Errorf("%w: templates %q and %q share a name or suffix", ErrInvalidOption, u.Name, t.Name)
			}
		}
	}

	return nil
}

func nonNegative(name string, v float64, set *float64) error {
	if !finite(v) || v < 0 {
		return fmt.Errorf("%w: %s must be >= 0, got %g", ErrInvalidOption, name, v)
	}

	*set = v

	return nil
}

// WithBassRootBonus sets the weight of the bass evidence (default
// [DefaultBassRootBonus]). The bonus is weight·b/max, where max is the
// window's strongest bass weight and b the bass weight of the root, or the
// inversion share times that of another chord tone if larger. 0 ignores the
// bass.
func WithBassRootBonus(weight float64) ChordOption {
	return func(cfg *chordConfig) error { return nonNegative("bass root bonus", weight, &cfg.bassWeight) }
}

// WithInversionShare sets the share of the bass bonus, in [0, 1], for a bass
// on a chord tone other than the root (default [DefaultInversionShare]).
func WithInversionShare(share float64) ChordOption {
	return func(cfg *chordConfig) error {
		if !(share >= 0 && share <= 1) {
			return fmt.Errorf("%w: inversion share must be in [0, 1], got %g", ErrInvalidOption, share)
		}

		cfg.inversionShare = share

		return nil
	}
}

// WithSeventhPenalty sets the score subtracted from templates with more than
// three tones (default [DefaultSeventhPenalty]).
func WithSeventhPenalty(penalty float64) ChordOption {
	return func(cfg *chordConfig) error { return nonNegative("seventh penalty", penalty, &cfg.seventhCost) }
}

// WithInKeyBonus sets the score added to chords whose tones all lie in the
// key's scale (default [DefaultInKeyBonus]).
func WithInKeyBonus(bonus float64) ChordOption {
	return func(cfg *chordConfig) error { return nonNegative("in-key bonus", bonus, &cfg.inKeyBonus) }
}

// WithMinPeakRatio sets the lowest ratio of the strongest to the mean chroma
// value for a window to carry a chord (default [DefaultMinPeakRatio]).
// Windows with zero chroma are always "N".
func WithMinPeakRatio(ratio float64) ChordOption {
	return func(cfg *chordConfig) error { return nonNegative("minimum peak ratio", ratio, &cfg.minPeakRatio) }
}

// WithGateDB sets the level gate in dBFS (default [DefaultGateDB]): windows
// whose [Window.LevelDB] is below it are "N". math.Inf(-1) disables the gate.
func WithGateDB(db float64) ChordOption {
	return func(cfg *chordConfig) error {
		if math.IsNaN(db) || math.IsInf(db, 1) {
			return fmt.Errorf("%w: gate must be finite or -Inf, got %g", ErrInvalidOption, db)
		}

		cfg.gateDB = db

		return nil
	}
}

// WithSwitchPenalty sets the cost of a chord change in the Viterbi smoothing
// (default [DefaultSwitchPenalty]). 0 picks the best chord per window.
func WithSwitchPenalty(penalty float64) ChordOption {
	return func(cfg *chordConfig) error { return nonNegative("switch penalty", penalty, &cfg.switchCost) }
}

// WithScoreDecimals sets the number of decimals, at most 15, that
// [Chord.Score] and [Chord.Margin] are rounded to (default
// [DefaultScoreDecimals]). Each window's score and margin are rounded before
// merging and the running mean score after each merged window, as in
// AudioVisualizer. A negative value disables rounding: Score is then the
// exact running mean.
func WithScoreDecimals(decimals int) ChordOption {
	return func(cfg *chordConfig) error {
		if decimals > maxScoreDecimals {
			return fmt.Errorf("%w: score decimals must be <= %d, got %d", ErrInvalidOption, maxScoreDecimals, decimals)
		}

		cfg.decimals = decimals

		return nil
	}
}

// chordState is one Viterbi state: a root and a template. State 0 is "N".
type chordState struct {
	root     int
	template int
	// tones are the chord pitch classes in template order, root first.
	tones []int
	// member marks the chord pitch classes for the cosine similarity.
	member [pitchClasses]bool
	// norm is the squared norm of the binary template.
	norm    float64
	seventh bool
	symbol  string
	// slash is the slash-chord symbol for a bass on a non-root chord tone,
	// indexed by the bass pitch class; empty for other pitch classes.
	slash [pitchClasses]string
}

// Chorder detects chords with reusable buffers. Create it with
// [NewChorder]; once its buffers have grown to the largest input, its
// [Chorder.Chords] method does not allocate. A Chorder is not safe for
// concurrent use.
type Chorder struct {
	cfg    chordConfig
	scale  float64
	states []chordState

	inKey  []bool
	scores []float64
	back   []int32
	acc    []float64
	next   []float64
	path   []int32
	merged []int
}

// NewChorder returns a chord detector with the given options.
func NewChorder(opts ...ChordOption) (*Chorder, error) {
	cfg := defaultChordConfig()

	for i, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("%w: chord option %d", ErrNilOption, i)
		}

		err := opt(&cfg)
		if err != nil {
			return nil, err
		}
	}

	c := &Chorder{cfg: cfg, scale: 1}
	for range max(cfg.decimals, 0) {
		c.scale *= 10
	}

	c.states = make([]chordState, 1, 1+pitchClasses*len(cfg.templates))
	c.states[0] = chordState{root: -1, template: -1, symbol: noChord}

	for root := range pitchClasses {
		for q, t := range cfg.templates {
			st := chordState{root: root, template: q, seventh: len(t.Intervals) > 3}
			st.symbol = pitch.PitchClass(root).String() + t.Suffix

			for _, iv := range t.Intervals {
				pc := (root + iv) % pitchClasses
				st.tones = append(st.tones, pc)
				st.member[pc] = true
				st.norm++
			}

			for _, pc := range st.tones[1:] {
				st.slash[pc] = st.symbol + "/" + pitch.PitchClass(pc).String()
			}

			c.states = append(c.states, st)
		}
	}

	ns := len(c.states)
	c.inKey = make([]bool, ns)
	c.acc = make([]float64, ns)
	c.next = make([]float64, ns)

	return c, nil
}

// Chords detects the chord chart of windows (as from [Windows]) in the given
// key, the one-shot form of [Chorder.Chords]. The zero [Key] is C major, so
// pass the estimated key (see [EstimateKey]): the in-key bonus applies to the
// chords of the key passed in. Chords returns an empty, non-nil slice for no
// windows.
func Chords(windows []Window, key Key, opts ...ChordOption) ([]Chord, error) {
	c, err := NewChorder(opts...)
	if err != nil {
		return nil, err
	}

	return c.Chords(nil, windows, key)
}

// Chords detects the chord chart of windows (as from [Windows]) in the given
// key and appends it to dst[:0], reusing dst's storage including the
// capacity of its Voicing slices. The windows should be in time order. The
// result is never nil: no windows give an empty slice.
//
// The in-key bonus ([WithInKeyBonus]) applies to chords whose tones all lie
// in the scale of key ([Key.Scale]). The zero [Key] is C major, so a key
// left unset favours the chords of C major.
//
// Each window is scored against every root and template:
//
//   - the cosine similarity of the window chroma with the binary template;
//   - plus the bass bonus (see [WithBassRootBonus], [WithInversionShare])
//     when the window has bass evidence;
//   - minus the seventh penalty for templates of more than three tones;
//   - plus the in-key bonus when every chord tone is in the key's scale.
//
// Windows below the level gate or with flat chroma (see [WithGateDB],
// [WithMinPeakRatio]) can only be "N" and no other window can. A Viterbi
// pass with a constant switch penalty picks the chord sequence with the best
// total score (ties keep the lower state, "N" first, then roots C to B and
// templates in order). A chord is a slash chord when the strongest bass
// pitch class is one of its non-root tones. Consecutive windows with the same
// symbol are merged.
//
// Window chroma and bass values must be finite and non-negative, window
// bounds finite with Start ≤ End, and LevelDB not NaN or +Inf.
func (c *Chorder) Chords(dst []Chord, windows []Window, key Key) ([]Chord, error) {
	err := key.validate()
	if err != nil {
		return nil, err
	}

	for i := range windows {
		err = validateWindow(i, &windows[i])
		if err != nil {
			return nil, err
		}
	}

	nw, ns := len(windows), len(c.states)
	c.grow(nw)

	scale := key.Scale()

	for s := 1; s < ns; s++ {
		in := true
		for _, pc := range c.states[s].tones {
			in = in && scale.Contains(pitch.PitchClass(pc))
		}

		c.inKey[s] = in
	}

	for w := range windows {
		c.score(&windows[w], c.scores[w*ns:(w+1)*ns])
	}

	viterbi(c.scores[:nw*ns], nw, ns, c.cfg.switchCost, c.acc, c.next, c.back[:nw*ns], c.path[:nw])

	out := c.collect(dst[:0], windows)
	if out == nil {
		out = []Chord{}
	}

	return out, nil
}

func validateWindow(i int, w *Window) error {
	if !finite(w.Start) || !finite(w.End) || w.Start > w.End {
		return fmt.Errorf("%w: window %d spans %g..%g s", ErrInvalidInput, i, w.Start, w.End)
	}

	if math.IsNaN(w.LevelDB) || math.IsInf(w.LevelDB, 1) {
		return fmt.Errorf("%w: window %d level is %g dBFS", ErrInvalidInput, i, w.LevelDB)
	}

	for pc := range pitchClasses {
		if !finite(w.Chroma[pc]) || w.Chroma[pc] < 0 || !finite(w.Bass[pc]) || w.Bass[pc] < 0 {
			return fmt.Errorf("%w: window %d pitch class %d has chroma %g, bass %g",
				ErrInvalidInput, i, pc, w.Chroma[pc], w.Bass[pc])
		}
	}

	return nil
}

func (c *Chorder) grow(nw int) {
	ns := len(c.states)
	if cap(c.scores) < nw*ns {
		c.scores = make([]float64, nw*ns)
		c.back = make([]int32, nw*ns)
	}

	if cap(c.path) < nw {
		c.path = make([]int32, nw)
		c.merged = make([]int, nw)
	}

	c.scores, c.back = c.scores[:cap(c.scores)], c.back[:cap(c.back)]
	c.path, c.merged = c.path[:cap(c.path)], c.merged[:cap(c.merged)]
}

// score fills row with the state scores of one window. The arithmetic and its
// order match AudioVisualizer's DetectChords.
func (c *Chorder) score(win *Window, row []float64) {
	p := &c.cfg

	maxC, mean, maxB := 0.0, 0.0, 0.0
	for i := range pitchClasses {
		maxC = math.Max(maxC, win.Chroma[i])
		mean += win.Chroma[i] / pitchClasses
		maxB = math.Max(maxB, win.Bass[i])
	}

	if win.LevelDB < p.gateDB || mean == 0 || maxC/mean < p.minPeakRatio {
		for s := range row {
			row[s] = math.Inf(-1)
		}

		row[0] = 1

		return
	}

	row[0] = math.Inf(-1)

	// Squared chroma norm, accumulated in pitch-class order like the
	// original cosine.
	na := 0.0
	for i := range pitchClasses {
		na += win.Chroma[i] * win.Chroma[i]
	}

	for s := 1; s < len(row); s++ {
		st := &c.states[s]

		// Cosine with the binary template. Zero template entries add exact
		// zeros in the original, so summing only the members in pitch-class
		// order gives the same bits.
		v := 0.0

		if na != 0 {
			dot := 0.0

			for i := range pitchClasses {
				if st.member[i] {
					dot += win.Chroma[i]
				}
			}

			v = dot / math.Sqrt(na*st.norm)
		}

		if maxB > 0 {
			bass := win.Bass[st.root]
			for _, pc := range st.tones[1:] {
				bass = math.Max(bass, p.inversionShare*win.Bass[pc])
			}

			v += p.bassWeight * bass / maxB
		}

		if st.seventh {
			v -= p.seventhCost
		}

		if c.inKey[s] {
			v += p.inKeyBonus
		}

		row[s] = v
	}
}

// collect turns the decoded path into merged chords appended to out.
func (c *Chorder) collect(out []Chord, windows []Window) []Chord {
	ns := len(c.states)

	for w := range windows {
		win := &windows[w]
		row := c.scores[w*ns : (w+1)*ns]
		state := int(c.path[w])
		st := &c.states[state]

		best, second := math.Inf(-1), math.Inf(-1)
		for _, v := range row {
			if v > best {
				best, second = v, best
			} else if v > second {
				second = v
			}
		}

		ch := Chord{Start: win.Start, End: win.End, NoChord: true, Quality: noChord, Symbol: noChord, Score: c.round(row[state])}
		if !math.IsInf(second, -1) {
			ch.Margin = c.round(best - second)
		}

		if st.root >= 0 {
			t := &c.cfg.templates[st.template]
			ch.NoChord = false
			ch.Root, ch.Quality, ch.Bass, ch.Symbol = pitch.PitchClass(st.root), t.Name, pitch.PitchClass(st.root), st.symbol

			bass, maxB := 0, 0.0
			for i, v := range win.Bass {
				if v > maxB {
					bass, maxB = i, v
				}
			}

			if maxB > 0 && st.slash[bass] != "" {
				ch.Bass, ch.Symbol = pitch.PitchClass(bass), st.slash[bass]
			}
		}

		if k := len(out) - 1; k >= 0 && out[k].Symbol == ch.Symbol {
			c.merged[k]++
			out[k].End = ch.End
			out[k].Score = c.round(out[k].Score + (ch.Score-out[k].Score)/float64(c.merged[k]))
			out[k].Margin = math.Min(out[k].Margin, ch.Margin)

			continue
		}

		// Reuse the Voicing storage of the slot this chord goes to.
		if len(out) < cap(out) {
			ch.Voicing = out[:len(out)+1][len(out)].Voicing[:0]
		}

		if !ch.NoChord {
			for _, iv := range c.cfg.templates[st.template].Intervals {
				ch.Voicing = append(ch.Voicing, VoicingBase+st.root+iv)
			}
		}

		c.merged[len(out)] = 1
		out = append(out, ch)
	}

	return out
}

func (c *Chorder) round(v float64) float64 {
	if c.cfg.decimals < 0 {
		return v
	}

	return math.Round(v*c.scale) / c.scale
}
