package structure_test

import (
	"math/rand/v2"
	"testing"

	"github.com/cwbudde/algo-dsp/measure/music/structure"
)

// benchRows is a 256-beat song of 33-dim beat vectors (the app's beat
// vector size) in sections of 32 beats.
func benchRows() [][]float64 {
	rng := rand.New(rand.NewPCG(45, 45))

	var rows [][]float64

	sections := [][][]float64{section(rng, 32, 33, 0.4), section(rng, 32, 33, 0.4), section(rng, 32, 33, 0.4)}
	for _, s := range []int{0, 1, 0, 1, 2, 0, 1, 2} {
		rows = append(rows, concat(sections[s])...)
	}

	return rows
}

func benchSSM(b *testing.B, rows [][]float64) [][]float64 {
	b.Helper()

	ssm, err := structure.SelfSimilarity(rows)
	if err != nil {
		b.Fatal(err)
	}

	return ssm
}

func BenchmarkBlocks(b *testing.B) {
	rows := benchRows()

	b.ReportAllocs()

	for b.Loop() {
		_, err := structure.Blocks([]structure.Block{
			{Rows: rows, Weight: 1},
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSelfSimilarityInto(b *testing.B) {
	rows := benchRows()
	dst := benchSSM(b, rows)

	b.ReportAllocs()

	for b.Loop() {
		err := structure.SelfSimilarityInto(dst, rows)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFooteNoveltyInto(b *testing.B) {
	ssm := benchSSM(b, benchRows())
	dst := make([]float64, len(ssm))

	b.ReportAllocs()

	for b.Loop() {
		err := structure.FooteNoveltyInto(dst, ssm, structure.DefaultHalfWidth)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPeaks(b *testing.B) {
	novelty, err := structure.FooteNovelty(benchSSM(b, benchRows()), structure.DefaultHalfWidth)
	if err != nil {
		b.Fatal(err)
	}

	marks := []float64{0, 32, 64, 96, 128, 160, 192, 224}

	b.ReportAllocs()

	for b.Loop() {
		_, err := structure.Peaks(novelty, structure.DefaultPeakRadius, structure.DefaultPeakSigma, structure.WithMarks(marks))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLabel(b *testing.B) {
	ssm := benchSSM(b, benchRows())

	b.ReportAllocs()

	for b.Loop() {
		_, err := structure.Label(ssm, 4, structure.DefaultSame, structure.DefaultVariant)
		if err != nil {
			b.Fatal(err)
		}
	}
}
