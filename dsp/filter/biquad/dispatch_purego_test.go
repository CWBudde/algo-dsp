//go:build purego

package biquad

import (
	"testing"

	archregistry "github.com/cwbudde/algo-dsp/dsp/filter/biquad/internal/arch/registry"
	"github.com/cwbudde/algo-dsp/internal/cpu"
)

func TestProcessBlockDispatch_PuregoUsesGeneric(t *testing.T) {
	entry := archregistry.Global.Lookup(cpu.Features{
		Architecture: "amd64",
		ForceGeneric: true,
	})
	if entry == nil {
		t.Fatal("Lookup returned nil")
	}
	if entry.Name != "generic" {
		t.Fatalf("expected generic implementation in purego, got %q", entry.Name)
	}
}
