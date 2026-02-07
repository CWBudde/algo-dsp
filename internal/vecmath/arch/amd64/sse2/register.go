//go:build amd64 && !purego

package sse2

import (
	"github.com/cwbudde/algo-dsp/internal/cpu"
	"github.com/cwbudde/algo-dsp/internal/vecmath/registry"
)

// init registers the SSE2-optimized implementations with the vecmath registry.
//
// SSE2 provides 128-bit SIMD operations and is part of the x86-64 baseline,
// so it's available on all amd64 CPUs.
//
// Currently only MaxAbs is implemented in SSE2. Other operations fall back to
// either AVX2 (if available) or generic implementations.
//
// Priority: 10 (medium - preferred over generic, but lower than AVX2)
func init() {
	registry.Global.Register(registry.OpEntry{
		Name:      "sse2",
		SIMDLevel: cpu.SIMDSSE2,
		Priority:  10,

		// Reduction operations
		MaxAbs: MaxAbs,

		// Note: Other operations (Add, Mul, Scale, Fused) are not implemented
		// in SSE2 yet. The registry will fall back to the next best implementation
		// (AVX2 or generic) for those operations.
	})
}
