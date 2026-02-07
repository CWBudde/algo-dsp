//go:build amd64

package vecmath

// This file imports amd64-specific implementation packages to trigger
// their init() functions, which register implementations with the global registry.

import (
	// Import registry package
	_ "github.com/cwbudde/algo-dsp/internal/vecmath/registry"

	// Generic implementations (pure Go fallback)
	_ "github.com/cwbudde/algo-dsp/internal/vecmath/arch/generic"

	// AMD64 implementations
	_ "github.com/cwbudde/algo-dsp/internal/vecmath/arch/amd64/avx2"
	_ "github.com/cwbudde/algo-dsp/internal/vecmath/arch/amd64/sse2"
)
