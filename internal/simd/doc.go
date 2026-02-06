// Package simd contains optional internal SIMD kernels and dispatch helpers.
//
// This package provides SIMD-optimized implementations of common DSP operations
// with pure Go fallbacks for portability. The optimized paths are automatically
// selected based on build tags:
//
//   - Default (amd64): Uses AVX2 SIMD instructions
//   - purego tag: Uses pure Go scalar implementation
//   - Other architectures: Uses pure Go fallback
//
// # Block Operations
//
// The package provides element-wise arithmetic operations commonly used
// in DSP for window application, mixing, and signal processing:
//
// Multiplication:
//   - MulBlock: dst[i] = a[i] * b[i]
//   - MulBlockInPlace: dst[i] *= src[i]
//   - ScaleBlock: dst[i] = src[i] * scale
//   - ScaleBlockInPlace: dst[i] *= scale
//
// Addition:
//   - AddBlock: dst[i] = a[i] + b[i]
//   - AddBlockInPlace: dst[i] += src[i]
//
// Fused operations (reduced memory traffic):
//   - AddMulBlock: dst[i] = (a[i] + b[i]) * scale (mix with gain)
//   - MulAddBlock: dst[i] = a[i] * b[i] + c[i] (FMA pattern)
//
// All operations have zero allocations and are safe for concurrent use
// (different goroutines may operate on different slices).
//
// # Performance
//
// On AMD64 with AVX2, these operations achieve 2-5x speedup over scalar Go code,
// processing 4 float64 values per SIMD instruction.
package simd
