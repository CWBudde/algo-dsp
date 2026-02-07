//go:build arm64 && !purego

package vecmath

// This file imports arm64-specific implementation packages to trigger
// their init() functions, which register implementations with the global registry.

import (
	// ARM64 implementations
	_ "github.com/cwbudde/algo-dsp/internal/vecmath/arch/arm64/neon"
	// Generic implementations (pure Go fallback)
	_ "github.com/cwbudde/algo-dsp/internal/vecmath/arch/generic"
	// Import registry package
	_ "github.com/cwbudde/algo-dsp/internal/vecmath/registry"
)
