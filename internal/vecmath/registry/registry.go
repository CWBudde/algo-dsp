// Package registry provides the implementation registry for vecmath operations.
//
// The registry-based dispatch system allows multiple implementation variants
// (generic, SSE2, AVX2, NEON, etc.) to coexist. The best implementation for
// the current CPU is selected automatically at runtime.
//
// Architecture-specific implementations register themselves via init() functions,
// and the vecmath package uses the registry to select the best implementation
// at runtime based on detected CPU features.
package registry

import (
	"sync"

	"github.com/cwbudde/algo-dsp/internal/cpu"
)

// OpEntry represents a registered implementation variant for vecmath operations.
//
// Each entry contains typed function pointers for all supported operations at a
// specific SIMD level. Not all fields need to be populated - only implement the
// operations available at that SIMD level.
type OpEntry struct {
	// Name is a human-readable identifier for this implementation (e.g., "avx2", "neon").
	Name string

	// SIMDLevel indicates the SIMD instruction set required for this implementation.
	SIMDLevel cpu.SIMDLevel

	// Priority determines selection order when multiple compatible implementations exist.
	// Higher priority implementations are preferred. Suggested priorities:
	//   - Generic (SIMDNone): 0
	//   - SSE2: 10
	//   - AVX/NEON: 15
	//   - AVX2: 20
	//   - AVX-512: 30
	Priority int

	// AddBlock performs element-wise addition: dst[i] = a[i] + b[i].
	AddBlock func(dst, a, b []float64)

	// AddBlockInPlace performs in-place element-wise addition: dst[i] += src[i].
	AddBlockInPlace func(dst, src []float64)

	// MulBlock performs element-wise multiplication: dst[i] = a[i] * b[i].
	MulBlock func(dst, a, b []float64)

	// MulBlockInPlace performs in-place element-wise multiplication: dst[i] *= src[i].
	MulBlockInPlace func(dst, src []float64)

	// ScaleBlock performs element-wise scaling: dst[i] = src[i] * scalar.
	ScaleBlock func(dst, src []float64, scalar float64)

	// ScaleBlockInPlace performs in-place element-wise scaling: dst[i] *= scalar.
	ScaleBlockInPlace func(dst []float64, scalar float64)

	// AddMulBlock performs fused add-multiply: dst[i] = a[i] + b[i] * scalar.
	AddMulBlock func(dst, a, b []float64, scalar float64)

	// MulAddBlock performs fused multiply-add: dst[i] = a[i] * b[i] + c[i].
	MulAddBlock func(dst, a, b, c []float64)

	// MaxAbs returns the maximum absolute value in the slice: max(|x[i]|).
	MaxAbs func(x []float64) float64

	// Sum returns the sum of all elements in the slice: sum(x[i]).
	Sum func(x []float64) float64

	// DotProduct returns the dot product of two slices: sum(a[i] * b[i]).
	DotProduct func(a, b []float64) float64

	// Magnitude computes magnitude from separate real and imaginary parts: dst[i] = sqrt(re[i]^2 + im[i]^2).
	Magnitude func(dst, re, im []float64)

	// Power computes power (magnitude squared) from separate real and imaginary parts: dst[i] = re[i]^2 + im[i]^2.
	Power func(dst, re, im []float64)
}

// OpRegistry manages the registration and lookup of vecmath implementation variants.
//
// Implementations register themselves via init() functions. At runtime, Lookup()
// selects the highest-priority implementation compatible with the current CPU.
type OpRegistry struct {
	mu      sync.RWMutex
	entries []OpEntry
	sorted  bool // true if entries are sorted by priority (descending)
}

// Global is the default registry instance used by all vecmath operations.
var Global = &OpRegistry{}

// Register adds an implementation variant to the registry.
//
// This function is typically called from init() functions in architecture-specific
// implementation packages. It is safe to call concurrently, but all registrations
// should complete before the first call to Lookup().
func (r *OpRegistry) Register(entry OpEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries = append(r.entries, entry)
	r.sorted = false
}

// Lookup finds the best implementation variant for the given CPU features.
//
// Returns the highest-priority entry compatible with the CPU. If no compatible
// implementations are found, returns nil (which should never happen if a generic
// fallback is registered).
//
// This function is thread-safe and performs lazy sorting of entries on first call.
func (r *OpRegistry) Lookup(features cpu.Features) *OpEntry {
	r.mu.Lock()
	if !r.sorted {
		// Sort entries by priority (descending) for efficient lookup
		r.sortByPriority()
		r.sorted = true
	}
	r.mu.Unlock()

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Find highest priority compatible implementation
	for i := range r.entries {
		entry := &r.entries[i]
		if cpu.Supports(features, entry.SIMDLevel) {
			return entry
		}
	}

	return nil // Should never happen if generic fallback is registered
}

// sortByPriority sorts entries by priority in descending order.
// Must be called with r.mu held (write lock).
func (r *OpRegistry) sortByPriority() {
	// Simple insertion sort (registry is small, ~3-5 entries)
	for i := 1; i < len(r.entries); i++ {
		key := r.entries[i]
		j := i - 1
		for j >= 0 && r.entries[j].Priority < key.Priority {
			r.entries[j+1] = r.entries[j]
			j--
		}
		r.entries[j+1] = key
	}
}

// ListEntries returns a copy of all registered entries, sorted by priority.
// This function is primarily intended for testing and debugging.
func (r *OpRegistry) ListEntries() []OpEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries := make([]OpEntry, len(r.entries))
	copy(entries, r.entries)
	return entries
}

// Reset clears all registered entries.
// This function is intended for testing purposes only.
func (r *OpRegistry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries = nil
	r.sorted = false
}
