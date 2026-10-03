//nolint:all // Verbatim reference copy; kept byte-for-byte close to the original.
package onset_test

// Verbatim copy (adapted to standalone functions) of DetectOnsets and
// ClassifyDrums from AudioVisualizer/internal/audioanalysis/features.go.
// The parity tests compare this package against it bit for bit.

import (
	"math"
	"sort"
)

const (
	SampleRate = 24000
	FFTSize    = 2048
	Hop        = 240
)

type refEvent struct {
	Time     float64
	Strength float64
	Kind     string
}

type refTrack struct {
	RMS    []float64
	Flux   []float64
	Bands  [5][]float64
	Events []refEvent
}

func percentile(x []float64, p float64) float64 {
	if len(x) == 0 {
		return 0
	}
	y := append([]float64(nil), x...)
	sort.Float64s(y)
	return y[int(math.Round(p*float64(len(y)-1)))]
}

func refDetectOnsets(channels [][]float64, t *refTrack) []refEvent {
	step := SampleRate / 200
	attack := make([]float64, (len(channels[0])+step-1)/step)
	prev := 0.0
	for i := range attack {
		e, count := 0.0, 0.0
		for j := i * step; j < min(len(channels[0]), (i+1)*step); j++ {
			for _, ch := range channels {
				e += ch[j] * ch[j]
				count++
			}
		}
		r := math.Sqrt(e / math.Max(1, count))
		attack[i] = math.Max(0, r-prev)
		prev = r
	}
	active := make([]float64, 0, len(t.Flux))
	for _, v := range t.Flux {
		if v > 0 {
			active = append(active, v)
		}
	}
	hi := percentile(active, 0.95)
	if hi <= 0 {
		return nil
	}
	candidates := []refEvent{}
	for i := 1; i < len(t.Flux)-1; i++ {
		if t.RMS[i] < 0.001 || t.Flux[i] < 0.12*hi || t.Flux[i] <= t.Flux[i-1] || t.Flux[i] < t.Flux[i+1] {
			continue
		}
		lo, end := max(0, i-25), min(len(t.Flux), i+26)
		avg := 0.0
		for j := lo; j < end; j++ {
			avg += t.Flux[j]
		}
		avg /= float64(end - lo)
		if t.Flux[i] < 1.35*avg {
			continue
		}
		position := float64(i) * Hop / SampleRate
		best := 0.0
		for j := max(0, i*2-10); j < min(len(attack), i*2+11); j++ {
			if attack[j] > best {
				best = attack[j]
				position = float64(j*step) / SampleRate
			}
		}
		candidates = append(candidates, refEvent{Time: position, Strength: math.Min(1, t.Flux[i]/hi)})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Strength > candidates[j].Strength })
	events := []refEvent{}
	for _, e := range candidates {
		keep := true
		for _, existing := range events {
			if math.Abs(e.Time-existing.Time) < 0.075 {
				keep = false
				break
			}
		}
		if keep {
			events = append(events, e)
		}
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Time < events[j].Time })
	return events
}

func refClassifyDrums(t *refTrack) {
	for i, e := range t.Events {
		frame := int(math.Round(e.Time * SampleRate / Hop))
		var power [5]float64
		total := 0.0
		for j := frame; j < min(len(t.RMS), frame+3); j++ {
			for b := range power {
				p := t.Bands[b][j] * t.Bands[b][j]
				power[b] += p
				total += p
			}
		}
		kind := "snare"
		switch {
		case total <= 0:
		case power[0]/total >= 0.45:
			kind = "kick"
		case power[4]/total >= 0.4 && (power[0]+power[1])/total < 0.15:
			kind = "hat"
		}
		t.Events[i].Kind = kind
	}
}
