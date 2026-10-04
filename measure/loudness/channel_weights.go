package loudness

import "fmt"

// BS1770ChannelWeights returns power weights for selected source channels in
// packed order. channels must be in [1,8]; nil selects every channel. Five-channel
// order is L,R,C,Ls,Rs; six-channel order is L,R,C,LFE,Ls,Rs. Surround channels
// have weight 1.41 and LFE has zero weight. Other counts use discrete unit weights
// because their count alone does not identify a speaker layout. Applications
// with a different layout must provide explicit IntegratedConfig.ChannelWeights.
// An LFE-only selection returns zero; it has no defined programme loudness.
func BS1770ChannelWeights(channels int, selected []int) ([]float64, error) {
	if channels < 1 || channels > 8 {
		return nil, fmt.Errorf("BS.1770 channel count: %w", ErrInvalid)
	}

	if selected == nil {
		selected = make([]int, channels)
		for i := range selected {
			selected[i] = i
		}
	}

	if len(selected) == 0 {
		return nil, fmt.Errorf("BS.1770 empty selection: %w", ErrInvalid)
	}

	weights := make([]float64, len(selected))

	seen := uint(0)
	for i, channel := range selected {
		if channel < 0 || channel >= channels || seen&(1<<channel) != 0 {
			return nil, fmt.Errorf("BS.1770 source channel: %w", ErrInvalid)
		}

		seen |= 1 << channel

		weights[i] = 1
		if channels == 5 && channel >= 3 || channels == 6 && channel >= 4 {
			weights[i] = 1.41
		}

		if channels == 6 && channel == 3 {
			weights[i] = 0
		}
	}

	return weights, nil
}
