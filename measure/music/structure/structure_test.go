package structure_test

import (
	"errors"
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/cwbudde/algo-dsp/measure/music/structure"
)

// randomUnit returns a random unit vector of dims values.
func randomUnit(rng *rand.Rand, dims int) []float64 {
	v := make([]float64, dims)
	norm := 0.0

	for i := range v {
		v[i] = rng.NormFloat64()
		norm += v[i] * v[i]
	}

	for i := range v {
		v[i] /= math.Sqrt(norm)
	}

	return v
}

// section returns n rows scattered around a random base vector; repeated
// sections reuse the returned rows.
func section(rng *rand.Rand, n, dims int, spread float64) [][]float64 {
	base := randomUnit(rng, dims)
	rows := make([][]float64, n)

	for i := range rows {
		noise := randomUnit(rng, dims)
		rows[i] = make([]float64, dims)

		for k := range rows[i] {
			rows[i][k] = base[k] + spread*noise[k]
		}
	}

	return rows
}

func concat(parts ...[][]float64) [][]float64 {
	var out [][]float64

	for _, p := range parts {
		for _, r := range p {
			out = append(out, slices.Clone(r))
		}
	}

	return out
}

// analyse runs the default pipeline: z-score, cosine SSM, novelty, peaks
// and phrase labels of phrase rows.
func analyse(t *testing.T, rows [][]float64, phrase int) ([]float64, []structure.Boundary, []structure.Phrase) {
	t.Helper()

	vectors, err := structure.Blocks([]structure.Block{{Rows: rows, Weight: 1}})
	if err != nil {
		t.Fatal(err)
	}

	ssm, err := structure.SelfSimilarity(vectors)
	if err != nil {
		t.Fatal(err)
	}

	novelty, err := structure.FooteNovelty(ssm, structure.DefaultHalfWidth)
	if err != nil {
		t.Fatal(err)
	}

	peaks, err := structure.Peaks(novelty, structure.DefaultPeakRadius, structure.DefaultPeakSigma)
	if err != nil {
		t.Fatal(err)
	}

	phrases, err := structure.Label(ssm, phrase, structure.DefaultSame, structure.DefaultVariant)
	if err != nil {
		t.Fatal(err)
	}

	return novelty, peaks, phrases
}

func indices(peaks []structure.Boundary) []int {
	out := make([]int, len(peaks))
	for i, p := range peaks {
		out[i] = p.Index
	}

	return out
}

func labels(phrases []structure.Phrase) []string {
	out := make([]string, len(phrases))
	for i, p := range phrases {
		out[i] = p.Label
	}

	return out
}

func TestABAB(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(1, 2))
	a, b := section(rng, 16, 24, 0.3), section(rng, 16, 24, 0.3)

	_, peaks, phrases := analyse(t, concat(a, b, a, b), 16)

	if got := indices(peaks); !slices.Equal(got, []int{16, 32, 48}) {
		t.Fatalf("boundaries at %v, want the joins 16, 32, 48 (%+v)", got, peaks)
	}

	if got := labels(phrases); !slices.Equal(got, []string{"A", "B", "A", "B"}) {
		t.Fatalf("labels %v, want A B A B (%+v)", got, phrases)
	}

	if phrases[2].Similarity != 1 || phrases[2].Variant || phrases[1].Letter != 1 {
		t.Fatalf("phrases %+v", phrases)
	}
}

func TestPerturbedPhraseIsVariant(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(3, 4))
	a, b := section(rng, 16, 24, 0.3), section(rng, 16, 24, 0.3)

	// The third phrase keeps A's base but its rows wander: mixing half of
	// each A row with an unrelated vector lowers its similarity to A into
	// the variant band.
	perturbed := make([][]float64, len(a))
	for i, row := range a {
		r := randomUnit(rng, len(row))
		perturbed[i] = make([]float64, len(row))

		for k := range row {
			perturbed[i][k] = 0.5*row[k] + 0.6*r[k]
		}
	}

	_, _, phrases := analyse(t, concat(a, b, perturbed, b), 16)

	if got := labels(phrases); !slices.Equal(got, []string{"A", "B", "A'", "B"}) {
		t.Fatalf("labels %v, want A B A' B (%+v)", got, phrases)
	}

	if s := phrases[2].Similarity; s < structure.DefaultVariant || s >= structure.DefaultSame || !phrases[2].Variant {
		t.Fatalf("variant phrase %+v", phrases[2])
	}
}

// TestLabelPhrases is the app's own label test: random unit rows, a
// repeated phrase, a variant at cosine 0.5 and a new phrase.
func TestLabelPhrases(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(1, 1))
	phrase := func() [][]float64 {
		out := [][]float64{}
		for range 4 {
			out = append(out, randomUnit(rng, 400))
		}

		return out
	}
	a, b, c := phrase(), phrase(), phrase()

	variant := [][]float64{}

	for _, v := range a {
		r, w := randomUnit(rng, 400), make([]float64, 400)
		for i := range w {
			w[i] = 0.5*v[i] + 0.866*r[i]
		}

		variant = append(variant, w)
	}

	ssm, err := structure.SelfSimilarity(slices.Concat(a, b, a, variant, c))
	if err != nil {
		t.Fatal(err)
	}

	phrases, err := structure.Label(ssm, 4, 0.6, 0.35, structure.WithPrime("′"))
	if err != nil {
		t.Fatal(err)
	}

	if got := labels(phrases); !slices.Equal(got, []string{"A", "B", "A", "A′", "C"}) {
		t.Fatalf("labels %v (%+v)", got, phrases)
	}

	bars, err := structure.Label(ssm, 1, 0.6, 0.35, structure.WithLowercase())
	if err != nil {
		t.Fatal(err)
	}

	if bars[0].Label != "a" || bars[8].Label != "a" || bars[12].Label != "a'" {
		t.Fatalf("bar labels %v", labels(bars))
	}
}

func TestConstantInput(t *testing.T) {
	t.Parallel()

	rows := make([][]float64, 40)
	for i := range rows {
		rows[i] = []float64{1, 2, 3}
	}

	novelty, peaks, phrases := analyse(t, rows, 4)

	if len(peaks) != 0 {
		t.Fatalf("boundaries %+v in constant input", peaks)
	}

	for i, v := range novelty {
		if v != 0 {
			t.Fatalf("novelty[%d] = %v", i, v)
		}
	}

	for _, p := range phrases {
		if math.IsNaN(p.Similarity) {
			t.Fatalf("NaN similarity in %+v", phrases)
		}
	}
}

func TestNoveltyPeaksAtStepChange(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(2, 2))
	u, v := randomUnit(rng, 64), randomUnit(rng, 64)

	rows := [][]float64{}

	for i := range 40 {
		base := u
		if i >= 23 {
			base = v
		}

		noise := randomUnit(rng, 64)
		row := make([]float64, 64)

		for k := range row {
			row[k] = base[k] + 0.2*noise[k]
		}

		rows = append(rows, row)
	}

	ssm, err := structure.SelfSimilarity(rows)
	if err != nil {
		t.Fatal(err)
	}

	n, err := structure.FooteNovelty(ssm, 8)
	if err != nil {
		t.Fatal(err)
	}

	peak := 0

	for i, x := range n {
		if x > n[peak] {
			peak = i
		}
	}

	if peak != 23 || n[peak] != 1 {
		t.Fatalf("novelty peak at %d (%v)", peak, n)
	}
}

func TestZScore(t *testing.T) {
	t.Parallel()

	rows := [][]float64{{1, 5}, {2, 5}, {3, 5}}

	err := structure.ZScore(rows)
	if err != nil {
		t.Fatal(err)
	}

	s := math.Sqrt(1.5)
	want := [][]float64{{-s, 0}, {0, 0}, {s, 0}}

	for i := range rows {
		for k := range rows[i] {
			if math.Abs(rows[i][k]-want[i][k]) > 1e-15 {
				t.Fatalf("rows %v, want %v", rows, want)
			}
		}
	}

	if err := structure.ZScore(nil); err != nil {
		t.Fatal(err)
	}

	ragged := [][]float64{{1, 2}, {3}}

	if err := structure.ZScore(ragged); !errors.Is(err, structure.ErrShape) {
		t.Fatalf("ragged rows: %v", err)
	}

	if ragged[0][0] != 1 {
		t.Fatal("ZScore modified rows it rejected")
	}
}

func TestBlocks(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(5, 5))
	chroma, bands := section(rng, 10, 12, 1), section(rng, 10, 5, 1)

	out, err := structure.Blocks([]structure.Block{{Rows: chroma, Weight: 1}, {Rows: bands, Weight: 0.5}})
	if err != nil {
		t.Fatal(err)
	}

	if len(out) != 10 || len(out[0]) != 17 {
		t.Fatalf("shape %d×%d", len(out), len(out[0]))
	}

	for _, row := range out {
		for _, part := range []struct {
			values []float64
			weight float64
		}{{row[:12], 1}, {row[12:], 0.5}} {
			norm := 0.0
			for _, x := range part.values {
				norm += x * x
			}

			if math.Abs(math.Sqrt(norm)-part.weight) > 1e-12 {
				t.Fatalf("block norm %v, want weight %v", math.Sqrt(norm), part.weight)
			}
		}
	}

	if out, err := structure.Blocks(nil); out != nil || err != nil {
		t.Fatalf("no blocks: %v, %v", out, err)
	}

	two := [][]float64{{1}, {2}}
	three := [][]float64{{1}, {2}, {3}}

	for _, tc := range []struct {
		blocks []structure.Block
		want   error
	}{
		{[]structure.Block{{Rows: two, Weight: 1}, {Rows: three, Weight: 1}}, structure.ErrShape},
		{[]structure.Block{{Rows: [][]float64{{1, 2}, {3}}, Weight: 1}}, structure.ErrShape},
		{[]structure.Block{{Rows: two, Weight: math.NaN()}}, structure.ErrInvalidArgument},
		{[]structure.Block{{Rows: two, Weight: math.Inf(1)}}, structure.ErrInvalidArgument},
	} {
		if _, err := structure.Blocks(tc.blocks); !errors.Is(err, tc.want) {
			t.Fatalf("blocks %+v: error %v, want %v", tc.blocks, err, tc.want)
		}
	}
}

func TestSelfSimilarity(t *testing.T) {
	t.Parallel()

	rows := [][]float64{{1, 0}, {0, 2}, {-3, 0}, {0, 0}}

	s, err := structure.SelfSimilarity(rows)
	if err != nil {
		t.Fatal(err)
	}

	want := [][]float64{{1, 0, -1, 0}, {0, 1, 0, 0}, {-1, 0, 1, 0}, {0, 0, 0, 0}}
	for i := range want {
		if !slices.Equal(s[i], want[i]) {
			t.Fatalf("ssm %v, want %v", s, want)
		}
	}

	dst := make([][]float64, 4)
	for i := range dst {
		dst[i] = make([]float64, 4)
	}

	if err := structure.SelfSimilarityInto(dst, rows); err != nil || !slices.Equal(dst[2], want[2]) {
		t.Fatalf("SelfSimilarityInto: %v, %v", dst, err)
	}

	if _, err := structure.SelfSimilarity([][]float64{{1}, {1, 2}}); !errors.Is(err, structure.ErrShape) {
		t.Fatalf("ragged rows: %v", err)
	}

	for _, bad := range [][][]float64{dst[:3], {dst[0], dst[1], dst[2], dst[3][:3]}} {
		if err := structure.SelfSimilarityInto(bad, rows); !errors.Is(err, structure.ErrShape) {
			t.Fatalf("destination %d rows: %v", len(bad), err)
		}
	}

	if err := structure.SelfSimilarityInto(dst, [][]float64{{1}, {1, 2}}); !errors.Is(err, structure.ErrShape) {
		t.Fatalf("ragged rows: %v", err)
	}
}

func TestFooteNoveltyErrorsAndAllocs(t *testing.T) {
	t.Parallel()

	ssm := [][]float64{{1, 0}, {0, 1}}

	for _, hw := range []int{0, -3} {
		if _, err := structure.FooteNovelty(ssm, hw); !errors.Is(err, structure.ErrInvalidArgument) {
			t.Fatalf("half-width %d: %v", hw, err)
		}
	}

	if _, err := structure.FooteNovelty([][]float64{{1, 0}}, 2); !errors.Is(err, structure.ErrShape) {
		t.Fatalf("non-square: %v", err)
	}

	if err := structure.FooteNoveltyInto(make([]float64, 3), ssm, 2); !errors.Is(err, structure.ErrShape) {
		t.Fatalf("short destination: %v", err)
	}

	if err := structure.FooteNoveltyInto(make([]float64, 2), ssm, 0); !errors.Is(err, structure.ErrInvalidArgument) {
		t.Fatalf("half-width 0: %v", err)
	}

	out, err := structure.FooteNovelty(nil, 4)
	if err != nil || len(out) != 0 {
		t.Fatalf("empty: %v, %v", out, err)
	}
}

// TestIntoZeroAllocs is not parallel: AllocsPerRun refuses parallel tests.
func TestIntoZeroAllocs(t *testing.T) {
	rng := rand.New(rand.NewPCG(6, 6))
	rows := section(rng, 32, 12, 0.5)

	ssm := make([][]float64, len(rows))
	for i := range ssm {
		ssm[i] = make([]float64, len(rows))
	}

	novelty := make([]float64, len(rows))

	allocs := testing.AllocsPerRun(10, func() {
		if err := structure.SelfSimilarityInto(ssm, rows); err != nil {
			t.Fatal(err)
		}

		if err := structure.FooteNoveltyInto(novelty, ssm, 8); err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 0 {
		t.Fatalf("SelfSimilarityInto + FooteNoveltyInto allocate %v times", allocs)
	}
}

func TestPeaks(t *testing.T) {
	t.Parallel()

	novelty := []float64{0, 0.1, 1, 0.2, 0, 0.1, 0.9, 0.9, 0.1, 0}

	peaks, err := structure.Peaks(novelty, 2, 1,
		structure.WithPositions(func(i int) float64 { return 0.5 * float64(i) }),
		structure.WithMarks([]float64{0.75, 3.25, 3.5}))
	if err != nil {
		t.Fatal(err)
	}

	// Of the equal values at 6 and 7 only the earlier counts.
	want := []structure.Boundary{
		{Index: 2, Novelty: 1, Position: 1, Mark: 0, MarkOffset: 0.25},
		{Index: 6, Novelty: 0.9, Position: 3, Mark: 1, MarkOffset: -0.25},
	}
	if !slices.Equal(peaks, want) {
		t.Fatalf("peaks %+v, want %+v", peaks, want)
	}

	plain, err := structure.Peaks(novelty, 2, 1)
	if err != nil {
		t.Fatal(err)
	}

	if len(plain) != 2 || plain[1].Position != 6 || plain[1].Mark != -1 || plain[1].MarkOffset != 0 {
		t.Fatalf("peaks without options %+v", plain)
	}

	// Equal distance to two marks: the earlier mark wins.
	tie, err := structure.Peaks(novelty, 2, 1, structure.WithMarks([]float64{1, 3}))
	if err != nil {
		t.Fatal(err)
	}

	if tie[0].Mark != 0 || tie[0].MarkOffset != 1 {
		t.Fatalf("tie %+v", tie[0])
	}

	for _, none := range [][]float64{nil, {0.4, 0.4, 0.4}} {
		if p, err := structure.Peaks(none, 4, 1); err != nil || len(p) != 0 {
			t.Fatalf("novelty %v: %+v, %v", none, p, err)
		}
	}

	for _, tc := range []struct {
		radius int
		sigma  float64
		opts   []structure.PeakOption
		want   error
	}{
		{-1, 1, nil, structure.ErrInvalidArgument},
		{1, math.NaN(), nil, structure.ErrInvalidArgument},
		{1, math.Inf(-1), nil, structure.ErrInvalidArgument},
		{1, 1, []structure.PeakOption{nil}, structure.ErrNilOption},
		{1, 1, []structure.PeakOption{structure.WithPositions(nil)}, structure.ErrInvalidArgument},
		{1, 1, []structure.PeakOption{structure.WithMarks([]float64{1, math.NaN()})}, structure.ErrInvalidArgument},
	} {
		if _, err := structure.Peaks(novelty, tc.radius, tc.sigma, tc.opts...); !errors.Is(err, tc.want) {
			t.Fatalf("radius %d sigma %v: error %v, want %v", tc.radius, tc.sigma, err, tc.want)
		}
	}
}

func TestLabelLettersAndErrors(t *testing.T) {
	t.Parallel()

	// An identity matrix: every unit is new.
	n := 30
	ssm := make([][]float64, n)

	for i := range ssm {
		ssm[i] = make([]float64, n)
		ssm[i][i] = 1
	}

	phrases, err := structure.Label(ssm, 1, 0.6, 0.35)
	if err != nil {
		t.Fatal(err)
	}

	if phrases[25].Label != "Z" || phrases[26].Label != "A1" || phrases[29].Label != "D1" || phrases[29].Letter != 29 {
		t.Fatalf("labels %v", labels(phrases))
	}

	// A short last unit compares only its rows.
	short, err := structure.Label([][]float64{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}, 2, 0.6, 0.35)
	if err != nil {
		t.Fatal(err)
	}

	if len(short) != 2 {
		t.Fatalf("units %+v", short)
	}

	for _, tc := range []struct {
		ssm     [][]float64
		unit    int
		same    float64
		variant float64
		opts    []structure.LabelOption
		want    error
	}{
		{ssm, 0, 0.6, 0.35, nil, structure.ErrInvalidArgument},
		{ssm, 1, math.NaN(), 0.35, nil, structure.ErrInvalidArgument},
		{ssm, 1, 0.6, math.Inf(1), nil, structure.ErrInvalidArgument},
		{[][]float64{{1, 0}}, 1, 0.6, 0.35, nil, structure.ErrShape},
		{ssm, 1, 0.6, 0.35, []structure.LabelOption{nil}, structure.ErrNilOption},
		{ssm, 1, 0.6, 0.35, []structure.LabelOption{structure.WithPrime("")}, structure.ErrInvalidArgument},
	} {
		if _, err := structure.Label(tc.ssm, tc.unit, tc.same, tc.variant, tc.opts...); !errors.Is(err, tc.want) {
			t.Fatalf("unit %d same %v variant %v: error %v, want %v", tc.unit, tc.same, tc.variant, err, tc.want)
		}
	}
}

func TestDeterministic(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(9, 9))
	a, b := section(rng, 16, 24, 0.5), section(rng, 16, 24, 0.5)
	rows := concat(a, b, a, b)

	n1, p1, l1 := analyse(t, concat(rows), 16)
	n2, p2, l2 := analyse(t, concat(rows), 16)

	if !slices.Equal(n1, n2) || !slices.Equal(p1, p2) || !slices.Equal(l1, l2) {
		t.Fatal("repeated analysis differs")
	}
}

// TestNonFiniteInputRejected checks that NaN and ±Inf inputs give
// ErrInvalidArgument instead of propagating into the novelty and peaks.
func TestNonFiniteInputRejected(t *testing.T) {
	t.Parallel()

	for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := structure.Peaks([]float64{0, 1, bad, 0.2}, 1, 1); !errors.Is(err, structure.ErrInvalidArgument) {
			t.Fatalf("Peaks with %v: %v", bad, err)
		}

		rows := [][]float64{{1, 0}, {0, bad}, {1, 1}}
		if _, err := structure.SelfSimilarity(rows); !errors.Is(err, structure.ErrInvalidArgument) {
			t.Fatalf("SelfSimilarity with %v: %v", bad, err)
		}

		dst := [][]float64{make([]float64, 3), make([]float64, 3), make([]float64, 3)}
		if err := structure.SelfSimilarityInto(dst, rows); !errors.Is(err, structure.ErrInvalidArgument) {
			t.Fatalf("SelfSimilarityInto with %v: %v", bad, err)
		}

		ssm := [][]float64{{1, 0}, {bad, 1}}
		if _, err := structure.FooteNovelty(ssm, 1); !errors.Is(err, structure.ErrInvalidArgument) {
			t.Fatalf("FooteNovelty with %v: %v", bad, err)
		}

		if err := structure.FooteNoveltyInto(make([]float64, 2), ssm, 1); !errors.Is(err, structure.ErrInvalidArgument) {
			t.Fatalf("FooteNoveltyInto with %v: %v", bad, err)
		}
	}

	// A ragged matrix still reports the shape first.
	if _, err := structure.SelfSimilarity([][]float64{{1, math.NaN()}, {1}}); !errors.Is(err, structure.ErrShape) {
		t.Fatalf("ragged rows: %v", err)
	}
}

// TestLabelBeyond260Letters checks that letter names stay unique past Z9:
// the 261st letter is A10, not A followed by ':'.
func TestLabelBeyond260Letters(t *testing.T) {
	t.Parallel()

	n := 290
	ssm := make([][]float64, n)

	for i := range ssm {
		ssm[i] = make([]float64, n)
		ssm[i][i] = 1
	}

	phrases, err := structure.Label(ssm, 1, 0.6, 0.35, structure.WithLowercase())
	if err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}

	for i, p := range phrases {
		if p.Letter != i || seen[p.Label] {
			t.Fatalf("phrase %d: letter %d label %q (duplicate %v)", i, p.Letter, p.Label, seen[p.Label])
		}

		seen[p.Label] = true
	}

	for i, want := range map[int]string{0: "a", 25: "z", 26: "a1", 259: "z9", 260: "a10", 285: "z10", 286: "a11"} {
		if phrases[i].Label != want {
			t.Fatalf("phrase %d: label %q, want %q", i, phrases[i].Label, want)
		}
	}
}
