//go:build arm64 && !purego

package neon

import (
	"github.com/cwbudde/algo-dsp/internal/cpu"
	"github.com/cwbudde/algo-dsp/internal/vecmath/registry"
)

// init registers the NEON-optimized implementations with the vecmath registry.
//
// NEON (ARM Advanced SIMD) provides 128-bit SIMD operations and is mandatory
// on ARMv8 (arm64), so it's available on all arm64 CPUs. NEON processes 2 float64
// values at once.
//
// Priority: 15 (medium-high - ARM's equivalent to AVX/AVX2)
func init() {
	registry.Global.Register(registry.OpEntry{
		Name:      "neon",
		SIMDLevel: cpu.SIMDNEON,
		Priority:  15,

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
		MaxAbs: MaxAbs,
		Sum:    Sum,
	})
}
