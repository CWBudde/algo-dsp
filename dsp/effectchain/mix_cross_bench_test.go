package effectchain

import (
	"strconv"
	"testing"
)

// BenchmarkMixReference measures the scalar form mixParentEdgesInto had before
// vecmath, in the same binary as BenchmarkMixParentEdgesInto.
func BenchmarkMixReference(b *testing.B) {
	benchMix(b, mixReference)
}

func BenchmarkMixVecmath(b *testing.B) {
	benchMix(b, mixParentEdgesInto)
}

func benchMix(b *testing.B, fn func([]compiledEdge, []float64, []float64, func(compiledEdge) []float64)) {
	b.Helper()

	for _, parents := range []int{2, 4} {
		for _, n := range []int{16, 32, 64, 128, 256, 512} {
			sources, edges := mixTestSources(n, parents)
			edgeSrc := func(e compiledEdge) []float64 { return sources[e.From] }
			dst, mixBuf := make([]float64, n), make([]float64, n)

			b.Run("p"+strconv.Itoa(parents)+"/"+strconv.Itoa(n), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					fn(edges, dst, mixBuf, edgeSrc)
				}
			})
		}
	}
}
