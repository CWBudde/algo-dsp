package simd

import (
	"math"
	"testing"
)

// Reference implementations for testing
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

func addMulBlockRef(dst, a, b []float64, scale float64) {
	for i := range dst {
		dst[i] = (a[i] + b[i]) * scale
	}
}

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

func mulAddBlockRef(dst, a, b, c []float64) {
	for i := range dst {
		dst[i] = a[i]*b[i] + c[i]
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

func TestScaleBlockPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("ScaleBlock should panic on mismatched lengths")
		}
	}()
	ScaleBlock(make([]float64, 5), make([]float64, 6), 1.0)
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

func TestMulAddBlockPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MulAddBlock should panic on mismatched lengths")
		}
	}()
	MulAddBlock(make([]float64, 5), make([]float64, 5), make([]float64, 5), make([]float64, 6))
}

// Helpers

func closeEnough(a, b float64) bool {
	const epsilon = 1e-14
	if a == b {
		return true
	}
	diff := math.Abs(a - b)
	if a == 0 || b == 0 {
		return diff < epsilon
	}
	return diff/math.Max(math.Abs(a), math.Abs(b)) < epsilon
}

func sizeStr(n int) string {
	return "n=" + itoa(n)
}

func floatStr(f float64) string {
	if f == 0.0 {
		return "0"
	}
	if f == 1.0 {
		return "1"
	}
	if f == -1.0 {
		return "-1"
	}
	if f == 0.5 {
		return "0.5"
	}
	if f == 2.0 {
		return "2"
	}
	return "pi"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
