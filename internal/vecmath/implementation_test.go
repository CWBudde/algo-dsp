package vecmath

import (
	"testing"

	"github.com/cwbudde/algo-dsp/internal/cpu"
	"github.com/cwbudde/algo-dsp/internal/vecmath/registry"
)

func hasImplementation(name string) bool {
	for _, entry := range registry.Global.ListEntries() {
		if entry.Name == name {
			return true
		}
	}
	return false
}

// TestForceGeneric tests that we can force the generic implementation via CPU features
func TestForceGeneric(t *testing.T) {
	// Force generic implementation
	cpu.SetForcedFeatures(cpu.Features{
		ForceGeneric: true,
	})
	defer cpu.ResetDetection()

	// Now test - should use generic implementation
	dst := make([]float64, 10)
	a := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}

	AddBlock(dst, a, b)

	for i := range dst {
		expected := a[i] + b[i]
		if dst[i] != expected {
			t.Errorf("AddBlock[%d] = %v, want %v", i, dst[i], expected)
		}
	}

	// Verify generic was selected
	entry := registry.Global.Lookup(cpu.DetectFeatures())
	if entry == nil {
		t.Fatal("No implementation found in registry")
	}
	if entry.Name != "generic" {
		t.Errorf("Expected generic implementation, got %s", entry.Name)
	}
}

// TestForceAVX2 tests that we can force AVX2 implementation via CPU features
func TestForceAVX2(t *testing.T) {
	if !hasImplementation("avx2") {
		t.Skip("avx2 implementation not registered in this build")
	}

	// Force AVX2 features
	cpu.SetForcedFeatures(cpu.Features{
		HasSSE2:      true,
		HasAVX2:      true,
		Architecture: "amd64",
	})
	defer cpu.ResetDetection()

	// Test - should use AVX2 implementation
	dst := make([]float64, 10)
	a := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}

	AddBlock(dst, a, b)

	for i := range dst {
		expected := a[i] + b[i]
		if dst[i] != expected {
			t.Errorf("AddBlock[%d] = %v, want %v", i, dst[i], expected)
		}
	}

	// Verify AVX2 was selected
	entry := registry.Global.Lookup(cpu.DetectFeatures())
	if entry == nil {
		t.Fatal("No implementation found in registry")
	}
	if entry.Name != "avx2" {
		t.Errorf("Expected avx2 implementation, got %s", entry.Name)
	}
}

// TestForceSSE2 tests that we can force SSE2 implementation
func TestForceSSE2(t *testing.T) {
	if !hasImplementation("sse2") {
		t.Skip("sse2 implementation not registered in this build")
	}

	// Force SSE2 only (no AVX2)
	cpu.SetForcedFeatures(cpu.Features{
		HasSSE2:      true,
		HasAVX2:      false,
		Architecture: "amd64",
	})
	defer cpu.ResetDetection()

	// Test AddBlock - should use SSE2 implementation
	dst := make([]float64, 10)
	a := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}

	AddBlock(dst, a, b)

	for i := range dst {
		expected := a[i] + b[i]
		if dst[i] != expected {
			t.Errorf("AddBlock[%d] = %v, want %v", i, dst[i], expected)
		}
	}

	// Test MaxAbs - should use SSE2 implementation
	x := []float64{-1.5, 2.0, -3.5, 4.0, -5.5}
	result := MaxAbs(x)

	expected := 5.5
	if result != expected {
		t.Errorf("MaxAbs() = %v, want %v", result, expected)
	}

	// Verify SSE2 was selected
	entry := registry.Global.Lookup(cpu.DetectFeatures())
	if entry == nil {
		t.Fatal("No implementation found in registry")
	}
	if entry.Name != "sse2" {
		t.Errorf("Expected sse2 implementation, got %s", entry.Name)
	}
}

// BenchmarkCompareImplementations benchmarks all available implementations
func BenchmarkCompareImplementations(b *testing.B) {
	sizes := []int{64, 256, 1024}

	// Test each size
	for _, n := range sizes {
		b.Run(sizeStr(n), func(b *testing.B) {
			dst := make([]float64, n)
			a := make([]float64, n)
			src := make([]float64, n)

			// Fill with data
			for i := 0; i < n; i++ {
				a[i] = float64(i)
				src[i] = float64(i * 2)
			}

			// Benchmark Generic
			b.Run("Generic", func(b *testing.B) {
				cpu.SetForcedFeatures(cpu.Features{ForceGeneric: true})
				defer cpu.ResetDetection()

				b.ResetTimer()
				b.ReportAllocs()

				for i := 0; i < b.N; i++ {
					AddBlock(dst, a, src)
				}

				bytes := int64(n) * 8 * 3
				b.SetBytes(bytes)
			})

			// Benchmark AVX2 (if on amd64)
			b.Run("AVX2", func(b *testing.B) {
				cpu.SetForcedFeatures(cpu.Features{
					HasSSE2:      true,
					HasAVX2:      true,
					Architecture: "amd64",
				})
				defer cpu.ResetDetection()

				b.ResetTimer()
				b.ReportAllocs()

				for i := 0; i < b.N; i++ {
					AddBlock(dst, a, src)
				}

				bytes := int64(n) * 8 * 3
				b.SetBytes(bytes)
			})
		})
	}
}
