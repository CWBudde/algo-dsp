//go:build arm64 && !purego

package neon

import (
	"github.com/cwbudde/algo-dsp/internal/cpu"
	"github.com/cwbudde/algo-dsp/internal/vecmath/registry"
)

// init registers the NEON-optimized implementations with the vecmath registry.
//
// NEON (ARM Advanced SIMD) provides 128-bit SIMD operations and is mandatory
// on ARMv8 (arm64), so it's available on all arm64 CPUs.
//
// Currently only MaxAbs is implemented in NEON. Other operations fall back to
// generic implementations.
//
// Priority: 15 (medium-high - ARM's equivalent to AVX/AVX2)
func init() {
	registry.Global.Register(registry.OpEntry{
		Name:      "neon",
		SIMDLevel: cpu.SIMDNEON,
		Priority:  15,

		// Reduction operations
		MaxAbs: MaxAbs,

		// Note: Other operations (Add, Mul, Scale, Fused) are not implemented
		// in NEON yet. The registry will fall back to generic implementations
		// for those operations on ARM.
	})
}
