//go:build arm64

package vecmath

// This file imports arm64-specific implementation packages to trigger
// their init() functions, which register implementations with the global registry.

import (
	// Import registry package
	_ "github.com/cwbudde/algo-dsp/internal/vecmath/registry"

	// Generic implementations (pure Go fallback)
	_ "github.com/cwbudde/algo-dsp/internal/vecmath/arch/generic"

	// ARM64 implementations
	_ "github.com/cwbudde/algo-dsp/internal/vecmath/arch/arm64/neon"
)
