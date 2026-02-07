package vecmath

import "testing"

// Reference implementations for addition testing
func addBlockRef(dst, a, b []float64) {
	for i := range dst {
		dst[i] = a[i] + b[i]
	}
}

func addBlockInPlaceRef(dst, src []float64) {
	for i := range dst {
		dst[i] += src[i]
	}
}

func TestAddBlock(t *testing.T) {
	sizes := []int{0, 1, 2, 3, 4, 5, 7, 8, 15, 16, 17, 31, 32, 33, 63, 64, 100, 1000}

	for _, n := range sizes {
		t.Run(sizeStr(n), func(t *testing.T) {
			a := make([]float64, n)
			b := make([]float64, n)
			dst := make([]float64, n)
			expected := make([]float64, n)

			for i := range a {
				a[i] = float64(i) + 0.5
				b[i] = float64(n-i) * 0.1
			}

			addBlockRef(expected, a, b)
			AddBlock(dst, a, b)

			for i := range dst {
				if !closeEnough(dst[i], expected[i]) {
					t.Errorf("AddBlock[%d]: got %v, want %v", i, dst[i], expected[i])
				}
			}
		})
	}
}

func TestAddBlockInPlace(t *testing.T) {
	sizes := []int{0, 1, 2, 3, 4, 5, 7, 8, 15, 16, 17, 31, 32, 33, 63, 64, 100, 1000}

	for _, n := range sizes {
		t.Run(sizeStr(n), func(t *testing.T) {
			src := make([]float64, n)
			dst := make([]float64, n)
			expected := make([]float64, n)

			for i := range src {
				src[i] = float64(i) + 0.5
				dst[i] = float64(n-i) * 0.1
				expected[i] = dst[i]
			}

			addBlockInPlaceRef(expected, src)
			AddBlockInPlace(dst, src)

			for i := range dst {
				if !closeEnough(dst[i], expected[i]) {
					t.Errorf("AddBlockInPlace[%d]: got %v, want %v", i, dst[i], expected[i])
				}
			}
		})
	}
}

func TestAddBlockPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("AddBlock should panic on mismatched lengths")
		}
	}()
	AddBlock(make([]float64, 5), make([]float64, 5), make([]float64, 6))
}

func TestAddBlockInPlacePanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("AddBlockInPlace should panic on mismatched lengths")
		}
	}()
	AddBlockInPlace(make([]float64, 5), make([]float64, 6))
}
