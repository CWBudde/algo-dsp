//go:build !amd64 && !arm64

package cpu

import "runtime"

// detectFeaturesImpl is the fallback for other architectures.
//
// Returns a Features struct with all SIMD flags set to false,
// indicating only generic (non-SIMD) kernels should be used.
func detectFeaturesImpl() Features {
	return Features{
		Architecture: runtime.GOARCH,
	}
}
