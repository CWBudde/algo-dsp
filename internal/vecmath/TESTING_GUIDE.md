# Vecmath Testing Guide

## Testing Individual Implementations

With the registry pattern, you have **three strategies** for testing individual SIMD implementations:

---

## Strategy 1: Direct Tests in arch/ Packages ⭐ Recommended

Test implementations directly in their own packages without going through the registry.

### Location
```
internal/vecmath/arch/
├── generic/
│   ├── add.go
│   └── add_test.go          ← Test generic impl directly
├── amd64/
│   └── avx2/
│       ├── add.go
│       └── add_test.go      ← Test AVX2 impl directly
└── arm64/
    └── neon/
        ├── maxabs.go
        └── maxabs_test.go   ← Test NEON impl directly
```

### Example: Testing AVX2 Implementation

**File:** `internal/vecmath/arch/amd64/avx2/add_test.go`

```go
//go:build amd64 && !purego

package avx2

import (
	"fmt"
	"testing"
)

func TestAddBlock_AVX2(t *testing.T) {
	sizes := []int{0, 1, 4, 8, 16, 32, 64, 100, 1000}

	for _, n := range sizes {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			a := make([]float64, n)
			b := make([]float64, n)
			dst := make([]float64, n)
			expected := make([]float64, n)

			// Fill test data
			for i := 0; i < n; i++ {
				a[i] = float64(i) + 0.5
				b[i] = float64(i) * 2.0
				expected[i] = a[i] + b[i]
			}

			// Call AVX2 implementation directly
			AddBlock(dst, a, b)

			// Verify
			for i := 0; i < n; i++ {
				if dst[i] != expected[i] {
					t.Errorf("AddBlock[%d] = %v, want %v", i, dst[i], expected[i])
				}
			}
		})
	}
}

func BenchmarkAddBlock_AVX2_Direct(b *testing.B) {
	sizes := []int{16, 64, 256, 1024, 4096}

	for _, n := range sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			dst := make([]float64, n)
			a := make([]float64, n)
			src := make([]float64, n)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				AddBlock(dst, a, src)
			}

			bytes := int64(n) * 8 * 3
			b.SetBytes(bytes)
		})
	}
}
```

### Running Direct Tests

```bash
# Test AVX2 implementation directly
go test -v github.com/cwbudde/algo-dsp/internal/vecmath/arch/amd64/avx2

# Test generic implementation directly
go test -v github.com/cwbudde/algo-dsp/internal/vecmath/arch/generic

# Test NEON implementation directly (on ARM64)
go test -v github.com/cwbudde/algo-dsp/internal/vecmath/arch/arm64/neon

# Benchmark AVX2 directly
go test -bench=. github.com/cwbudde/algo-dsp/internal/vecmath/arch/amd64/avx2
```

### Pros & Cons

**Pros:**
- ✅ No registry overhead
- ✅ Tests run only on appropriate platforms (via build tags)
- ✅ Direct access to implementation
- ✅ Easy to debug
- ✅ Clear separation of concerns

**Cons:**
- ⚠️ Requires test duplication across packages
- ⚠️ Can't test AVX2 on non-AVX2 machines

---

## Strategy 2: Forced CPU Features

Use `cpu.SetForcedFeatures()` to control which implementation the registry selects.

### Example: Force Generic Implementation

```go
package vecmath

import (
	"testing"
	"github.com/cwbudde/algo-dsp/internal/cpu"
)

func TestAddBlock_ForcedGeneric(t *testing.T) {
	// Force generic implementation
	cpu.SetForcedFeatures(cpu.Features{
		ForceGeneric: true,
	})
	defer cpu.ResetDetection()

	// Now AddBlock() will use generic implementation
	dst := make([]float64, 10)
	a := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}

	AddBlock(dst, a, b)

	// Verify results
	for i := range dst {
		expected := a[i] + b[i]
		if dst[i] != expected {
			t.Errorf("AddBlock[%d] = %v, want %v", i, dst[i], expected)
		}
	}
}
```

### Example: Force AVX2 Implementation

```go
func TestAddBlock_ForcedAVX2(t *testing.T) {
	// Force AVX2 features
	cpu.SetForcedFeatures(cpu.Features{
		HasSSE2:      true,
		HasAVX2:      true,
		Architecture: "amd64",
	})
	defer cpu.ResetDetection()

	// Now AddBlock() will use AVX2 implementation
	dst := make([]float64, 10)
	a := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}

	AddBlock(dst, a, b)

	// Verify results
	for i := range dst {
		expected := a[i] + b[i]
		if dst[i] != expected {
			t.Errorf("AddBlock[%d] = %v, want %v", i, dst[i], expected)
		}
	}
}
```

### Example: Force SSE2 for MaxAbs

```go
func TestMaxAbs_ForcedSSE2(t *testing.T) {
	// Force SSE2 only (no AVX2)
	cpu.SetForcedFeatures(cpu.Features{
		HasSSE2:      true,
		HasAVX2:      false, // Explicitly disable AVX2
		Architecture: "amd64",
	})
	defer cpu.ResetDetection()

	// Test MaxAbs - should use SSE2 implementation
	x := []float64{-1.5, 2.0, -3.5, 4.0, -5.5}
	result := MaxAbs(x)

	expected := 5.5
	if result != expected {
		t.Errorf("MaxAbs() = %v, want %v", result, expected)
	}
}
```

### Comparative Benchmarks

```go
func BenchmarkCompareImplementations(b *testing.B) {
	n := 1024
	dst := make([]float64, n)
	a := make([]float64, n)
	src := make([]float64, n)

	// Benchmark Generic
	b.Run("Generic", func(b *testing.B) {
		cpu.SetForcedFeatures(cpu.Features{ForceGeneric: true})
		defer cpu.ResetDetection()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			AddBlock(dst, a, src)
		}
	})

	// Benchmark AVX2
	b.Run("AVX2", func(b *testing.B) {
		cpu.SetForcedFeatures(cpu.Features{
			HasSSE2:      true,
			HasAVX2:      true,
			Architecture: "amd64",
		})
		defer cpu.ResetDetection()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			AddBlock(dst, a, src)
		}
	})

	// Benchmark SSE2
	b.Run("SSE2", func(b *testing.B) {
		cpu.SetForcedFeatures(cpu.Features{
			HasSSE2:      true,
			HasAVX2:      false,
			Architecture: "amd64",
		})
		defer cpu.ResetDetection()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			MaxAbs(dst) // Only MaxAbs has SSE2 impl currently
		}
	})
}
```

### Running Tests with Forced Features

```bash
# Run all forced-feature tests
go test -v -run TestAddBlock_Forced github.com/cwbudde/algo-dsp/internal/vecmath

# Run comparative benchmarks
go test -bench=BenchmarkCompareImplementations github.com/cwbudde/algo-dsp/internal/vecmath
```

### Pros & Cons

**Pros:**
- ✅ Can test any implementation on any platform
- ✅ Easy to write comparative benchmarks
- ✅ Tests the full dispatch path (registry + implementation)
- ✅ Good for integration testing

**Cons:**
- ⚠️ Registry overhead included in measurements
- ⚠️ Must remember to call `defer cpu.ResetDetection()`
- ⚠️ Concurrent tests may interfere (global state)

---

## Strategy 3: Direct Import in Tests

Import implementation packages directly and call their functions.

### Example

```go
package vecmath_test

import (
	"testing"

	"github.com/cwbudde/algo-dsp/internal/vecmath/arch/amd64/avx2"
	"github.com/cwbudde/algo-dsp/internal/vecmath/arch/generic"
)

func TestCompareImplementations(t *testing.T) {
	n := 100
	a := make([]float64, n)
	b := make([]float64, n)

	// Fill test data
	for i := 0; i < n; i++ {
		a[i] = float64(i)
		b[i] = float64(i * 2)
	}

	// Test generic
	genericResult := make([]float64, n)
	generic.AddBlock(genericResult, a, b)

	// Test AVX2
	avx2Result := make([]float64, n)
	avx2.AddBlock(avx2Result, a, b)

	// Compare results
	for i := 0; i < n; i++ {
		if genericResult[i] != avx2Result[i] {
			t.Errorf("Implementations differ at [%d]: generic=%v, avx2=%v",
				i, genericResult[i], avx2Result[i])
		}
	}
}
```

### Pros & Cons

**Pros:**
- ✅ Direct comparison between implementations
- ✅ No global state modifications
- ✅ Clear and explicit

**Cons:**
- ⚠️ Build tags required for platform-specific code
- ⚠️ Manual import management
- ⚠️ Less discoverable than dedicated test files

---

## Recommended Testing Strategy

For **comprehensive coverage**, use a combination:

### 1. Direct Tests in arch/ (Primary)
- Test each implementation in its own package
- Verify correctness with various input sizes
- Benchmark performance directly
- **Location:** `internal/vecmath/arch/{platform}/{simd}/*_test.go`

### 2. Forced Features (Validation)
- Write integration tests that force specific implementations
- Create comparative benchmarks across implementations
- Validate registry selection logic
- **Location:** `internal/vecmath/implementation_test.go`

### 3. Public API Tests (End-to-End)
- Test via public API without forcing features
- Verify automatic selection works correctly
- Ensure correctness regardless of platform
- **Location:** `internal/vecmath/*_test.go`

## Test Organization

```
internal/vecmath/
├── add_test.go                    # Public API tests (end-to-end)
├── implementation_test.go         # Forced feature tests (validation)
├── arch/
│   ├── generic/
│   │   ├── add.go
│   │   ├── add_test.go            # Direct generic tests
│   │   └── register.go
│   ├── amd64/
│   │   └── avx2/
│   │       ├── add.go
│   │       ├── add_test.go        # Direct AVX2 tests
│   │       └── register.go
│   └── arm64/
│       └── neon/
│           ├── maxabs.go
│           ├── maxabs_test.go     # Direct NEON tests
│           └── register.go
└── registry/
    ├── registry.go
    ├── registry_test.go           # Registry unit tests
    └── integration_*_test.go      # Registry integration tests
```

## Running All Tests

```bash
# Run all vecmath tests (public API + direct impl tests)
go test ./internal/vecmath/...

# Run only public API tests
go test github.com/cwbudde/algo-dsp/internal/vecmath

# Run only AVX2 implementation tests
go test github.com/cwbudde/algo-dsp/internal/vecmath/arch/amd64/avx2

# Run only generic implementation tests
go test github.com/cwbudde/algo-dsp/internal/vecmath/arch/generic

# Run comparative benchmarks
go test -bench=BenchmarkCompare github.com/cwbudde/algo-dsp/internal/vecmath

# Run all benchmarks including direct implementation benchmarks
go test -bench=. ./internal/vecmath/...
```

## Best Practices

### 1. Always Clean Up Forced Features
```go
cpu.SetForcedFeatures(features)
defer cpu.ResetDetection()  // ← Always use defer!
```

### 2. Use Build Tags Correctly
```go
//go:build amd64 && !purego

package avx2
```

### 3. Test Edge Cases
- Empty slices
- Single element
- Unaligned sizes (not multiples of SIMD width)
- Large slices

### 4. Benchmark All Sizes
```go
sizes := []int{16, 64, 256, 1024, 4096, 16384}
```

### 5. Verify Numerical Accuracy
```go
const tolerance = 1e-15
if math.Abs(result - expected) > tolerance {
    t.Errorf("numerical error too large")
}
```

---

## Summary

✅ **Yes, you CAN test individual implementations!**

**Best approach:** Combine all three strategies:
1. **Direct tests in arch/** - Primary correctness and performance validation
2. **Forced features** - Comparative benchmarks and integration tests
3. **Public API tests** - End-to-end correctness verification

This gives you full test coverage at every level:
- ✅ Individual implementations (unit tests)
- ✅ Registry selection logic (integration tests)
- ✅ Public API behavior (end-to-end tests)
