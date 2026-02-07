package registry

import (
	"testing"

	"github.com/cwbudde/algo-vecmath/cpu"
)

func TestRegistryLookupPrefersHigherPriority(t *testing.T) {
	reg := &OpRegistry{}
	reg.Register(OpEntry{Name: "generic", SIMDLevel: cpu.SIMDNone, Priority: 0})
	reg.Register(OpEntry{Name: "sse2", SIMDLevel: cpu.SIMDSSE2, Priority: 10})
	reg.Register(OpEntry{Name: "avx2", SIMDLevel: cpu.SIMDAVX2, Priority: 20})

	entry := reg.Lookup(cpu.Features{HasSSE2: true, HasAVX2: true})
	if entry == nil || entry.Name != "avx2" {
		t.Fatalf("expected avx2, got %#v", entry)
	}

	entry = reg.Lookup(cpu.Features{HasSSE2: true})
	if entry == nil || entry.Name != "sse2" {
		t.Fatalf("expected sse2, got %#v", entry)
	}

	entry = reg.Lookup(cpu.Features{})
	if entry == nil || entry.Name != "generic" {
		t.Fatalf("expected generic, got %#v", entry)
	}
}

func TestRegistryLookupForceGeneric(t *testing.T) {
	reg := &OpRegistry{}
	reg.Register(OpEntry{Name: "generic", SIMDLevel: cpu.SIMDNone, Priority: 0})
	reg.Register(OpEntry{Name: "avx2", SIMDLevel: cpu.SIMDAVX2, Priority: 20})

	entry := reg.Lookup(cpu.Features{HasAVX2: true, ForceGeneric: true})
	if entry == nil || entry.Name != "generic" {
		t.Fatalf("expected generic with ForceGeneric, got %#v", entry)
	}
}
