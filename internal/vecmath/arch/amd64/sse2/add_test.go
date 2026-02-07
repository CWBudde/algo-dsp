//go:build amd64 && !purego

package sse2

import (
	"fmt"
	"testing"
)

// TestAddBlock_SSE2 tests the SSE2 implementation directly
func TestAddBlock_SSE2(t *testing.T) {
	sizes := []int{0, 1, 2, 3, 4, 5, 7, 8, 15, 16, 17, 32, 64, 100, 1000}

	for _, n := range sizes {
		t.Run(sizeStr(n), func(t *testing.T) {
			a := make([]float64, n)
			b := make([]float64, n)
			dst := make([]float64, n)
			expected := make([]float64, n)

			// Fill with test data
			for i := 0; i < n; i++ {
				a[i] = float64(i) + 0.5
				b[i] = float64(i) * 2.0
				expected[i] = a[i] + b[i]
			}

			// Call SSE2 implementation directly
			AddBlock(dst, a, b)

			// Verify results
			for i := 0; i < n; i++ {
				if dst[i] != expected[i] {
					t.Errorf("AddBlock[%d] = %v, want %v", i, dst[i], expected[i])
				}
			}
		})
	}
}

// TestAddBlockInPlace_SSE2 tests the SSE2 in-place implementation
func TestAddBlockInPlace_SSE2(t *testing.T) {
	sizes := []int{0, 1, 2, 3, 4, 8, 16, 32, 64, 100}

	for _, n := range sizes {
		t.Run(sizeStr(n), func(t *testing.T) {
			dst := make([]float64, n)
			src := make([]float64, n)
			expected := make([]float64, n)

			for i := 0; i < n; i++ {
				dst[i] = float64(i) + 1.0
				src[i] = float64(i) * 2.0
				expected[i] = dst[i] + src[i]
			}

			AddBlockInPlace(dst, src)

			for i := 0; i < n; i++ {
				if dst[i] != expected[i] {
					t.Errorf("AddBlockInPlace[%d] = %v, want %v", i, dst[i], expected[i])
				}
			}
		})
	}
}

// BenchmarkAddBlock_SSE2_Direct benchmarks the SSE2 implementation directly
func BenchmarkAddBlock_SSE2_Direct(b *testing.B) {
	sizes := []int{16, 64, 256, 1024, 4096}

	for _, n := range sizes {
		b.Run(sizeStr(n), func(b *testing.B) {
			dst := make([]float64, n)
			a := make([]float64, n)
			src := make([]float64, n)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				AddBlock(dst, a, src)
			}

			bytes := int64(n) * 8 * 3 // 3 slices, 8 bytes per float64
			b.SetBytes(bytes)
		})
	}
}

func sizeStr(n int) string {
	if n >= 1024 {
		return fmt.Sprintf("%dK", n/1024)
	}
	return fmt.Sprintf("%d", n)
}
