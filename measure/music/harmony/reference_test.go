package harmony

// This file holds a verbatim copy of AudioVisualizer's
// internal/story/harmony.go (EstimateKey, DetectChords and their helpers) and
// of the window pooling helpers in internal/story/story.go (harmony, bassPC,
// meanDB, normalized) and grid.go (frames, r3, r6), adapted to standalone test
// code: the Grid, Track and Melody types are replaced by plain slices and a
// frame rate, DetectChords no longer takes the Grid (it only labelled Bar and
// BeatInBar, which are dropped) and also returns its score matrix so the
// parity tests can compare the unrounded scores, identifiers carry a ref
// prefix, and blank lines were added for the linter. The arithmetic and its operation order are
// unchanged; the parity tests compare this copy with the package bit for bit.

import (
	"fmt"
	"math"
	"sort"

	"github.com/cwbudde/algo-dsp/dsp/core"
)

var refPitchNames = [12]string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

// Krumhansl–Kessler probe-tone profiles, tonic first.
var (
	refMajorProfile = [12]float64{6.35, 2.23, 3.48, 2.33, 4.38, 4.09, 2.52, 5.19, 2.39, 3.66, 2.29, 2.88}
	refMinorProfile = [12]float64{6.33, 2.68, 3.52, 5.38, 2.60, 3.53, 2.54, 4.75, 3.98, 2.69, 3.34, 3.17}
)

func refR6(v float64) float64 { return math.Round(v*1e6) / 1e6 }
func refR3(v float64) float64 { return math.Round(v*1e3) / 1e3 }

type refKeyEstimate struct {
	Tonic               int
	Mode                string
	Name                string
	Correlation         float64
	RunnerUp            string
	RunnerUpCorrelation float64
	Relative            string
	RelativeCorrelation float64
	RelativeAmbiguous   bool
	RelativeEvidence    string
	Profile             [12]float64
}

type refKeyScore struct {
	tonic int
	minor bool
	r     float64
}

func refKeyName(tonic int, minor bool) string {
	if minor {
		return refPitchNames[tonic] + " minor"
	}

	return refPitchNames[tonic] + " major"
}

// Sharps returns the key signature (negative for flats) of the estimate.
func (k refKeyEstimate) Sharps() int {
	major := k.Tonic
	if k.Mode == "minor" {
		major = (k.Tonic + 3) % 12
	}

	s := major * 7 % 12
	if s > 6 {
		s -= 12
	}

	return s
}

// Diatonic reports whether pitch class pc belongs to the key's scale
// (natural minor for minor keys).
func (k refKeyEstimate) Diatonic(pc int) bool {
	steps := []int{0, 2, 4, 5, 7, 9, 11}
	if k.Mode == "minor" {
		steps = []int{0, 2, 3, 5, 7, 8, 10}
	}

	for _, s := range steps {
		if (k.Tonic+s)%12 == ((pc%12)+12)%12 {
			return true
		}
	}

	return false
}

func refPearson(a, b [12]float64) float64 {
	ma, mb := 0.0, 0.0
	for i := range a {
		ma += a[i] / 12
		mb += b[i] / 12
	}

	num, da, db := 0.0, 0.0, 0.0
	for i := range a {
		num += (a[i] - ma) * (b[i] - mb)
		da += (a[i] - ma) * (a[i] - ma)
		db += (b[i] - mb) * (b[i] - mb)
	}

	if da == 0 || db == 0 {
		return 0
	}

	return num / math.Sqrt(da*db)
}

// refEstimateKey correlates a pitch-class profile with all 24 rotated
// Krumhansl–Kessler profiles. When the best key and its relative differ by
// less than tie, tonicEvidence (for example bass weight at section edges)
// decides between them and the estimate is flagged ambiguous.
func refEstimateKey(pc [12]float64, tonicEvidence [12]float64, tie float64) refKeyEstimate {
	scores := []refKeyScore{}

	for tonic := range 12 {
		for _, minor := range []bool{false, true} {
			profile := refMajorProfile
			if minor {
				profile = refMinorProfile
			}

			var rot [12]float64
			for i := range rot {
				rot[(tonic+i)%12] = profile[i]
			}

			scores = append(scores, refKeyScore{tonic, minor, refPearson(pc, rot)})
		}
	}

	sort.SliceStable(scores, func(i, j int) bool { return scores[i].r > scores[j].r })
	best, runner := scores[0], scores[1]

	relTonic := (best.tonic + 9) % 12
	if best.minor {
		relTonic = (best.tonic + 3) % 12
	}

	k := refKeyEstimate{}

	for _, s := range scores[1:] {
		if s.tonic == relTonic && s.minor != best.minor && best.r-s.r < tie {
			k.RelativeAmbiguous = true
			k.RelativeEvidence = fmt.Sprintf("%s vs %s within %.2f; section-edge bass tonic weight %.3f vs %.3f", refKeyName(best.tonic, best.minor), refKeyName(s.tonic, s.minor), tie, tonicEvidence[best.tonic], tonicEvidence[s.tonic])

			if tonicEvidence[s.tonic] > tonicEvidence[best.tonic] {
				best, runner = s, best
			} else {
				runner = s
			}
		}
	}

	k.Tonic, k.Mode, k.Name, k.Correlation = best.tonic, "major", refKeyName(best.tonic, best.minor), refR3(best.r)
	if best.minor {
		k.Mode = "minor"
	}

	k.RunnerUp, k.RunnerUpCorrelation = refKeyName(runner.tonic, runner.minor), refR3(runner.r)

	rel := refKeyScore{(best.tonic + 9) % 12, true, 0}
	if best.minor {
		rel = refKeyScore{(best.tonic + 3) % 12, false, 0}
	}

	for _, s := range scores {
		if s.tonic == rel.tonic && s.minor == rel.minor {
			k.Relative, k.RelativeCorrelation = refKeyName(s.tonic, s.minor), refR3(s.r)
		}
	}

	sum := 0.0
	for _, v := range pc {
		sum += v
	}

	for i, v := range pc {
		if sum > 0 {
			k.Profile[i] = refR3(v / sum)
		}
	}

	return k
}

// refChordQuality is one template: intervals above the root.
type refChordQuality struct {
	Name, Suffix string
	Intervals    []int
}

var refQualities = []refChordQuality{
	{"maj", "", []int{0, 4, 7}},
	{"min", "m", []int{0, 3, 7}},
	{"7", "7", []int{0, 4, 7, 10}},
	{"maj7", "maj7", []int{0, 4, 7, 11}},
	{"m7", "m7", []int{0, 3, 7, 10}},
}

type refChordParams struct {
	BeatsPerChord  int
	BassWeight     float64
	InversionShare float64
	SeventhCost    float64
	InKeyBonus     float64
	MinPeakRatio   float64
	GateDB         float64
	SwitchCost     float64
}

// The spec's 0.3 root bonus let a bass on the third outvote the chroma (B7/D#
// read as D#m); 0.2 plus half credit for a bass on another chord tone fits
// inversions.
func refDefaultChordParams() refChordParams {
	return refChordParams{2, 0.2, 0.5, 0.03, 0.05, 1.5, -50, 0.10}
}

type refChord struct {
	Start   float64
	End     float64
	Root    int // pitch class, -1 for no chord
	Quality string
	Bass    int
	Symbol  string
	Score   float64
	Margin  float64
	Voicing []int
}

// refChordWindow is the evidence for one chord slot: harmony chroma, bass
// pitch-class weight and the harmony stem level.
type refChordWindow struct {
	Start, End float64
	Chroma     [12]float64
	Bass       [12]float64
	LevelDB    float64
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

// refDetectChords scores 12 roots × Qualities per window against the chroma,
// adds bass evidence (full for the root, InversionShare for another chord
// tone) and key evidence, and Viterbi-smooths with a switch penalty.
// Windows below the gate or with flat chroma are "N". Equal neighbours merge.
func refDetectChords(windows []refChordWindow, key refKeyEstimate, p refChordParams) ([]refChord, [][]float64) {
	type state struct{ root, quality int } // quality -1: N

	states := []state{{-1, -1}}

	for root := range 12 {
		for q := range refQualities {
			states = append(states, state{root, q})
		}
	}

	tones := func(s state) []int {
		out := []int{}
		for _, iv := range refQualities[s.quality].Intervals {
			out = append(out, (s.root+iv)%12)
		}

		return out
	}
	scores := make([][]float64, len(windows))

	for w, win := range windows {
		scores[w] = make([]float64, len(states))

		maxC, mean, maxB := 0.0, 0.0, 0.0
		for i := range 12 {
			maxC = math.Max(maxC, win.Chroma[i])
			mean += win.Chroma[i] / 12
			maxB = math.Max(maxB, win.Bass[i])
		}

		if win.LevelDB < p.GateDB || mean == 0 || maxC/mean < p.MinPeakRatio {
			for s := range states {
				scores[w][s] = math.Inf(-1)
			}

			scores[w][0] = 1

			continue
		}

		scores[w][0] = math.Inf(-1)

		for s, st := range states[1:] {
			var tpl [12]float64

			inKey := true

			for _, pc := range tones(st) {
				tpl[pc] = 1
				inKey = inKey && key.Diatonic(pc)
			}

			v := refCosine(win.Chroma[:], tpl[:])
			if maxB > 0 {
				bass := win.Bass[st.root]
				for _, pc := range tones(st)[1:] {
					bass = math.Max(bass, p.InversionShare*win.Bass[pc])
				}

				v += p.BassWeight * bass / maxB
			}

			if len(refQualities[st.quality].Intervals) > 3 {
				v -= p.SeventhCost
			}

			if inKey {
				v += p.InKeyBonus
			}

			scores[w][s+1] = v
		}
	}
	// Viterbi; ties keep the lower state index for determinism.
	path := make([]int, len(windows))
	if len(windows) > 0 {
		acc := append([]float64(nil), scores[0]...)
		back := make([][]int, len(windows))

		for w := 1; w < len(windows); w++ {
			back[w] = make([]int, len(states))
			next := make([]float64, len(states))

			bestPrev := 0

			for s := range states {
				if acc[s] > acc[bestPrev] {
					bestPrev = s
				}
			}

			for s := range states {
				stay, move := acc[s], acc[bestPrev]-p.SwitchCost
				if stay >= move {
					next[s], back[w][s] = stay+scores[w][s], s
				} else {
					next[s], back[w][s] = move+scores[w][s], bestPrev
				}
			}

			acc = next
		}

		last := 0

		for s := range states {
			if acc[s] > acc[last] {
				last = s
			}
		}

		for w := len(windows) - 1; w >= 0; w-- {
			path[w] = last
			if w > 0 {
				last = back[w][last]
			}
		}
	}

	chords, merged := []refChord{}, []int{}

	for w, win := range windows {
		st := states[path[w]]
		sorted := append([]float64(nil), scores[w]...)
		sort.Sort(sort.Reverse(sort.Float64Slice(sorted)))

		c := refChord{Start: refR6(win.Start), End: refR6(win.End), Root: -1, Bass: -1, Quality: "N", Symbol: "N", Score: refR3(scores[w][path[w]])}
		if len(sorted) > 1 && !math.IsInf(sorted[1], -1) {
			c.Margin = refR3(sorted[0] - sorted[1])
		}

		if st.root >= 0 {
			q := refQualities[st.quality]
			c.Root, c.Quality, c.Bass = st.root, q.Name, st.root
			c.Symbol = refPitchNames[st.root] + q.Suffix

			bass, maxB := 0, 0.0

			for i, v := range win.Bass {
				if v > maxB {
					bass, maxB = i, v
				}
			}

			for _, pc := range tones(st)[1:] {
				if maxB > 0 && pc == bass {
					c.Bass = bass
					c.Symbol += "/" + refPitchNames[bass]
				}
			}

			for _, iv := range q.Intervals {
				c.Voicing = append(c.Voicing, 48+st.root+iv)
			}
		}

		if k := len(chords) - 1; k >= 0 && chords[k].Symbol == c.Symbol {
			merged[k]++
			chords[k].End = c.End
			chords[k].Score = refR3(chords[k].Score + (c.Score-chords[k].Score)/float64(merged[k]))
			chords[k].Margin = math.Min(chords[k].Margin, c.Margin)

			continue
		}

		chords, merged = append(chords, c), append(merged, 1)
	}

	return chords, scores
}

// refFrames is the app's Grid.frames: the frames [lo, hi) of [start, end).
func refFrames(frameRate, start, end float64, n int) (int, int) {
	lo := min(n, max(0, int(math.Ceil(start*frameRate))))

	return lo, min(n, max(lo, int(math.Ceil(end*frameRate))))
}

// refDB is the app's audioanalysis.DB.
func refDB(v float64) float64 { return core.LinearToDBFloor(v, 1e-6) }

// refHarmony is the other-stem chroma over [t0, t1), each frame weighted by
// its RMS because the stored chroma is normalised per frame.
func refHarmony(rms []float64, chroma [12][]float64, frameRate, t0, t1 float64) [12]float64 {
	var c [12]float64

	lo, hi := refFrames(frameRate, t0, t1, len(rms))
	for i := lo; i < hi; i++ {
		for pc := range 12 {
			c[pc] += rms[i] * chroma[pc][i]
		}
	}

	return c
}

type refStoryNote struct {
	Start, End float64
	MIDI       int
	Strength   float64
}

// refBassPC is voicing-weighted bass chroma (in seconds) plus clean bass note
// overlap × strength.
func refBassPC(voicing []float64, chroma [12][]float64, frameRate, t0, t1 float64, notes []refStoryNote) [12]float64 {
	var c [12]float64

	lo, hi := refFrames(frameRate, t0, t1, len(voicing))
	for i := lo; i < hi; i++ {
		for pc := range 12 {
			c[pc] += voicing[i] * chroma[pc][i] / frameRate
		}
	}

	for _, n := range notes {
		if o := math.Min(n.End, t1) - math.Max(n.Start, t0); o > 0 {
			c[n.MIDI%12] += o * n.Strength
		}
	}

	return c
}

func refMeanDB(rms []float64, frameRate, t0, t1 float64) float64 {
	lo, hi := refFrames(frameRate, t0, t1, len(rms))
	sum := 0.0

	for i := lo; i < hi; i++ {
		sum += rms[i] * rms[i]
	}

	if hi <= lo {
		return refDB(0)
	}

	return refDB(math.Sqrt(sum / float64(hi-lo)))
}

func refNormalized(c [12]float64) [12]float64 {
	sum := 0.0
	for _, v := range c {
		sum += v
	}

	if sum > 0 {
		for i := range c {
			c[i] /= sum
		}
	}

	return c
}
