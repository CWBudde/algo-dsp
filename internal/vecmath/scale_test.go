package vecmath

import (
	"math"
	"testing"
)

// Reference implementations for scaling testing
func scaleBlockRef(dst, src []float64, scale float64) {
	for i := range dst {
		dst[i] = src[i] * scale
	}
}

func scaleBlockInPlaceRef(dst []float64, scale float64) {
	for i := range dst {
		dst[i] *= scale
	}
}

func TestScaleBlock(t *testing.T) {
	sizes := []int{0, 1, 2, 3, 4, 5, 7, 8, 15, 16, 17, 31, 32, 33, 63, 64, 100, 1000}
	scales := []float64{0.0, 1.0, -1.0, 0.5, 2.0, math.Pi}

	for _, n := range sizes {
		for _, scale := range scales {
			t.Run(sizeStr(n)+"_scale_"+floatStr(scale), func(t *testing.T) {
				src := make([]float64, n)
				dst := make([]float64, n)
				expected := make([]float64, n)

				for i := 0; i < n; i++ {
					src[i] = float64(i) + 0.5
				}

				scaleBlockRef(expected, src, scale)
				ScaleBlock(dst, src, scale)

				for i := 0; i < n; i++ {
					if !closeEnough(dst[i], expected[i]) {
						t.Errorf("ScaleBlock[%d]: got %v, want %v", i, dst[i], expected[i])
					}
				}
			})
		}
	}
}

func TestScaleBlockInPlace(t *testing.T) {
	sizes := []int{0, 1, 2, 3, 4, 5, 7, 8, 15, 16, 17, 31, 32, 33, 63, 64, 100, 1000}
	scales := []float64{0.0, 1.0, -1.0, 0.5, 2.0, math.Pi}

	for _, n := range sizes {
		for _, scale := range scales {
			t.Run(sizeStr(n)+"_scale_"+floatStr(scale), func(t *testing.T) {
				dst := make([]float64, n)
				expected := make([]float64, n)

				for i := 0; i < n; i++ {
					dst[i] = float64(i) + 0.5
					expected[i] = dst[i]
				}

				scaleBlockInPlaceRef(expected, scale)
				ScaleBlockInPlace(dst, scale)

				for i := 0; i < n; i++ {
					if !closeEnough(dst[i], expected[i]) {
						t.Errorf("ScaleBlockInPlace[%d]: got %v, want %v", i, dst[i], expected[i])
					}
				}
			})
		}
	}
}

func TestScaleBlockPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("ScaleBlock should panic on mismatched lengths")
		}
	}()
	ScaleBlock(make([]float64, 5), make([]float64, 6), 1.0)
}
