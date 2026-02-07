//go:build amd64 && !purego

package avx2

import (
	"github.com/cwbudde/algo-dsp/internal/cpu"
	"github.com/cwbudde/algo-dsp/internal/vecmath/registry"
)

// init registers the AVX2-optimized implementations with the vecmath registry.
//
// AVX2 provides 256-bit SIMD operations with improved integer and floating-point
// performance compared to SSE2. Available on Intel Haswell (2013+) and AMD Excavator (2015+).
//
// Priority: 20 (high - preferred over SSE2 and generic when available)
func init() {
	registry.Global.Register(registry.OpEntry{
		Name:      "avx2",
		SIMDLevel: cpu.SIMDAVX2,
		Priority:  20,

		// Arithmetic operations
		AddBlock:          AddBlock,
		AddBlockInPlace:   AddBlockInPlace,
		MulBlock:          MulBlock,
		MulBlockInPlace:   MulBlockInPlace,
		ScaleBlock:        ScaleBlock,
		ScaleBlockInPlace: ScaleBlockInPlace,

		// Fused operations
		AddMulBlock: AddMulBlock,
		MulAddBlock: MulAddBlock,

		// Reduction operations
		MaxAbs:     MaxAbs,
		Sum:        Sum,
		DotProduct: DotProduct,
	})
}
