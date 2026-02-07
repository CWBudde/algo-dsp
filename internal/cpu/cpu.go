// Package cpu provides CPU feature detection for DSP kernel selection.
//
// This package detects SIMD instruction set extensions (SSE2, AVX2, NEON) available
// on the current processor and caches the results for efficient querying.
//
// Detection is performed lazily on the first call to DetectFeatures() and the
// results are cached for subsequent calls using sync.Once for thread-safety.
package cpu

import (
	"sync"
)

// SIMDLevel represents a SIMD instruction set extension level.
// Higher numeric values generally indicate more advanced SIMD capabilities,
// but levels are not strictly comparable across architectures (e.g., AVX2 vs NEON).
type SIMDLevel int

const (
	// SIMDNone indicates no SIMD optimization (pure Go fallback).
	SIMDNone SIMDLevel = iota

	// SIMDSSE2 indicates x86-64 SSE2 (baseline for amd64).
	SIMDSSE2

	// SIMDAVX indicates x86-64 AVX (Advanced Vector Extensions).
	SIMDAVX

	// SIMDAVX2 indicates x86-64 AVX2 (256-bit integer operations).
	SIMDAVX2

	// SIMDAVX512 indicates x86-64 AVX-512 (512-bit vectors, future).
	SIMDAVX512

	// SIMDNEON indicates ARM NEON / Advanced SIMD.
	SIMDNEON

	// SIMDSVELTE indicates ARM SVE (Scalable Vector Extension, future).
	SIMDSVELTE
)

// String returns a human-readable name for the SIMD level.
func (s SIMDLevel) String() string {
	switch s {
	case SIMDNone:
		return "None"
	case SIMDSSE2:
		return "SSE2"
	case SIMDAVX:
		return "AVX"
	case SIMDAVX2:
		return "AVX2"
	case SIMDAVX512:
		return "AVX-512"
	case SIMDNEON:
		return "NEON"
	case SIMDSVELTE:
		return "SVE"
	default:
		return "Unknown"
	}
}

// Features describes CPU capabilities relevant to DSP kernel selection.
type Features struct {
	// x86/amd64 SIMD features
	HasSSE2   bool // Streaming SIMD Extensions 2 (baseline for amd64)
	HasAVX    bool // Advanced Vector Extensions
	HasAVX2   bool // Advanced Vector Extensions 2
	HasAVX512 bool // Advanced Vector Extensions 512 (future)

	// ARM SIMD features
	HasNEON bool // ARM Advanced SIMD (NEON)

	// Control flags
	ForceGeneric bool // Disable all SIMD optimizations (for testing/debugging)

	// Runtime information
	Architecture string // runtime.GOARCH (e.g., "amd64", "arm64")
}

var (
	// detectedFeatures holds the cached CPU features detected on this system.
	detectedFeatures Features

	// detectOnce ensures feature detection runs exactly once, thread-safely.
	detectOnce sync.Once

	// detectMutex serializes access to detectOnce/detectedFeatures.
	detectMutex sync.Mutex

	// forcedFeatures allows overriding actual hardware detection for testing.
	forcedFeatures *Features

	// forcedMutex protects forcedFeatures from concurrent access during testing.
	forcedMutex sync.RWMutex
)

// DetectFeatures returns the CPU features available on the current system.
//
// Detection is performed once on the first call and cached for subsequent calls.
// This function is thread-safe and can be called concurrently from multiple goroutines.
func DetectFeatures() Features {
	forcedMutex.RLock()
	forced := forcedFeatures
	forcedMutex.RUnlock()

	if forced != nil {
		return *forced
	}

	detectMutex.Lock()
	detectOnce.Do(func() {
		detectedFeatures = detectFeaturesImpl()
	})
	features := detectedFeatures
	detectMutex.Unlock()

	return features
}

// HasAVX2 returns true if the CPU supports AVX2 instructions.
func HasAVX2() bool {
	return DetectFeatures().HasAVX2
}

// HasSSE2 returns true if the CPU supports SSE2 instructions.
func HasSSE2() bool {
	return DetectFeatures().HasSSE2
}

// HasNEON returns true if the CPU supports ARM NEON (Advanced SIMD) instructions.
func HasNEON() bool {
	return DetectFeatures().HasNEON
}

// SetForcedFeatures overrides CPU feature detection with the specified features.
// This is intended for testing purposes only.
func SetForcedFeatures(f Features) {
	forcedMutex.Lock()
	defer forcedMutex.Unlock()
	forced := f
	forcedFeatures = &forced
}

// ResetDetection clears any forced features and the detection cache.
// This is intended for testing purposes.
func ResetDetection() {
	forcedMutex.Lock()
	forcedFeatures = nil
	forcedMutex.Unlock()

	detectMutex.Lock()
	detectOnce = sync.Once{}
	detectedFeatures = Features{}
	detectMutex.Unlock()
}

// Supports returns true if the given CPU features support the specified SIMD level.
// This function is used by the vecmath registry to determine implementation compatibility.
func Supports(features Features, level SIMDLevel) bool {
	if features.ForceGeneric {
		return level == SIMDNone
	}

	switch level {
	case SIMDNone:
		return true
	case SIMDSSE2:
		return features.HasSSE2
	case SIMDAVX:
		return features.HasAVX
	case SIMDAVX2:
		return features.HasAVX2
	case SIMDAVX512:
		return features.HasAVX512
	case SIMDNEON:
		return features.HasNEON
	case SIMDSVELTE:
		// SVE not yet supported
		return false
	default:
		return false
	}
}
