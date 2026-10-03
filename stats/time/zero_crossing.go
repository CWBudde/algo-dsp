package time

import "math"

// NearestZeroCrossing finds the nearest candidate sample index within radius
// samples of target, choosing the earlier index when distances are equal.
// A candidate is an exact finite zero (including negative zero), or the second
// sample of a pair of finite samples with strictly opposite signs. NaN and
// infinity do not form crossings, and gaps containing them are not bridged.
//
// Target may be anywhere in [0, len(signal)], including the position after the
// last sample. Candidates are actual sample indices, never synthetic EOF zeros.
// Empty signals, invalid targets, negative radii, and windows without candidates
// return (0, false). The inclusive search window is clipped to the signal without
// integer overflow. Only that window and its immediate predecessor are read.
// The function does not allocate or modify signal.
func NearestZeroCrossing[T ~float32 | ~float64](signal []T, target, radius int) (index int, found bool) {
	if len(signal) == 0 || target < 0 || target > len(signal) || radius < 0 {
		return 0, false
	}

	start := 0
	if radius < target {
		start = target - radius
	}

	end := len(signal) - 1
	if target < end && radius < end-target {
		end = target + radius
	}

	bestDistance := int(^uint(0) >> 1)

	for candidate := start; candidate <= end; candidate++ {
		value := float64(signal[candidate])
		if math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}

		crossing := value == 0
		if !crossing && candidate > 0 {
			previous := float64(signal[candidate-1])
			crossing = !math.IsNaN(previous) && !math.IsInf(previous, 0) &&
				((previous < 0 && value > 0) || (previous > 0 && value < 0))
		}

		if !crossing {
			continue
		}

		distance := target - candidate
		if candidate > target {
			distance = candidate - target
		}

		if !found || distance < bestDistance {
			index, found, bestDistance = candidate, true, distance
		}
	}

	return index, found
}
