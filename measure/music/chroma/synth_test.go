package chroma

import (
	"math"
	"sort"

	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
)

// event is a chord sounding from start to end seconds: harmonic tones of the
// given MIDI notes.
type event struct {
	start, end float64
	notes      []float64
}

// synth renders events at sampleRate Hz, tuned to ref (the frequency of A4),
// into a signal of dur seconds. Every note has 4 harmonics with amplitudes
// 0.1·0.5^(h−1) and 10 ms raised-cosine fades; the output is deterministic.
func synth(sampleRate, dur, ref float64, events ...event) []float64 {
	const (
		harmonics = 4
		amp       = 0.1
		fade      = 0.01
	)

	x := make([]float64, int(math.Round(dur*sampleRate)))

	for _, ev := range events {
		from := int(math.Round(ev.start * sampleRate))
		to := min(len(x), int(math.Round(ev.end*sampleRate)))
		nFade := int(fade * sampleRate)

		for _, m := range ev.notes {
			f := pitch.MIDIToFrequency(m, ref)

			for h := 1; h <= harmonics; h++ {
				fh := f * float64(h)
				if fh >= sampleRate/2 {
					break
				}

				a := amp * math.Pow(0.5, float64(h-1))

				for i := from; i < to; i++ {
					g := 1.0
					if d := min(i-from, to-1-i); d < nFade {
						g = 0.5 - 0.5*math.Cos(math.Pi*float64(d)/float64(nFade))
					}

					x[i] += g * a * math.Sin(2*math.Pi*fh*float64(i-from)/sampleRate)
				}
			}
		}
	}

	return x
}

// sine returns n samples of amp·sin(2π·f·t).
func sine(n int, sampleRate, f, amp float64) []float64 {
	x := make([]float64, n)
	for i := range x {
		x[i] = amp * math.Sin(2*math.Pi*f*float64(i)/sampleRate)
	}

	return x
}

// ranked returns the pitch classes of v, strongest first (ties by class).
func ranked(v [12]float64) []int {
	idx := make([]int, 12)
	for i := range idx {
		idx[i] = i
	}

	sort.SliceStable(idx, func(a, b int) bool { return v[idx[a]] > v[idx[b]] })

	return idx
}

// classSet returns the sorted pitch classes of MIDI notes, without
// duplicates.
func classSet(notes ...float64) []int {
	seen := map[int]bool{}

	var out []int

	for _, m := range notes {
		pc := pitchClassOf(int(math.Round(m)))
		if !seen[pc] {
			seen[pc] = true
			out = append(out, pc)
		}
	}

	sort.Ints(out)

	return out
}

// frameOf returns the chroma vector of frame i.
func frameOf(c [12][]float64, i int) [12]float64 {
	var v [12]float64
	for pc := range c {
		v[pc] = c[pc][i]
	}

	return v
}
