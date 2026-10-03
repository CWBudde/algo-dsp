//nolint:all // Verbatim reference copy; kept byte-for-byte close to the original.
package rhythm_test

// Verbatim copy (adapted to standalone functions) of novelty, tempoScore,
// EstimateRhythm and EstimateDownbeat from
// AudioVisualizer/internal/audioanalysis/rhythm.go. The parity tests compare
// this package against it bit for bit. The Evidence/Meter strings are
// app-specific and omitted.

import (
	"math"
	"sort"
)

const (
	SampleRate = 24000
	FFTSize    = 2048
	Hop        = 240
)

type Event struct {
	Time     float64
	Strength float64
	Kind     string
}

type TempoCandidate struct {
	BPM         float64
	Correlation float64
}

type Rhythm struct {
	BPM                float64
	BeatOrigin         float64
	Candidates         []TempoCandidate
	Beats              []float64
	Downbeat           int
	MedianOnsetErrorMS float64
}

type refTrack struct {
	Duration float64
	RMS      []float64
	Flux     []float64
	Bands    [5][]float64
	Events   []Event
}

func percentile(x []float64, p float64) float64 {
	if len(x) == 0 {
		return 0
	}
	y := append([]float64(nil), x...)
	sort.Float64s(y)
	return y[int(math.Round(p*float64(len(y)-1)))]
}

func novelty(x []float64) []float64 {
	y := make([]float64, len(x))
	for i, v := range x {
		lo, hi := max(0, i-30), min(len(x), i+31)
		mean := 0.0
		for j := lo; j < hi; j++ {
			mean += x[j]
		}
		mean /= float64(hi - lo)
		y[i] = math.Max(0, v-mean)
	}
	return y
}

func tempoScore(x []float64, bpm float64) float64 {
	lag := 60 * SampleRate / (bpm * Hop)
	s, aa, bb := 0.0, 0.0, 0.0
	for _, multiple := range []float64{1, 2, 4, 8} {
		shift := lag * multiple
		for i := int(math.Ceil(shift)); i < len(x); i++ {
			position := float64(i) - shift
			j := int(position)
			fraction := position - float64(j)
			v := x[j]*(1-fraction) + x[min(j+1, len(x)-1)]*fraction
			s += x[i] * v
			aa += x[i] * x[i]
			bb += v * v
		}
	}
	if aa*bb <= 1e-15 {
		return 0
	}
	return s / math.Sqrt(aa*bb)
}

func refEstimateRhythm(t *refTrack, preferredBPM float64) Rhythm {
	x := novelty(t.Flux)
	all := []TempoCandidate{}
	for bpm := 60.0; bpm <= 180; bpm += 0.5 {
		all = append(all, TempoCandidate{bpm, tempoScore(x, bpm)})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Correlation > all[j].Correlation })
	candidates := []TempoCandidate{}
	for _, c := range all {
		near := false
		for _, p := range candidates {
			if math.Abs(c.BPM-p.BPM) < 2 {
				near = true
			}
		}
		if !near {
			candidates = append(candidates, c)
		}
		if len(candidates) == 6 {
			break
		}
	}
	best := TempoCandidate{BPM: preferredBPM}
	for bpm := preferredBPM - 3; bpm <= preferredBPM+3; bpm += 0.01 {
		score := tempoScore(x, bpm)
		if score > best.Correlation {
			best = TempoCandidate{bpm, score}
		}
	}
	if best.Correlation == 0 {
		return Rhythm{Candidates: candidates}
	}
	period := 60 / best.BPM
	low := make([]float64, len(t.RMS))
	for i := 1; i < len(low); i++ {
		low[i] = math.Max(0, t.Bands[0][i]-t.Bands[0][i-1])
	}
	phase, bestPhaseScore := 0.0, -1.0
	for p := 0.0; p < period; p += 0.001 {
		score := 0.0
		for time := p; time < t.Duration; time += period {
			pos := time * SampleRate / Hop
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
	r := Rhythm{BPM: best.BPM, BeatOrigin: phase, Candidates: candidates}
	for time := phase; time < t.Duration; time += period {
		r.Beats = append(r.Beats, time)
	}
	errors := []float64{}
	for _, e := range t.Events {
		p := period / 4
		distance := math.Abs(e.Time - phase - math.Round((e.Time-phase)/p)*p)
		errors = append(errors, distance*1000)
	}
	r.MedianOnsetErrorMS = percentile(errors, 0.5)
	return r
}

func refEstimateDownbeat(r Rhythm, drums, bass []Event) int {
	if len(r.Beats) < 4 {
		return 0
	}
	var score [4]float64
	add := func(events []Event, kind string, weight float64) {
		for _, e := range events {
			if kind != "" && e.Kind != kind {
				continue
			}
			i := sort.SearchFloat64s(r.Beats, e.Time)
			for _, b := range []int{i - 1, i} {
				if b >= 0 && b < len(r.Beats) && math.Abs(e.Time-r.Beats[b]) < 0.06 {
					score[b%4] += weight * e.Strength
				}
			}
		}
	}
	add(drums, "kick", 1)
	add(bass, "", 1)
	best := 0
	for o := range score {
		if score[o] > score[best] {
			best = o
		}
	}
	return best
}
