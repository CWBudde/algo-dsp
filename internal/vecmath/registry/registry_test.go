package registry

import (
	"testing"

	"github.com/cwbudde/algo-dsp/internal/cpu"
)

func TestOpRegistry_Register(t *testing.T) {
	// Create a fresh registry for testing
	reg := &OpRegistry{}

	// Register a generic implementation
	genericEntry := OpEntry{
		Name:      "generic",
		SIMDLevel: cpu.SIMDNone,
		Priority:  0,
		AddBlock: func(dst, a, b []float64) {
			// Dummy implementation
		},
	}
	reg.Register(genericEntry)

	// Register an AVX2 implementation
	avx2Entry := OpEntry{
		Name:      "avx2",
		SIMDLevel: cpu.SIMDAVX2,
		Priority:  20,
		AddBlock: func(dst, a, b []float64) {
			// Dummy implementation
		},
	}
	reg.Register(avx2Entry)

	// Verify both entries were registered
	entries := reg.ListEntries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestOpRegistry_Lookup_Priority(t *testing.T) {
	// Create a fresh registry for testing
	reg := &OpRegistry{}

	// Register implementations in random order to test sorting
	reg.Register(OpEntry{
		Name:      "generic",
		SIMDLevel: cpu.SIMDNone,
		Priority:  0,
	})
	reg.Register(OpEntry{
		Name:      "avx2",
		SIMDLevel: cpu.SIMDAVX2,
		Priority:  20,
	})
	reg.Register(OpEntry{
		Name:      "sse2",
		SIMDLevel: cpu.SIMDSSE2,
		Priority:  10,
	})

	tests := []struct {
		name     string
		features cpu.Features
		want     string
	}{
		{
			name: "AVX2 available - select AVX2",
			features: cpu.Features{
				HasSSE2: true,
				HasAVX2: true,
			},
			want: "avx2",
		},
		{
			name: "SSE2 only - select SSE2",
			features: cpu.Features{
				HasSSE2: true,
				HasAVX2: false,
			},
			want: "sse2",
		},
		{
			name: "No SIMD - select generic",
			features: cpu.Features{
				HasSSE2: false,
				HasAVX2: false,
			},
			want: "generic",
		},
		{
			name: "ForceGeneric - select generic",
			features: cpu.Features{
				HasSSE2:      true,
				HasAVX2:      true,
				ForceGeneric: true,
			},
			want: "generic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := reg.Lookup(tt.features)
			if entry == nil {
				t.Fatal("Lookup returned nil")
			}
			if entry.Name != tt.want {
				t.Errorf("expected %q, got %q", tt.want, entry.Name)
			}
		})
	}
}

func TestOpRegistry_Lookup_ARM(t *testing.T) {
	reg := &OpRegistry{}

	// Register generic and NEON implementations
	reg.Register(OpEntry{
		Name:      "generic",
		SIMDLevel: cpu.SIMDNone,
		Priority:  0,
	})
	reg.Register(OpEntry{
		Name:      "neon",
		SIMDLevel: cpu.SIMDNEON,
		Priority:  15,
	})

	tests := []struct {
		name     string
		features cpu.Features
		want     string
	}{
		{
			name: "NEON available - select NEON",
			features: cpu.Features{
				HasNEON: true,
			},
			want: "neon",
		},
		{
			name: "NEON unavailable - select generic",
			features: cpu.Features{
				HasNEON: false,
			},
			want: "generic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := reg.Lookup(tt.features)
			if entry == nil {
				t.Fatal("Lookup returned nil")
			}
			if entry.Name != tt.want {
				t.Errorf("expected %q, got %q", tt.want, entry.Name)
			}
		})
	}
}

func TestSIMDLevel_String(t *testing.T) {
	tests := []struct {
		level cpu.SIMDLevel
		want  string
	}{
		{cpu.SIMDNone, "None"},
		{cpu.SIMDSSE2, "SSE2"},
		{cpu.SIMDAVX, "AVX"},
		{cpu.SIMDAVX2, "AVX2"},
		{cpu.SIMDAVX512, "AVX-512"},
		{cpu.SIMDNEON, "NEON"},
		{cpu.SIMDSVELTE, "SVE"},
		{cpu.SIMDLevel(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.level.String()
			if got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestCPU_Supports(t *testing.T) {
	tests := []struct {
		name     string
		features cpu.Features
		level    cpu.SIMDLevel
		want     bool
	}{
		{
			name:     "Generic always supported",
			features: cpu.Features{},
			level:    cpu.SIMDNone,
			want:     true,
		},
		{
			name: "SSE2 supported when HasSSE2",
			features: cpu.Features{
				HasSSE2: true,
			},
			level: cpu.SIMDSSE2,
			want:  true,
		},
		{
			name: "SSE2 not supported without HasSSE2",
			features: cpu.Features{
				HasSSE2: false,
			},
			level: cpu.SIMDSSE2,
			want:  false,
		},
		{
			name: "AVX2 supported when HasAVX2",
			features: cpu.Features{
				HasAVX2: true,
			},
			level: cpu.SIMDAVX2,
			want:  true,
		},
		{
			name: "NEON supported when HasNEON",
			features: cpu.Features{
				HasNEON: true,
			},
			level: cpu.SIMDNEON,
			want:  true,
		},
		{
			name: "ForceGeneric blocks all SIMD",
			features: cpu.Features{
				HasSSE2:      true,
				HasAVX2:      true,
				ForceGeneric: true,
			},
			level: cpu.SIMDAVX2,
			want:  false,
		},
		{
			name: "ForceGeneric allows Generic",
			features: cpu.Features{
				ForceGeneric: true,
			},
			level: cpu.SIMDNone,
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cpu.Supports(tt.features, tt.level)
			if got != tt.want {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
