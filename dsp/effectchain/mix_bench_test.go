package effectchain

import (
	"strconv"
	"testing"
)

// BenchmarkMixParentEdgesInto covers the node-mixing path at the block sizes an
// audio callback actually uses. The small sizes are the ones that decide whether
// dispatching into vecmath beats an inlined scalar loop.
func BenchmarkMixParentEdgesInto(b *testing.B) {
	for _, parents := range []int{2, 3, 4} {
		for _, n := range []int{32, 64, 128, 512} {
			sources, edges := mixTestSources(n, parents)
			edgeSrc := func(e compiledEdge) []float64 { return sources[e.From] }
			dst, mixBuf := make([]float64, n), make([]float64, n)

			name := "parents=" + strconv.Itoa(parents) + "/n=" + strconv.Itoa(n)
			b.Run(name, func(b *testing.B) {
				b.ReportAllocs()

				for b.Loop() {
					mixParentEdgesInto(edges, dst, mixBuf, edgeSrc)
				}
			})
		}
	}
}
