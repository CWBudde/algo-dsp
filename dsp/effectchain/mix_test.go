package effectchain

import (
	"math"
	"testing"
)

// mixReference is the scalar form mixParentEdgesInto had before it was routed
// through vecmath.
func mixReference(parents []compiledEdge, dst, mixBuf []float64, edgeSrc func(compiledEdge) []float64) {
	if len(parents) == 0 {
		for i := range dst {
			dst[i] = 0
		}

		return
	}

	if len(parents) == 1 {
		copy(dst, edgeSrc(parents[0]))
		return
	}

	for i := range mixBuf {
		mixBuf[i] = 0
	}

	for _, edge := range parents {
		src := edgeSrc(edge)
		for i := range mixBuf {
			mixBuf[i] += src[i]
		}
	}

	scale := 1.0 / float64(len(parents))
	for i := range mixBuf {
		dst[i] = mixBuf[i] * scale
	}
}

func mixTestSources(n, parents int) (map[string][]float64, []compiledEdge) {
	sources := make(map[string][]float64, parents)
	edges := make([]compiledEdge, parents)

	for p := range parents {
		id := string(rune('a' + p))
		buf := make([]float64, n)

		for i := range buf {
			buf[i] = math.Sin(float64(i)*0.11+float64(p)) * float64(p+1)
		}

		sources[id] = buf
		edges[p] = compiledEdge{From: id}
	}

	return sources, edges
}

// TestMixParentEdgesIntoMatchesReference checks the vecmath path against the
// scalar form it replaced, for the disjoint dst/mixBuf case.
func TestMixParentEdgesIntoMatchesReference(t *testing.T) {
	for _, parents := range []int{0, 1, 2, 3, 4, 7} {
		for _, n := range []int{1, 3, 32, 64, 128, 512} {
			sources, edges := mixTestSources(n, parents)
			edgeSrc := func(e compiledEdge) []float64 { return sources[e.From] }

			got, gotMix := make([]float64, n), make([]float64, n)
			want, wantMix := make([]float64, n), make([]float64, n)

			mixParentEdgesInto(edges, got, gotMix, edgeSrc)
			mixReference(edges, want, wantMix, edgeSrc)

			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("parents=%d n=%d index %d: got %v, want %v",
						parents, n, i, got[i], want[i])
				}
			}
		}
	}
}

// TestMixParentEdgesIntoAliasedDst covers the sidechain path, where
// processSidechainNode passes c.mixBuf[:len(dst)] as dst and c.mixBuf as mixBuf
// -- the same backing array. vecmath.ScaleBlock loads each vector before storing
// it, so exact aliasing is safe, but nothing else in the package pins that.
func TestMixParentEdgesIntoAliasedDst(t *testing.T) {
	for _, parents := range []int{2, 3, 5} {
		for _, n := range []int{1, 32, 128, 513} {
			sources, edges := mixTestSources(n, parents)
			edgeSrc := func(e compiledEdge) []float64 { return sources[e.From] }

			shared := make([]float64, n)
			mixParentEdgesInto(edges, shared, shared, edgeSrc)

			want, wantMix := make([]float64, n), make([]float64, n)
			mixReference(edges, want, wantMix, edgeSrc)

			for i := range shared {
				if shared[i] != want[i] {
					t.Fatalf("aliased parents=%d n=%d index %d: got %v, want %v",
						parents, n, i, shared[i], want[i])
				}
			}
		}
	}
}

// TestMixParentEdgesIntoAllocs pins the mix as allocation-free; it runs once per
// node per block on the audio path.
func TestMixParentEdgesIntoAllocs(t *testing.T) {
	sources, edges := mixTestSources(256, 3)
	edgeSrc := func(e compiledEdge) []float64 { return sources[e.From] }
	dst, mixBuf := make([]float64, 256), make([]float64, 256)

	got := testing.AllocsPerRun(100, func() {
		mixParentEdgesInto(edges, dst, mixBuf, edgeSrc)
	})
	if got != 0 {
		t.Errorf("mixParentEdgesInto allocated %v times per run, want 0", got)
	}
}
