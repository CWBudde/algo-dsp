package vecmath

import (
	"sync"

	"github.com/cwbudde/algo-dsp/internal/cpu"
	"github.com/cwbudde/algo-dsp/internal/vecmath/registry"
)

var (
	// Cached function pointers for add operations (initialized once, used many times)
	addBlockImpl        func([]float64, []float64, []float64)
	addBlockInPlaceImpl func([]float64, []float64)
	addInitOnce         sync.Once
)

// initAddOperations performs one-time initialization of add operation function pointers.
// This function selects the best implementation based on detected CPU features and
// caches the function pointers for subsequent calls.
func initAddOperations() {
	features := cpu.DetectFeatures()
	entry := registry.Global.Lookup(features)

	if entry == nil {
		panic("vecmath: no add implementation registered (missing generic fallback?)")
	}

	if entry.AddBlock == nil || entry.AddBlockInPlace == nil {
		panic("vecmath: selected implementation missing add operations")
	}

	addBlockImpl = entry.AddBlock
	addBlockInPlaceImpl = entry.AddBlockInPlace
}

// AddBlock performs element-wise addition: dst[i] = a[i] + b[i].
//
// All slices must have equal length. Panics if lengths differ.
//
// The implementation is automatically selected based on CPU features:
//   - AVX2 on x86-64 CPUs with AVX2 support (Haswell 2013+)
//   - Generic pure Go fallback otherwise
//
// After the first call, subsequent calls have zero dispatch overhead
// (direct function pointer call, equivalent to hand-written dispatch).
func AddBlock(dst, a, b []float64) {
	addInitOnce.Do(initAddOperations)
	addBlockImpl(dst, a, b)
}

// AddBlockInPlace performs in-place element-wise addition: dst[i] += src[i].
//
// Both slices must have equal length. Panics if lengths differ.
//
// The implementation is automatically selected based on CPU features:
//   - AVX2 on x86-64 CPUs with AVX2 support (Haswell 2013+)
//   - Generic pure Go fallback otherwise
//
// After the first call, subsequent calls have zero dispatch overhead
// (direct function pointer call, equivalent to hand-written dispatch).
func AddBlockInPlace(dst, src []float64) {
	addInitOnce.Do(initAddOperations)
	addBlockInPlaceImpl(dst, src)
}
