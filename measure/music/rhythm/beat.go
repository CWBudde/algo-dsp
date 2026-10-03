package rhythm

import (
	"fmt"
	"math"
	"sort"

	"github.com/cwbudde/algo-dsp/measure/music/features"
)

// PhaseStep is the resolution in seconds of [FitBeatPhase].
const PhaseStep = 0.001

// MaxBeats is the largest number of beats [BeatGrid] returns: 2^20, over 58
// hours at 300 BPM. Longer grids are rejected instead of allocated.
const MaxBeats = 1 << 20

// FitBeatPhase returns the time in [0, 60/bpm) of the first beat that best
// aligns a beat grid of bpm with the positive changes of lowBand, a
// low-frequency envelope with one value per frame of timing (for example
// features.Frames.Bands[0], kick-like energy). Candidate phases are tried in
// [PhaseStep] increments; each is scored by the sum of the positive changes,
// linearly interpolated at the beat times before duration (seconds). The
// first best phase wins. A non-positive bpm or an invalid timing returns 0.
func FitBeatPhase(lowBand []float64, timing features.Timing, bpm, duration float64) float64 {
	if !(bpm > 0) || timing.Validate() != nil {
		return 0
	}

	period := 60 / bpm

	low := make([]float64, len(lowBand))
	for i := 1; i < len(low); i++ {
		low[i] = math.Max(0, lowBand[i]-lowBand[i-1])
	}

	phase, bestPhaseScore := 0.0, -1.0

	for p := 0.0; p < period; p += PhaseStep {
		score := 0.0

		for time := p; time < duration; time += period {
			pos := time * timing.SampleRate / float64(timing.Hop)
			i := int(pos)
			fraction := pos - float64(i)

			value := 0.0
			if i < len(low) {
				value = low[i]*(1-fraction) + low[min(i+1, len(low)-1)]*fraction
			}

			score += value
		}

		if score > bestPhaseScore {
			phase = p
			bestPhaseScore = score
		}
	}

	return phase
}

// BeatGrid returns the beat times phase, phase+60/bpm, ... before duration
// seconds, accumulated by repeated addition of the period. It returns nil
// when no beat falls before duration.
//
// BeatGrid returns [ErrInvalidArgument] unless phase and duration are
// finite and bpm is positive with a finite positive period 60/bpm, and when
// the grid would hold more than [MaxBeats] beats (including a period too
// small to advance the time).
func BeatGrid(phase, bpm, duration float64) ([]float64, error) {
	period := 60 / bpm

	switch {
	case !(bpm > 0) || !(period > 0) || math.IsInf(period, 0):
		return nil, fmt.Errorf("%w: beat grid tempo %v BPM", ErrInvalidArgument, bpm)
	case math.IsNaN(phase) || math.IsInf(phase, 0):
		return nil, fmt.Errorf("%w: beat grid phase %v", ErrInvalidArgument, phase)
	case math.IsNaN(duration) || math.IsInf(duration, 0):
		return nil, fmt.Errorf("%w: beat grid duration %v", ErrInvalidArgument, duration)
	case (duration-phase)/period > MaxBeats:
		return nil, fmt.Errorf("%w: beat grid of %v s at %v BPM exceeds %d beats",
			ErrInvalidArgument, duration-phase, bpm, MaxBeats)
	}

	var beats []float64

	for time := phase; time < duration; time += period {
		if len(beats) == MaxBeats {
			return nil, fmt.Errorf("%w: beat grid at %v BPM from %v s exceeds %d beats",
				ErrInvalidArgument, bpm, phase, MaxBeats)
		}

		beats = append(beats, time)
	}

	return beats, nil
}

// GridError returns the median distance in seconds of the event times to the
// nearest point of a grid with subdivision points per beat (4 for a 16th
// grid in 4/4) anchored at phase. It returns 0 without events or for a
// non-positive bpm; a subdivision below 1 is treated as 1.
func GridError(events []float64, phase, bpm float64, subdivision int) float64 {
	if !(bpm > 0) {
		return 0
	}

	period := 60 / bpm
	p := period / float64(max(1, subdivision))

	distances := make([]float64, 0, len(events))
	for _, t := range events {
		distances = append(distances, math.Abs(t-phase-math.Round((t-phase)/p)*p))
	}

	return features.Percentile(distances, 0.5)
}

// Accent is a weighted event used by [Downbeat], for example a kick or bass
// onset with its strength as weight.
type Accent struct {
	// Time is the accent time in seconds.
	Time float64
	// Weight is the accent's contribution to its beat.
	Weight float64
}

// Downbeat returns the index in [0, beatsPerBar) of the first bar
// downbeat in beats (sorted beat times). Each accent within tolerance
// seconds (strictly) of a beat b adds its weight to phase b mod beatsPerBar;
// the first phase with the highest total wins. Accents are added in order.
// With fewer than beatsPerBar beats, or beatsPerBar < 1, Downbeat returns 0.
func Downbeat(beats []float64, accents []Accent, beatsPerBar int, tolerance float64) int {
	if beatsPerBar < 1 || len(beats) < beatsPerBar {
		return 0
	}

	score := make([]float64, beatsPerBar)

	for _, a := range accents {
		i := sort.SearchFloat64s(beats, a.Time)
		for _, b := range [...]int{i - 1, i} {
			if b >= 0 && b < len(beats) && math.Abs(a.Time-beats[b]) < tolerance {
				score[b%beatsPerBar] += a.Weight
			}
		}
	}

	best := 0

	for o := range score {
		if score[o] > score[best] {
			best = o
		}
	}

	return best
}
