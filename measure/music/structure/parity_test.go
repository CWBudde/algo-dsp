package structure

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

// appBoundaries is the adapter AudioVisualizer uses in place of its own
// Boundaries: positions are beat start times, marks are cue starts, and the
// rounding stays in the app.
func appBoundaries(novelty []float64, beatStart func(int) float64, barPosition func(float64) float64,
	cues []refCue, radius int, sigma float64,
) ([]refBoundary, error) {
	marks := make([]float64, len(cues))
	for i, c := range cues {
		marks[i] = c.Start
	}

	peaks, err := Peaks(novelty, radius, sigma, WithMarks(marks), WithPositions(beatStart))
	if err != nil {
		return nil, err
	}

	out := []refBoundary{}

	for _, p := range peaks {
		b := refBoundary{
			Time: refR6(p.Position), Beat: p.Index, BarPosition: refR3(barPosition(p.Position)),
			Novelty: refR3(p.Novelty), CueDelta: refR3(math.Inf(1)),
		}
		if p.Mark >= 0 {
			b.NearestCue, b.CueDelta = cues[p.Mark].Name, refR3(p.MarkOffset)
		}

		out = append(out, b)
	}

	return out, nil
}

// appLabel is the adapter AudioVisualizer uses in place of its own Label.
func appLabel(s [][]float64, size int, same, variant float64, lower bool) ([]string, []float64, error) {
	var opts []LabelOption
	if lower {
		opts = append(opts, WithLowercase())
	}

	phrases, err := Label(s, size, same, variant, opts...)
	if err != nil {
		return nil, nil, err
	}

	labels, toFirst := make([]string, len(phrases)), make([]float64, len(phrases))
	for i, p := range phrases {
		labels[i], toFirst[i] = p.Label, refR3(p.Similarity)
	}

	return labels, toFirst, nil
}

// song is a synthetic beat-level feature set shaped like the app's
// structure input: chroma (12), bass chroma (12), band dB (5) and stem dB (4)
// per beat, a 48-value drum grid per bar, and named cues.
type song struct {
	chroma, bass, bands, stems [][]float64
	drums                      [][]float64
	cues                       []refCue
	beatSeconds                float64
}

func (s song) beatStart(i int) float64       { return 0.25 + float64(i)*s.beatSeconds }
func (s song) barPosition(t float64) float64 { return (t - 0.25) / (4 * s.beatSeconds) }

func clone(rows [][]float64) [][]float64 {
	out := make([][]float64, len(rows))
	for i, r := range rows {
		out[i] = slices.Clone(r)
	}

	return out
}

// makeSong builds sections of bars (each a letter plus a perturbation
// level) from per-letter prototypes; a stem that never sounds gives a
// constant dimension and the silent intro beat a zero chroma row.
func makeSong(rng *rand.Rand, sections []byte, noise []float64) song {
	const barsPerSection = 4

	protos := map[byte][]float64{}
	proto := func(letter byte, dims int) []float64 {
		if p, ok := protos[letter]; ok && len(p) >= dims {
			return p[:dims]
		}

		p := make([]float64, 64)
		for i := range p {
			p[i] = rng.Float64()
		}

		protos[letter] = p

		return p[:dims]
	}

	s := song{beatSeconds: 60.0 / 105}
	beat := 0

	for k, letter := range sections {
		s.cues = append(s.cues, refCue{Name: string(rune(letter)) + string(rune('0'+k)), Start: s.beatStart(beat) + 0.07*float64(k%3-1)})

		for range barsPerSection {
			drum := make([]float64, 48)

			for pos := range 16 {
				if (pos%4 == 0) != (letter == 'B') {
					drum[pos*3] = 0.8 + 0.1*rng.Float64()
				}

				drum[pos*3+2] = 0.3 * rng.Float64()
			}

			s.drums = append(s.drums, drum)

			for range 4 {
				p := proto(letter, 33)
				row := make([]float64, 33)

				for i := range row {
					row[i] = p[i] + noise[k]*rng.NormFloat64()
				}

				if beat == 0 {
					clear(row[:12])
				}

				s.chroma = append(s.chroma, row[:12])
				s.bass = append(s.bass, row[12:24])
				band := row[24:29]

				for i := range band {
					band[i] = -30 + 20*band[i]
				}

				s.bands = append(s.bands, band)
				stem := row[29:33]
				stem[3] = -120 // silent vocals: a constant dimension
				s.stems = append(s.stems, stem)
				beat++
			}
		}
	}

	return s
}

type parityParams struct {
	halfWidth, radius, phrase int
	sigma, same, variant      float64
}

var defaultParity = parityParams{DefaultHalfWidth, DefaultPeakRadius, DefaultPhraseRows, DefaultPeakSigma, DefaultSame, DefaultVariant}

func (s song) beatBlocks(ref bool) ([]refBlock, []Block) {
	groups := []struct {
		rows   [][]float64
		weight float64
	}{{s.chroma, 1}, {s.bass, 0.7}, {s.bands, 0.5}, {s.stems, 0.5}}

	var (
		rb []refBlock
		lb []Block
	)

	for _, g := range groups {
		if ref {
			rb = append(rb, refBlock{values: clone(g.rows), weight: g.weight})
		} else {
			lb = append(lb, Block{Rows: clone(g.rows), Weight: g.weight})
		}
	}

	return rb, lb
}

// bars concatenates the four beat vectors of every bar and the assembled
// drum grid, as the app does.
func bars(beatVectors, drums [][]float64) [][]float64 {
	out := make([][]float64, len(drums))
	for b := range out {
		for k := range 4 {
			out[b] = append(out[b], beatVectors[4*b+k]...)
		}

		out[b] = append(out[b], drums[b]...)
	}

	return out
}

func requireBits(t *testing.T, what string, got, want []float64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s: length %d, want %d", what, len(got), len(want))
	}

	for i := range got {
		if math.Float64bits(got[i]) != math.Float64bits(want[i]) {
			t.Fatalf("%s[%d] = %v, want %v", what, i, got[i], want[i])
		}
	}
}

func requireMatrixBits(t *testing.T, what string, got, want [][]float64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s: %d rows, want %d", what, len(got), len(want))
	}

	for i := range got {
		requireBits(t, what, got[i], want[i])
	}
}

func runParity(t *testing.T, s song, p parityParams, cues []refCue) {
	t.Helper()

	// Reference pipeline (story.go structure()).
	rb, _ := s.beatBlocks(true)
	refBeats := refAssemble(rb)
	refBars := bars(refBeats, refAssemble([]refBlock{{values: clone(s.drums), weight: 0.5}}))
	refBeatSSM, refBarSSM := refSelfSimilarity(refBeats), refSelfSimilarity(refBars)
	refNovelty := refFooteNovelty(refBeatSSM, p.halfWidth)
	refB := refBoundaries(refNovelty, s.beatStart, s.barPosition, cues, p.radius, p.sigma)
	refPhrases, refToFirst := refLabel(refBarSSM, p.phrase, p.same, p.variant, false)
	refBarLabels, refBarToFirst := refLabel(refBarSSM, 1, p.same, p.variant, true)

	// Package pipeline.
	_, lb := s.beatBlocks(false)

	beats, err := Blocks(lb)
	if err != nil {
		t.Fatal(err)
	}

	drums, err := Blocks([]Block{{Rows: clone(s.drums), Weight: 0.5}})
	if err != nil {
		t.Fatal(err)
	}

	barRows := bars(beats, drums)

	beatSSM, err := SelfSimilarity(beats)
	if err != nil {
		t.Fatal(err)
	}

	barSSM, err := SelfSimilarity(barRows)
	if err != nil {
		t.Fatal(err)
	}

	novelty, err := FooteNovelty(beatSSM, p.halfWidth)
	if err != nil {
		t.Fatal(err)
	}

	boundaries, err := appBoundaries(novelty, s.beatStart, s.barPosition, cues, p.radius, p.sigma)
	if err != nil {
		t.Fatal(err)
	}

	phrases, toFirst, err := appLabel(barSSM, p.phrase, p.same, p.variant, false)
	if err != nil {
		t.Fatal(err)
	}

	barLabels, barToFirst, err := appLabel(barSSM, 1, p.same, p.variant, true)
	if err != nil {
		t.Fatal(err)
	}

	requireMatrixBits(t, "beat vectors", beats, refBeats)
	requireMatrixBits(t, "bar vectors", barRows, refBars)
	requireMatrixBits(t, "beat SSM", beatSSM, refBeatSSM)
	requireMatrixBits(t, "bar SSM", barSSM, refBarSSM)
	requireBits(t, "novelty", novelty, refNovelty)
	requireBits(t, "phrase similarity", toFirst, refToFirst)
	requireBits(t, "bar similarity", barToFirst, refBarToFirst)

	if !slices.Equal(boundaries, refB) {
		t.Fatalf("boundaries\n got %+v\nwant %+v", boundaries, refB)
	}

	if !slices.Equal(phrases, refPhrases) || !slices.Equal(barLabels, refBarLabels) {
		t.Fatalf("labels %v / %v, want %v / %v", phrases, barLabels, refPhrases, refBarLabels)
	}

	if len(refB) == 0 && len(cues) > 0 && p == defaultParity {
		t.Fatal("fixture has no boundaries; parity is not exercised")
	}
}

func TestStructureParity(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(45, 3))
	main := makeSong(rng, []byte("ABABCAB"), []float64{0.05, 0.05, 0.05, 0.25, 0.05, 0.4, 0.1})
	short := makeSong(rng, []byte("A"), []float64{0.1})

	cases := []struct {
		name   string
		s      song
		params parityParams
		cues   []refCue
	}{
		{"defaults", main, defaultParity, main.cues},
		{"no cues", main, defaultParity, nil},
		{"narrow kernel", main, parityParams{2, 1, 2, 0.5, 0.8, 0.5}, main.cues},
		{"wide kernel", main, parityParams{20, 10, 3, 0, 0.3, 0.1}, main.cues},
		{"shorter than kernel", short, defaultParity, short.cues},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			runParity(t, tc.s, tc.params, tc.cues)
		})
	}
}

func TestPeaksParityTies(t *testing.T) {
	t.Parallel()

	beatStart := func(i int) float64 { return 0.5 * float64(i) }
	barPosition := func(x float64) float64 { return x / 2 }
	cues := []refCue{{"a", 0}, {"b", 2}, {"c", 3}, {"d", 4}}

	for _, novelty := range [][]float64{
		{},
		{0.3},
		{0, 1, 1, 0, 0, 0.5, 0.5, 0.2, 1, 0},
		{0.2, 0.2, 0.2, 0.2},
		{1, 0, 0, 0, 0, 0, 0, 0, 0, 0.9},
	} {
		for _, radius := range []int{0, 1, 4} {
			got, err := appBoundaries(novelty, beatStart, barPosition, cues, radius, 0.5)
			if err != nil {
				t.Fatal(err)
			}

			want := refBoundaries(novelty, beatStart, barPosition, cues, radius, 0.5)
			if !slices.Equal(got, want) {
				t.Fatalf("novelty %v radius %d:\n got %+v\nwant %+v", novelty, radius, got, want)
			}
		}
	}
}

func TestLabelParityHandMade(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(7, 7))

	// Tie between letters A and B: the lower letter wins.
	tie := [][]float64{{1, 0.2, 0.5}, {0.2, 1, 0.5}, {0.5, 0.5, 1}}

	// Random matrix with more than 26 letters.
	many := make([][]float64, 60)
	for i := range many {
		many[i] = make([]float64, 60)
		for j := range many[i] {
			many[i][j] = 0.4 * math.Pow(rng.Float64(), 8)
		}
	}

	for _, s := range [][][]float64{tie, many, {}} {
		for _, size := range []int{1, 2, 7} {
			for _, lower := range []bool{false, true} {
				got, gotSim, err := appLabel(s, size, 0.6, 0.33, lower)
				if err != nil {
					t.Fatal(err)
				}

				want, wantSim := refLabel(s, size, 0.6, 0.33, lower)
				if !slices.Equal(got, want) {
					t.Fatalf("size %d lower %v: labels %v, want %v", size, lower, got, want)
				}

				requireBits(t, "similarity", gotSim, wantSim)

				if len(s) == 60 && size == 1 && !slices.Contains(got, "A1") && !slices.Contains(got, "a1") {
					t.Fatalf("fixture stays below 27 letters: %v", got)
				}
			}
		}
	}
}
