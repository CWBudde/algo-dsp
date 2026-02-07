package registry

import (
	"sync"

	"github.com/cwbudde/algo-dsp/internal/cpu"
)

// Coefficients are biquad transfer coefficients (a0 normalized to 1).
type Coefficients struct {
	B0, B1, B2 float64
	A1, A2     float64
}

// ProcessBlockFn processes buf in-place with one biquad section.
type ProcessBlockFn func(c Coefficients, d0, d1 float64, buf []float64) (newD0, newD1 float64)

// OpEntry is one registered biquad kernel implementation.
type OpEntry struct {
	Name         string
	SIMDLevel    cpu.SIMDLevel
	Priority     int
	ProcessBlock ProcessBlockFn
}

// OpRegistry stores available implementations.
type OpRegistry struct {
	mu      sync.RWMutex
	entries []OpEntry
	sorted  bool
}

// Global is the default biquad kernel registry.
var Global = &OpRegistry{}

// Register adds an implementation entry.
func (r *OpRegistry) Register(entry OpEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries = append(r.entries, entry)
	r.sorted = false
}

// Lookup returns the highest-priority implementation supported by features.
func (r *OpRegistry) Lookup(features cpu.Features) *OpEntry {
	r.mu.Lock()
	if !r.sorted {
		r.sortByPriority()
		r.sorted = true
	}
	r.mu.Unlock()

	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := range r.entries {
		entry := &r.entries[i]
		if cpu.Supports(features, entry.SIMDLevel) {
			return entry
		}
	}

	return nil
}

func (r *OpRegistry) sortByPriority() {
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

// ListEntries returns a copy of entries for tests/debugging.
func (r *OpRegistry) ListEntries() []OpEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries := make([]OpEntry, len(r.entries))
	copy(entries, r.entries)
	return entries
}

// Reset clears all entries. Intended for tests.
func (r *OpRegistry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries = nil
	r.sorted = false
}
