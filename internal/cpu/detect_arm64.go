//go:build arm64

package cpu

import (
	"runtime"

	"golang.org/x/sys/cpu"
)

// detectFeaturesImpl performs CPU feature detection on arm64 systems.
//
// On ARMv8 (arm64), NEON is mandatory, so HasNEON should always be true.
func detectFeaturesImpl() Features {
	return Features{
		HasNEON:      cpu.ARM64.HasASIMD,
		Architecture: runtime.GOARCH,
	}
}
