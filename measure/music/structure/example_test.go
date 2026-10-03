package structure_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/measure/music/structure"
)

// abab returns four sections of n rows each, A B A B, in two dimensions:
// A points one way, B the opposite way, with a small deterministic wobble.
func abab(n int) [][]float64 {
	var rows [][]float64

	for s := range 4 {
		for i := range n {
			wobble := 0.2 * math.Sin(float64(3*i+s))
			if s%2 == 0 {
				rows = append(rows, []float64{1, -1 + wobble})
			} else {
				rows = append(rows, []float64{-1 + wobble, 1})
			}
		}
	}

	return rows
}

func Example() {
	vectors, err := structure.Blocks([]structure.Block{{Rows: abab(8), Weight: 1}})
	if err != nil {
		panic(err)
	}

	ssm, err := structure.SelfSimilarity(vectors)
	if err != nil {
		panic(err)
	}

	novelty, err := structure.FooteNovelty(ssm, 4)
	if err != nil {
		panic(err)
	}

	peaks, err := structure.Peaks(novelty, structure.DefaultPeakRadius, structure.DefaultPeakSigma)
	if err != nil {
		panic(err)
	}

	phrases, err := structure.Label(ssm, 8, structure.DefaultSame, structure.DefaultVariant)
	if err != nil {
		panic(err)
	}

	for _, b := range peaks {
		fmt.Printf("boundary before row %d (novelty %.2f)\n", b.Index, b.Novelty)
	}

	for _, p := range phrases {
		fmt.Print(p.Label, " ")
	}

	fmt.Println()

	// Output:
	// boundary before row 8 (novelty 1.00)
	// boundary before row 16 (novelty 1.00)
	// boundary before row 24 (novelty 1.00)
	// A B A B
}

func ExampleZScore() {
	rows := [][]float64{{1, 7}, {2, 7}, {3, 7}}

	err := structure.ZScore(rows)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%.3f\n", rows)

	// Output:
	// [[-1.225 0.000] [0.000 0.000] [1.225 0.000]]
}

func ExampleBlocks() {
	chroma := [][]float64{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}, {1, 0, 0}}
	levels := [][]float64{{-20}, {-10}, {-20}, {-30}}

	rows, err := structure.Blocks([]structure.Block{
		{Rows: chroma, Weight: 1},
		{Rows: levels, Weight: 0.5},
	})
	if err != nil {
		panic(err)
	}

	for _, r := range rows {
		fmt.Printf("%5.2f\n", r)
	}

	// Output:
	// [ 0.77 -0.45 -0.45  0.00]
	// [-0.48  0.83 -0.28  0.50]
	// [-0.48 -0.28  0.83  0.00]
	// [ 0.77 -0.45 -0.45 -0.50]
}

func ExampleBlock() {
	// The app's beat vector: chroma, bass chroma, band dB and stem dB,
	// weighted 1, 0.7, 0.5 and 0.5. Here with two beats of toy data.
	blocks := []structure.Block{
		{Rows: [][]float64{{1, 0}, {0, 1}}, Weight: 1},
		{Rows: [][]float64{{1, 0}, {1, 0}}, Weight: 0.7}, // constant: contributes 0
		{Rows: [][]float64{{-20}, {-10}}, Weight: 0.5},
		{Rows: [][]float64{{-40}, {-12}}, Weight: 0.5},
	}

	rows, err := structure.Blocks(blocks)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%.3f\n", rows)

	// Output:
	// [[0.707 -0.707 0.000 0.000 -0.500 -0.500] [-0.707 0.707 0.000 0.000 0.500 0.500]]
}

func ExampleSelfSimilarity() {
	ssm, err := structure.SelfSimilarity([][]float64{{1, 0}, {1, 1}, {0, 2}})
	if err != nil {
		panic(err)
	}

	fmt.Printf("%.3f\n", ssm)

	// Output:
	// [[1.000 0.707 0.000] [0.707 1.000 0.707] [0.000 0.707 1.000]]
}

func ExampleSelfSimilarityInto() {
	rows := [][]float64{{1, 0}, {-1, 0}}

	dst := [][]float64{make([]float64, 2), make([]float64, 2)}

	err := structure.SelfSimilarityInto(dst, rows)
	if err != nil {
		panic(err)
	}

	fmt.Println(dst)

	// Output:
	// [[1 -1] [-1 1]]
}

func ExampleFooteNovelty() {
	ssm, err := structure.SelfSimilarity(abab(4))
	if err != nil {
		panic(err)
	}

	novelty, err := structure.FooteNovelty(ssm, 2)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%.2f\n", novelty)

	// Output:
	// [0.25 0.02 0.00 0.07 1.00 0.08 0.00 0.07 1.00 0.07 0.00 0.08 1.00 0.07 0.00 0.02]
}

func ExampleFooteNoveltyInto() {
	ssm, err := structure.SelfSimilarity(abab(4))
	if err != nil {
		panic(err)
	}

	novelty := make([]float64, len(ssm)) // reuse across calls

	err = structure.FooteNoveltyInto(novelty, ssm, 2)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%.2f\n", novelty[3:6])

	// Output:
	// [0.07 1.00 0.08]
}

func ExamplePeaks() {
	novelty := []float64{0.1, 0.2, 0.9, 0.3, 0.1, 0.1, 0.2, 1.0, 0.4, 0.1}

	peaks, err := structure.Peaks(novelty, 2, 1)
	if err != nil {
		panic(err)
	}

	for _, p := range peaks {
		fmt.Printf("index %d novelty %.1f\n", p.Index, p.Novelty)
	}

	// Output:
	// index 2 novelty 0.9
	// index 7 novelty 1.0
}

func ExampleWithMarks() {
	novelty := []float64{0.1, 0.2, 0.9, 0.3, 0.1, 0.1, 0.2, 1.0, 0.4, 0.1}
	cues := []string{"intro", "verse", "chorus"}
	cueStarts := []float64{0, 1.1, 3.3} // seconds

	peaks, err := structure.Peaks(novelty, 2, 1,
		structure.WithPositions(func(beat int) float64 { return 0.5 * float64(beat) }),
		structure.WithMarks(cueStarts))
	if err != nil {
		panic(err)
	}

	for _, p := range peaks {
		fmt.Printf("%.1f s: nearest cue %s, %+.1f s\n", p.Position, cues[p.Mark], p.MarkOffset)
	}

	// Output:
	// 1.0 s: nearest cue verse, -0.1 s
	// 3.5 s: nearest cue chorus, +0.2 s
}

func ExampleWithPositions() {
	novelty := []float64{0, 1, 0, 0, 0}

	// Beat i starts at 0.25 s + i·0.5 s.
	peaks, err := structure.Peaks(novelty, 1, 1,
		structure.WithPositions(func(beat int) float64 { return 0.25 + 0.5*float64(beat) }))
	if err != nil {
		panic(err)
	}

	fmt.Println(peaks[0].Index, peaks[0].Position)

	// Output:
	// 1 0.75
}

func ExampleLabel() {
	ssm, err := structure.SelfSimilarity(abab(8))
	if err != nil {
		panic(err)
	}

	phrases, err := structure.Label(ssm, 8, 0.6, 0.35)
	if err != nil {
		panic(err)
	}

	for _, p := range phrases {
		fmt.Printf("%s %.2f\n", p.Label, p.Similarity)
	}

	// Output:
	// A 1.00
	// B 1.00
	// A 0.99
	// B 1.00
}

func ExampleWithLowercase() {
	ssm, err := structure.SelfSimilarity(abab(2))
	if err != nil {
		panic(err)
	}

	bars, err := structure.Label(ssm, 1, 0.6, 0.35, structure.WithLowercase())
	if err != nil {
		panic(err)
	}

	for _, b := range bars {
		fmt.Print(b.Label, " ")
	}

	fmt.Println()

	// Output:
	// a a b b a a b b
}

func ExampleWithPrime() {
	// Unit 2 resembles unit 0 with cosine 0.5: a variant.
	ssm := [][]float64{
		{1, 0, 0.5},
		{0, 1, 0},
		{0.5, 0, 1},
	}

	phrases, err := structure.Label(ssm, 1, 0.6, 0.35, structure.WithPrime("′"))
	if err != nil {
		panic(err)
	}

	for _, p := range phrases {
		fmt.Print(p.Label, " ")
	}

	fmt.Println()

	// Output:
	// A B A′
}
