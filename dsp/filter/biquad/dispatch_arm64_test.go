//go:build arm64 && !purego

package biquad

import (
	"sync"
	"testing"

	archregistry "github.com/cwbudde/algo-dsp/dsp/filter/biquad/internal/arch/registry"
	"github.com/cwbudde/algo-vecmath/cpu"
)

func resetProcessBlockDispatchForTest() {
	processBlockImpl = nil
	processBlockInitOnce = sync.Once{}
}

func TestProcessBlockDispatch_ARM64Modes(t *testing.T) {
	tests := []struct {
		name     string
		features cpu.Features
		wantImpl string
	}{
		{
			name: "generic-forced",
			features: cpu.Features{
				ForceGeneric: true,
				Architecture: "arm64",
			},
			wantImpl: "generic",
		},
		{
			name: "neon",
			features: cpu.Features{
				HasNEON:      true,
				Architecture: "arm64",
			},
			wantImpl: "neon",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpu.SetForcedFeatures(tt.features)
			defer cpu.ResetDetection()
			resetProcessBlockDispatchForTest()

			entry := archregistry.Global.Lookup(cpu.DetectFeatures())
			if entry == nil {
				t.Fatal("Lookup returned nil")
			}
			if entry.Name != tt.wantImpl {
				t.Fatalf("expected %q, got %q", tt.wantImpl, entry.Name)
			}

			coeff := Coefficients{B0: 0.25, B1: 0.5, B2: 0.25, A1: -0.2, A2: 0.04}
			sRef := NewSection(coeff)
			sGot := NewSection(coeff)
			input := []float64{1, 0.5, -0.3, 0.7, 0, -1, 0.2, 0.8, -0.1}
			ref := make([]float64, len(input))
			for i, x := range input {
				ref[i] = sRef.ProcessSample(x)
			}

			got := append([]float64(nil), input...)
			sGot.ProcessBlock(got)

			for i := range got {
				if !almostEqual(got[i], ref[i], eps) {
					t.Fatalf("sample %d mismatch: got %.15f, want %.15f", i, got[i], ref[i])
				}
			}
		})
	}
}
