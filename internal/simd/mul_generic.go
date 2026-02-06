//go:build purego || !(amd64 || arm64)

// Package simd contains optional internal SIMD kernels and dispatch helpers.
package simd

// MulBlock performs element-wise multiplication: dst[i] = a[i] * b[i].
// Slices must have equal length. Panics if lengths differ.
// This is the pure Go fallback implementation.
func MulBlock(dst, a, b []float64) {
	if len(a) != len(b) || len(dst) != len(a) {
		panic("simd: slice length mismatch")
	}
	for i := range dst {
		dst[i] = a[i] * b[i]
	}
}

// MulBlockInPlace performs in-place element-wise multiplication: dst[i] *= src[i].
// Slices must have equal length. Panics if lengths differ.
// This is the pure Go fallback implementation.
func MulBlockInPlace(dst, src []float64) {
	if len(dst) != len(src) {
		panic("simd: slice length mismatch")
	}
	for i := range dst {
		dst[i] *= src[i]
	}
}

// ScaleBlock multiplies each element by a scalar: dst[i] = src[i] * scale.
// Slices must have equal length. Panics if lengths differ.
// This is the pure Go fallback implementation.
func ScaleBlock(dst, src []float64, scale float64) {
	if len(dst) != len(src) {
		panic("simd: slice length mismatch")
	}
	for i := range dst {
		dst[i] = src[i] * scale
	}
}

// ScaleBlockInPlace multiplies each element by a scalar in-place: dst[i] *= scale.
// This is the pure Go fallback implementation.
func ScaleBlockInPlace(dst []float64, scale float64) {
	for i := range dst {
		dst[i] *= scale
	}
}

// AddMulBlock performs fused add-multiply: dst[i] = (a[i] + b[i]) * scale.
// Slices must have equal length. Panics if lengths differ.
// This is the pure Go fallback implementation.
func AddMulBlock(dst, a, b []float64, scale float64) {
	if len(a) != len(b) || len(dst) != len(a) {
		panic("simd: slice length mismatch")
	}
	for i := range dst {
		dst[i] = (a[i] + b[i]) * scale
	}
}

// AddBlock performs element-wise addition: dst[i] = a[i] + b[i].
// Slices must have equal length. Panics if lengths differ.
// This is the pure Go fallback implementation.
func AddBlock(dst, a, b []float64) {
	if len(a) != len(b) || len(dst) != len(a) {
		panic("simd: slice length mismatch")
	}
	for i := range dst {
		dst[i] = a[i] + b[i]
	}
}

// AddBlockInPlace performs in-place element-wise addition: dst[i] += src[i].
// Slices must have equal length. Panics if lengths differ.
// This is the pure Go fallback implementation.
func AddBlockInPlace(dst, src []float64) {
	if len(dst) != len(src) {
		panic("simd: slice length mismatch")
	}
	for i := range dst {
		dst[i] += src[i]
	}
}

// MulAddBlock performs fused multiply-add: dst[i] = a[i] * b[i] + c[i].
// Slices must have equal length. Panics if lengths differ.
// This is the pure Go fallback implementation.
func MulAddBlock(dst, a, b, c []float64) {
	if len(a) != len(b) || len(dst) != len(a) || len(c) != len(a) {
		panic("simd: slice length mismatch")
	}
	for i := range dst {
		dst[i] = a[i]*b[i] + c[i]
	}
}
