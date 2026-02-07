package vecmath

import (
	"math"
	"testing"
)

// Reference implementations for fused operation testing
func addMulBlockRef(dst, a, b []float64, scale float64) {
	for i := range dst {
		dst[i] = (a[i] + b[i]) * scale
	}
}

func mulAddBlockRef(dst, a, b, c []float64) {
	for i := range dst {
		dst[i] = a[i]*b[i] + c[i]
	}
}

func TestAddMulBlock(t *testing.T) {
	sizes := []int{0, 1, 2, 3, 4, 5, 7, 8, 15, 16, 17, 31, 32, 33, 63, 64, 100, 1000}
	scales := []float64{0.0, 1.0, -1.0, 0.5, 2.0, math.Pi}

	for _, n := range sizes {
		for _, scale := range scales {
			t.Run(sizeStr(n)+"_scale_"+floatStr(scale), func(t *testing.T) {
				a := make([]float64, n)
				b := make([]float64, n)
				dst := make([]float64, n)
				expected := make([]float64, n)

				for i := range a {
					a[i] = float64(i) + 0.5
					b[i] = float64(n-i) * 0.1
				}

				addMulBlockRef(expected, a, b, scale)
				AddMulBlock(dst, a, b, scale)

				for i := range dst {
					if !closeEnough(dst[i], expected[i]) {
						t.Errorf("AddMulBlock[%d]: got %v, want %v", i, dst[i], expected[i])
					}
				}
			})
		}
	}
}

func TestMulAddBlock(t *testing.T) {
	sizes := []int{0, 1, 2, 3, 4, 5, 7, 8, 15, 16, 17, 31, 32, 33, 63, 64, 100, 1000}

	for _, n := range sizes {
		t.Run(sizeStr(n), func(t *testing.T) {
			a := make([]float64, n)
			b := make([]float64, n)
			c := make([]float64, n)
			dst := make([]float64, n)
			expected := make([]float64, n)

			for i := range a {
				a[i] = float64(i) + 0.5
				b[i] = float64(n-i) * 0.1
				c[i] = float64(i*2) - 1.0
			}

			mulAddBlockRef(expected, a, b, c)
			MulAddBlock(dst, a, b, c)

			for i := range dst {
				if !closeEnough(dst[i], expected[i]) {
					t.Errorf("MulAddBlock[%d]: got %v, want %v", i, dst[i], expected[i])
				}
			}
		})
	}
}

func TestAddMulBlockPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("AddMulBlock should panic on mismatched lengths")
		}
	}()
	AddMulBlock(make([]float64, 5), make([]float64, 5), make([]float64, 6), 1.0)
}

func TestMulAddBlockPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MulAddBlock should panic on mismatched lengths")
		}
	}()
	MulAddBlock(make([]float64, 5), make([]float64, 5), make([]float64, 5), make([]float64, 6))
}
