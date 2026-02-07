//go:build !amd64 && !arm64

package vecmath

// This file imports generic implementation packages for unsupported architectures.

import (
	// Generic implementations (pure Go fallback)
	_ "github.com/cwbudde/algo-dsp/internal/vecmath/arch/generic"
	// Import registry package
	_ "github.com/cwbudde/algo-dsp/internal/vecmath/registry"
)
