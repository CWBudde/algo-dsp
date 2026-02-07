//go:build amd64

package registry_test

import (
	"testing"

	"github.com/cwbudde/algo-dsp/internal/cpu"
	"github.com/cwbudde/algo-dsp/internal/vecmath/registry"

	// Import amd64-specific implementations
	_ "github.com/cwbudde/algo-dsp/internal/vecmath/arch/generic"
	_ "github.com/cwbudde/algo-dsp/internal/vecmath/arch/amd64/avx2"
	_ "github.com/cwbudde/algo-dsp/internal/vecmath/arch/amd64/sse2"
)

// TestRegistryIntegration_AMD64 verifies implementations register on amd64.
func TestRegistryIntegration_AMD64(t *testing.T) {
	entries := registry.Global.ListEntries()

	if len(entries) == 0 {
		t.Fatal("no implementations registered - init() functions not running")
	}

	t.Logf("Registered %d implementations on amd64:", len(entries))
	for _, e := range entries {
		t.Logf("  - %s (priority %d, level %s)", e.Name, e.Priority, e.SIMDLevel)
	}

	// Verify expected implementations for amd64
	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name] = true
	}

	if !names["generic"] {
		t.Error("generic implementation not registered")
	}
	if !names["avx2"] {
		t.Error("avx2 implementation not registered")
	}
	if !names["sse2"] {
		t.Error("sse2 implementation not registered")
	}

	// Test selection logic
	entry := registry.Global.Lookup(cpu.DetectFeatures())
	if entry == nil {
		t.Fatal("Lookup returned nil")
	}

	t.Logf("Selected implementation for current CPU: %s", entry.Name)

	// Verify core operations are registered
	if entry.AddBlock == nil {
		t.Errorf("%s implementation missing AddBlock", entry.Name)
	}
	if entry.MulBlock == nil {
		t.Errorf("%s implementation missing MulBlock", entry.Name)
	}
	if entry.MaxAbs == nil {
		t.Errorf("%s implementation missing MaxAbs", entry.Name)
	}
}
