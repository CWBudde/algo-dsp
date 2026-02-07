//go:build arm64 && !purego

package registry_test

import (
	"testing"

	"github.com/cwbudde/algo-dsp/internal/cpu"
	// Import arm64-specific implementations
	_ "github.com/cwbudde/algo-dsp/internal/vecmath/arch/arm64/neon"
	_ "github.com/cwbudde/algo-dsp/internal/vecmath/arch/generic"
	"github.com/cwbudde/algo-dsp/internal/vecmath/registry"
)

// TestRegistryIntegration_ARM64 verifies implementations register on arm64.
func TestRegistryIntegration_ARM64(t *testing.T) {
	entries := registry.Global.ListEntries()

	if len(entries) == 0 {
		t.Fatal("no implementations registered - init() functions not running")
	}

	t.Logf("Registered %d implementations on arm64:", len(entries))
	for _, e := range entries {
		t.Logf("  - %s (priority %d, level %s)", e.Name, e.Priority, e.SIMDLevel)
	}

	// Verify expected implementations for arm64
	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name] = true
	}

	if !names["generic"] {
		t.Error("generic implementation not registered")
	}
	if !names["neon"] {
		t.Error("neon implementation not registered")
	}

	// Test selection logic
	entry := registry.Global.Lookup(cpu.DetectFeatures())
	if entry == nil {
		t.Fatal("Lookup returned nil")
	}

	t.Logf("Selected implementation for current CPU: %s", entry.Name)

	// MaxAbs should be available on NEON
	if entry.MaxAbs == nil {
		t.Errorf("%s implementation missing MaxAbs", entry.Name)
	}
}
