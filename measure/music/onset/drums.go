package onset

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/measure/music/features"
)

// DrumRules are the band-share thresholds of [ClassifyDrums]. Band indices
// refer to features.Frames.Bands; a share is the summed band power of the
// listed bands divided by the power of all bands, over Frames frames
// starting at the onset frame.
type DrumRules struct {
	// Frames is the number of frames summed from the onset frame on.
	Frames int
	// KickBands are the low bands of a kick.
	KickBands []int
	// KickShare is the minimum KickBands share of a kick.
	KickShare float64
	// HatBands are the top bands of a hat.
	HatBands []int
	// HatShare is the minimum HatBands share of a hat.
	HatShare float64
	// BodyBands are the low/mid bands a hat must lack.
	BodyBands []int
	// BodyMax is the maximum BodyBands share of a hat.
	BodyMax float64
}

// DefaultDrumRules returns the rules for the five default bands
// (25–140, 140–400, 400–2000, 2000–6000, 6000–12000 Hz) over the first
// 30 ms (three 10 ms frames): kick if band 0 holds >= 45% of the power, hat
// if band 4 holds >= 40% and bands 0+1 hold < 15%, snare otherwise.
func DefaultDrumRules() DrumRules {
	return DrumRules{
		Frames:    3,
		KickBands: []int{0},
		KickShare: 0.45,
		HatBands:  []int{4},
		HatShare:  0.4,
		BodyBands: []int{0, 1},
		BodyMax:   0.15,
	}
}

func (r DrumRules) validate(bands int) error {
	if r.Frames < 1 {
		return fmt.Errorf("%w: drum rules use %d frames", ErrInvalidArgument, r.Frames)
	}

	for _, set := range [][]int{r.KickBands, r.HatBands, r.BodyBands} {
		for _, b := range set {
			if b < 0 || b >= bands {
				return fmt.Errorf("%w: drum rules use band %d of %d", ErrInvalidArgument, b, bands)
			}
		}
	}

	for _, v := range []float64{r.KickShare, r.HatShare, r.BodyMax} {
		if math.IsNaN(v) {
			return fmt.Errorf("%w: NaN drum share", ErrInvalidArgument)
		}
	}

	return nil
}

// DrumOption configures [ClassifyDrums].
type DrumOption func(*DrumRules) error

// WithDrumRules replaces the default rules. Use it for band layouts other
// than the five default bands.
func WithDrumRules(r DrumRules) DrumOption {
	return func(dst *DrumRules) error {
		*dst = r

		return nil
	}
}

// ClassifyDrums returns a copy of events with Kind set to KindKick,
// KindSnare or KindHat from the band shares of the frames starting at
// round(Time·SampleRate/Hop). Events without any band power are labelled
// snare. The labels are a spectral-shape heuristic meant for a drum stem.
//
// With the default rules f must have at least five bands.
func ClassifyDrums(events []Event, f *features.Frames, opts ...DrumOption) ([]Event, error) {
	rules := DefaultDrumRules()

	for i, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("%w: option %d", ErrNilOption, i)
		}

		err := opt(&rules)
		if err != nil {
			return nil, err
		}
	}

	if f == nil || len(f.Bands) == 0 {
		return nil, fmt.Errorf("%w: no bands", ErrInvalidArgument)
	}

	err := f.Validate()
	if err != nil {
		return nil, fmt.Errorf("onset: %w", err)
	}

	err = rules.validate(len(f.Bands))
	if err != nil {
		return nil, err
	}

	frames := len(f.Bands[0])
	for b, band := range f.Bands {
		if len(band) != frames {
			return nil, fmt.Errorf("%w: band %d has %d frames, band 0 has %d", ErrInvalidArgument, b, len(band), frames)
		}
	}

	power := make([]float64, len(f.Bands))

	out := append([]Event(nil), events...)

	for i, e := range out {
		frame := int(math.Round(f.FramePosition(e.Time)))

		clear(power)

		total := 0.0

		for j := max(0, frame); j < min(frames, frame+rules.Frames); j++ {
			for b := range power {
				p := f.Bands[b][j] * f.Bands[b][j]
				power[b] += p
				total += p
			}
		}

		out[i].Kind = rules.classify(power, total)
	}

	return out, nil
}

func (r DrumRules) classify(power []float64, total float64) Kind {
	share := func(bands []int) float64 {
		s := 0.0
		for _, b := range bands {
			s += power[b]
		}

		return s / total
	}

	switch {
	case total <= 0:
		return KindSnare
	case share(r.KickBands) >= r.KickShare:
		return KindKick
	case share(r.HatBands) >= r.HatShare && share(r.BodyBands) < r.BodyMax:
		return KindHat
	default:
		return KindSnare
	}
}
