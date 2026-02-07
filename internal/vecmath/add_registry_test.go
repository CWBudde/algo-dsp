package vecmath

import (
	"testing"

	"github.com/cwbudde/algo-dsp/internal/vecmath/registry"
)

// TestAddOperationsRegistry verifies the registry-based add operations work correctly.
func TestAddOperationsRegistry(t *testing.T) {
	// This test verifies that add operations work with registered implementations

	// Test AddBlock
	dst := make([]float64, 5)
	a := []float64{1, 2, 3, 4, 5}
	b := []float64{10, 20, 30, 40, 50}
	expected := []float64{11, 22, 33, 44, 55}

	AddBlock(dst, a, b)

	for i := range dst {
		if dst[i] != expected[i] {
			t.Errorf("AddBlock[%d] = %v, want %v", i, dst[i], expected[i])
		}
	}

	// Test AddBlockInPlace
	dst = []float64{1, 2, 3, 4, 5}
	src := []float64{10, 20, 30, 40, 50}
	expected = []float64{11, 22, 33, 44, 55}

	AddBlockInPlace(dst, src)

	for i := range dst {
		if dst[i] != expected[i] {
			t.Errorf("AddBlockInPlace[%d] = %v, want %v", i, dst[i], expected[i])
		}
	}
}

// TestAddOperationsWithRealImplementations tests with actual registered implementations.
func TestAddOperationsWithRealImplementations(t *testing.T) {
	// This test relies on the init() functions from arch packages
	// running before this test (which they do via init.go imports)

	entries := registry.Global.ListEntries()
	if len(entries) == 0 {
		t.Skip("no implementations registered - skipping real implementation test")
	}

	t.Logf("Testing with %d registered implementations", len(entries))

	dst := make([]float64, 100)
	a := make([]float64, 100)
	b := make([]float64, 100)

	// Fill with test data
	for i := range a {
		a[i] = float64(i)
		b[i] = float64(i * 2)
	}

	AddBlock(dst, a, b)

	// Verify results
	for i := range dst {
		expected := a[i] + b[i]
		if dst[i] != expected {
			t.Errorf("AddBlock[%d] = %v, want %v", i, dst[i], expected)
			break
		}
	}
}

// BenchmarkAddBlock_RegistryDispatch benchmarks the registry-based dispatch.
func BenchmarkAddBlock_RegistryDispatch(b *testing.B) {
	dst := make([]float64, 1024)
	a := make([]float64, 1024)
	src := make([]float64, 1024)

	// Fill with test data
	for i := range a {
		a[i] = float64(i)
		src[i] = float64(i * 2)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		AddBlock(dst, a, src)
	}

	// Calculate throughput
	bytes := int64(len(dst)) * 8 * 3 // 3 slices, 8 bytes per float64
	b.SetBytes(bytes)
}

// BenchmarkAddBlock_CachedCall measures the steady-state (cached) call overhead.
// After the first call, this should be identical to direct function call performance.
func BenchmarkAddBlock_CachedCall(b *testing.B) {
	dst := make([]float64, 1024)
	a := make([]float64, 1024)
	src := make([]float64, 1024)

	// Warm up - ensure init has happened
	AddBlock(dst, a, src)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		AddBlock(dst, a, src)
	}
}
