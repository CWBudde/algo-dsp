package features

import (
	"fmt"
	"math"
	"sort"
)

// NormalizePercentile is the percentile of the above-gate values that
// [Normalize] maps to 1.
const NormalizePercentile = 0.95

// Percentile returns the nearest-rank percentile p (0..1) of x: the element
// at index round(p·(len(x)-1)) of the sorted values. p is clamped to [0, 1];
// an empty x returns 0. x is not modified.
func Percentile(x []float64, p float64) float64 {
	if len(x) == 0 {
		return 0
	}

	if !(p > 0) {
		p = 0
	}

	p = math.Min(p, 1)

	y := append([]float64(nil), x...)
	sort.Float64s(y)

	return y[int(math.Round(p*float64(len(y)-1)))]
}

// Normalize turns an envelope into a smoothed control curve in [0, 1].
//
// The values above gate define the scale: the 95th percentile hi of those
// values maps to 1 and gate maps to 0, i.e. target = clamp((v-gate)/(hi-gate),
// 0, 1). If no value lies above the gate the result is all zeros.
// The targets are then smoothed by a one-pole attack/release follower,
//
//	y += α·(target - y),  α = 1 - exp(-(1/frameRate)/τ)
//
// with τ = attack while the target rises above the state and τ = release
// otherwise. attack and release are time constants in seconds (0 means
// instantaneous); frameRate is the envelope's frame rate in Hz
// ([Timing.FrameRate]).
func Normalize(x []float64, gate, attack, release, frameRate float64) ([]float64, error) {
	for _, v := range []float64{gate, attack, release} {
		if !(v >= 0) || math.IsInf(v, 0) {
			return nil, fmt.Errorf("%w: gate %v, attack %v, release %v", ErrInvalidArgument, gate, attack, release)
		}
	}

	if !(frameRate > 0) || math.IsInf(frameRate, 0) {
		return nil, fmt.Errorf("%w: frame rate %v", ErrInvalidArgument, frameRate)
	}

	active := make([]float64, 0, len(x))
	for _, v := range x {
		if v > gate {
			active = append(active, v)
		}
	}

	y := make([]float64, len(x))
	if len(active) == 0 {
		return y, nil
	}

	hi := Percentile(active, NormalizePercentile)

	dt := 1 / frameRate
	state := 0.0

	for i, v := range x {
		target := math.Min(1, math.Max(0, (v-gate)/(hi-gate)))

		seconds := release
		if target > state {
			seconds = attack
		}

		alpha := 1 - math.Exp(-dt/seconds)
		state += alpha * (target - state)
		y[i] = state
	}

	return y, nil
}
