package vecmath

import (
	"sync"

	"github.com/cwbudde/algo-dsp/internal/cpu"
	"github.com/cwbudde/algo-dsp/internal/vecmath/registry"
)

var (
	dotProductImpl     func([]float64, []float64) float64
	dotProductInitOnce sync.Once
)

func initDotProductOperation() {
	features := cpu.DetectFeatures()
	entry := registry.Global.Lookup(features)
	if entry == nil {
		panic("vecmath: no dotproduct implementation registered")
	}
	if entry.DotProduct == nil {
		panic("vecmath: selected implementation missing dotproduct operation")
	}
	dotProductImpl = entry.DotProduct
}

// DotProduct returns the dot product of a and b: sum(a[i] * b[i]).
// Returns 0 if slices are empty or have different lengths.
// Only the minimum length of the two slices is used.
func DotProduct(a, b []float64) float64 {
	dotProductInitOnce.Do(initDotProductOperation)
	return dotProductImpl(a, b)
}
