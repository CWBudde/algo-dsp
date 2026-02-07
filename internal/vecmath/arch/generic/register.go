package generic

import (
	"github.com/cwbudde/algo-dsp/internal/cpu"
	"github.com/cwbudde/algo-dsp/internal/vecmath/registry"
)

// init registers the generic (pure Go) implementations with the vecmath registry.
//
// Generic implementations serve as the baseline fallback when no SIMD optimizations
// are available or when ForceGeneric is enabled for testing.
//
// Priority: 0 (lowest - used only when no SIMD alternatives are available)
func init() {
	registry.Global.Register(registry.OpEntry{
		Name:      "generic",
		SIMDLevel: cpu.SIMDNone,
		Priority:  0,

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
	})
}
