package simd

import "testing"

var benchSizes = []struct {
	name string
	size int
}{
	{"16", 16},
	{"64", 64},
	{"256", 256},
	{"1K", 1024},
	{"4K", 4096},
	{"16K", 16384},
	{"64K", 65536},
}

func BenchmarkMulBlock(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			a := make([]float64, tc.size)
			c := make([]float64, tc.size)
			dst := make([]float64, tc.size)

			for i := range a {
				a[i] = float64(i) + 0.5
				c[i] = float64(tc.size-i) * 0.1
			}

			b.SetBytes(int64(tc.size * 8 * 3)) // 3 arrays accessed
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				MulBlock(dst, a, c)
			}
		})
	}
}

func BenchmarkMulBlockRef(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			a := make([]float64, tc.size)
			c := make([]float64, tc.size)
			dst := make([]float64, tc.size)

			for i := range a {
				a[i] = float64(i) + 0.5
				c[i] = float64(tc.size-i) * 0.1
			}

			b.SetBytes(int64(tc.size * 8 * 3))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				mulBlockRef(dst, a, c)
			}
		})
	}
}

func BenchmarkMulBlockInPlace(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			src := make([]float64, tc.size)
			dst := make([]float64, tc.size)

			for i := range src {
				src[i] = float64(i) + 0.5
			}

			b.SetBytes(int64(tc.size * 8 * 2))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Reset dst for fair comparison
				for j := range dst {
					dst[j] = float64(j) * 0.1
				}
				MulBlockInPlace(dst, src)
			}
		})
	}
}

func BenchmarkScaleBlock(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			src := make([]float64, tc.size)
			dst := make([]float64, tc.size)
			scale := 1.5

			for i := range src {
				src[i] = float64(i) + 0.5
			}

			b.SetBytes(int64(tc.size * 8 * 2))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				ScaleBlock(dst, src, scale)
			}
		})
	}
}

func BenchmarkScaleBlockRef(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			src := make([]float64, tc.size)
			dst := make([]float64, tc.size)
			scale := 1.5

			for i := range src {
				src[i] = float64(i) + 0.5
			}

			b.SetBytes(int64(tc.size * 8 * 2))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				scaleBlockRef(dst, src, scale)
			}
		})
	}
}

func BenchmarkScaleBlockInPlace(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			dst := make([]float64, tc.size)
			scale := 1.5

			b.SetBytes(int64(tc.size * 8))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Reset for fair comparison
				for j := range dst {
					dst[j] = float64(j) + 0.5
				}
				ScaleBlockInPlace(dst, scale)
			}
		})
	}
}

func BenchmarkAddMulBlock(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			a := make([]float64, tc.size)
			c := make([]float64, tc.size)
			dst := make([]float64, tc.size)
			scale := 0.5

			for i := range a {
				a[i] = float64(i) + 0.5
				c[i] = float64(tc.size-i) * 0.1
			}

			b.SetBytes(int64(tc.size * 8 * 3))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				AddMulBlock(dst, a, c, scale)
			}
		})
	}
}

func BenchmarkAddMulBlockRef(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			a := make([]float64, tc.size)
			c := make([]float64, tc.size)
			dst := make([]float64, tc.size)
			scale := 0.5

			for i := range a {
				a[i] = float64(i) + 0.5
				c[i] = float64(tc.size-i) * 0.1
			}

			b.SetBytes(int64(tc.size * 8 * 3))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				addMulBlockRef(dst, a, c, scale)
			}
		})
	}
}

func BenchmarkAddBlock(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			a := make([]float64, tc.size)
			c := make([]float64, tc.size)
			dst := make([]float64, tc.size)

			for i := range a {
				a[i] = float64(i) + 0.5
				c[i] = float64(tc.size-i) * 0.1
			}

			b.SetBytes(int64(tc.size * 8 * 3))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				AddBlock(dst, a, c)
			}
		})
	}
}

func BenchmarkAddBlockRef(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			a := make([]float64, tc.size)
			c := make([]float64, tc.size)
			dst := make([]float64, tc.size)

			for i := range a {
				a[i] = float64(i) + 0.5
				c[i] = float64(tc.size-i) * 0.1
			}

			b.SetBytes(int64(tc.size * 8 * 3))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				addBlockRef(dst, a, c)
			}
		})
	}
}

func BenchmarkMulAddBlock(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			a := make([]float64, tc.size)
			bslice := make([]float64, tc.size)
			c := make([]float64, tc.size)
			dst := make([]float64, tc.size)

			for i := range a {
				a[i] = float64(i) + 0.5
				bslice[i] = float64(tc.size-i) * 0.1
				c[i] = float64(i*2) - 1.0
			}

			b.SetBytes(int64(tc.size * 8 * 4))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				MulAddBlock(dst, a, bslice, c)
			}
		})
	}
}

func BenchmarkMulAddBlockRef(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			a := make([]float64, tc.size)
			bslice := make([]float64, tc.size)
			c := make([]float64, tc.size)
			dst := make([]float64, tc.size)

			for i := range a {
				a[i] = float64(i) + 0.5
				bslice[i] = float64(tc.size-i) * 0.1
				c[i] = float64(i*2) - 1.0
			}

			b.SetBytes(int64(tc.size * 8 * 4))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				mulAddBlockRef(dst, a, bslice, c)
			}
		})
	}
}
