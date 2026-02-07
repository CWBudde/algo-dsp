package vecmath

import "testing"

// Reference implementations for multiplication testing
func mulBlockRef(dst, a, b []float64) {
	for i := range dst {
		dst[i] = a[i] * b[i]
	}
}

func mulBlockInPlaceRef(dst, src []float64) {
	for i := range dst {
		dst[i] *= src[i]
	}
}

func TestMulBlock(t *testing.T) {
	sizes := []int{0, 1, 2, 3, 4, 5, 7, 8, 15, 16, 17, 31, 32, 33, 63, 64, 100, 1000, 1023, 1024, 1025}

	for _, n := range sizes {
		t.Run(sizeStr(n), func(t *testing.T) {
			a := make([]float64, n)
			b := make([]float64, n)
			dst := make([]float64, n)
			expected := make([]float64, n)

			// Fill with test data
			for i := 0; i < n; i++ {
				a[i] = float64(i) + 0.5
				b[i] = float64(n-i) * 0.1
			}

			// Compute reference
			mulBlockRef(expected, a, b)

			// Compute with SIMD
			MulBlock(dst, a, b)

			// Compare
			for i := 0; i < n; i++ {
				if !closeEnough(dst[i], expected[i]) {
					t.Errorf("MulBlock[%d]: got %v, want %v", i, dst[i], expected[i])
				}
			}
		})
	}
}

func TestMulBlockInPlace(t *testing.T) {
	sizes := []int{0, 1, 2, 3, 4, 5, 7, 8, 15, 16, 17, 31, 32, 33, 63, 64, 100, 1000}

	for _, n := range sizes {
		t.Run(sizeStr(n), func(t *testing.T) {
			src := make([]float64, n)
			dst := make([]float64, n)
			expected := make([]float64, n)

			for i := 0; i < n; i++ {
				src[i] = float64(i) + 0.5
				dst[i] = float64(n-i) * 0.1
				expected[i] = dst[i]
			}

			mulBlockInPlaceRef(expected, src)
			MulBlockInPlace(dst, src)

			for i := 0; i < n; i++ {
				if !closeEnough(dst[i], expected[i]) {
					t.Errorf("MulBlockInPlace[%d]: got %v, want %v", i, dst[i], expected[i])
				}
			}
		})
	}
}

func TestMulBlockPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MulBlock should panic on mismatched lengths")
		}
	}()
	MulBlock(make([]float64, 5), make([]float64, 5), make([]float64, 6))
}

func TestMulBlockInPlacePanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MulBlockInPlace should panic on mismatched lengths")
		}
	}()
	MulBlockInPlace(make([]float64, 5), make([]float64, 6))
}
