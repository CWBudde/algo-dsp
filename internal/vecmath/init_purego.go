//go:build purego

package vecmath

import (
	// Generic implementations (pure Go fallback)
	_ "github.com/cwbudde/algo-dsp/internal/vecmath/arch/generic"
	// Import registry package
	_ "github.com/cwbudde/algo-dsp/internal/vecmath/registry"
)
