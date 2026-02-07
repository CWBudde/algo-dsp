//go:build amd64

package cpu

import (
	"runtime"

	"golang.org/x/sys/cpu"
)

// detectFeaturesImpl performs CPU feature detection on amd64 systems.
//
// Uses golang.org/x/sys/cpu which provides portable CPUID access.
// SSE2 is always true on amd64 as it's part of the x86-64 baseline.
func detectFeaturesImpl() Features {
	return Features{
		HasSSE2:      cpu.X86.HasSSE2,
		HasAVX:       cpu.X86.HasAVX,
		HasAVX2:      cpu.X86.HasAVX2,
		HasAVX512:    cpu.X86.HasAVX512,
		Architecture: runtime.GOARCH,
	}
}
