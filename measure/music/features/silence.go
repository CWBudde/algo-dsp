package features

import (
	"fmt"
	"math"
)

// Interval is a time span in seconds.
type Interval struct {
	// Start is the start time in seconds.
	Start float64
	// End is the end time in seconds (exclusive).
	End float64
}

// Silence returns the intervals in which every channel stays at or below
// thresholdDB dBFS (|x| <= 10^(thresholdDB/20)) for at least minSeconds.
// Interval bounds are sample indices divided by sampleRate; an interval
// reaching the end of the signal ends at len/sampleRate. The AudioVisualizer
// uses -45 dBFS and 150 ms.
//
// The result is never nil. Channels must be non-empty and of equal length.
func Silence(channels [][]float64, sampleRate, thresholdDB, minSeconds float64) ([]Interval, error) {
	length, err := checkChannels(channels)
	if err != nil {
		return nil, err
	}

	if !(sampleRate > 0) || math.IsInf(sampleRate, 0) || math.IsNaN(thresholdDB) ||
		!(minSeconds >= 0) || math.IsInf(minSeconds, 0) {
		return nil, fmt.Errorf("%w: sample rate %v, threshold %v dB, min %v s",
			ErrInvalidArgument, sampleRate, thresholdDB, minSeconds)
	}

	threshold := math.Pow(10, thresholdDB/20)
	minSamples := FloorSamples(minSeconds * sampleRate)
	intervals := []Interval{}
	start := -1

	for i := 0; i <= length; i++ {
		quiet := i < length
		if quiet {
			for _, ch := range channels {
				if math.Abs(ch[i]) > threshold {
					quiet = false

					break
				}
			}
		}

		if quiet && start < 0 {
			start = i
		}

		if !quiet && start >= 0 {
			if i-start >= minSamples {
				intervals = append(intervals, Interval{float64(start) / sampleRate, float64(i) / sampleRate})
			}

			start = -1
		}
	}

	return intervals, nil
}

// FloorSamples truncates a non-negative sample count x (typically
// seconds·sampleRate) towards zero, tolerating the rounding error of a
// product that is mathematically integral: 0.15·24000 gives 3600, not 3599,
// and 0.005·44100 gives 220 as integer division 44100/200 would.
func FloorSamples(x float64) int {
	return int(math.Floor(x * (1 + 1e-12)))
}
