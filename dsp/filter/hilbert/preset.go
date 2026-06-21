package hilbert

import "fmt"

// Preset selects a coefficient-count/transition design profile.
type Preset int

const (
	// PresetFast matches the legacy default and is suitable for general-purpose
	// use with low CPU cost.
	PresetFast Preset = iota
	// PresetBalanced improves quadrature/image rejection in the low-mid band at
	// moderate additional CPU cost.
	PresetBalanced
	// PresetLowFrequency prioritizes low-frequency quadrature accuracy and image
	// rejection at higher CPU cost.
	PresetLowFrequency
)

func (p Preset) String() string {
	switch p {
	case PresetFast:
		return "fast"
	case PresetBalanced:
		return "balanced"
	case PresetLowFrequency:
		return "low_frequency"
	default:
		return "unknown"
	}
}

// PresetConfig returns coefficient count and transition bandwidth for a preset.
func PresetConfig(preset Preset) (numberOfCoeffs int, transition float64, err error) {
	switch preset {
	case PresetFast:
		return 8, 0.1, nil
	case PresetBalanced:
		return 12, 0.06, nil
	case PresetLowFrequency:
		return 20, 0.02, nil
	default:
		return 0, 0, fmt.Errorf("hilbert: invalid preset: %d", preset)
	}
}

// New64Preset creates a 64-bit processor using a preset profile.
func New64Preset(preset Preset) (*Processor64, error) {
	n, tr, err := PresetConfig(preset)
	if err != nil {
		return nil, err
	}

	return New64(n, tr)
}

// New32Preset creates a 32-bit processor using a preset profile.
func New32Preset(preset Preset) (*Processor32, error) {
	n, tr, err := PresetConfig(preset)
	if err != nil {
		return nil, err
	}

	return New32(n, tr)
}
