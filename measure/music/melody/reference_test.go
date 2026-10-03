package melody

// This file holds a verbatim copy of AudioVisualizer's
// internal/audioanalysis/melody.go (AnalyzeMelody, midiHz, medianVoiced,
// median, segmentNotes), adapted to standalone test code: the Audio and Event
// types are replaced by plain channels and onset times, identifiers carry a
// ref prefix, and blank lines were added for the linter. The arithmetic and
// its operation order are unchanged; the parity tests compare this copy with
// the package bit for bit.

import (
	"errors"
	"math"
	"sort"

	"github.com/cwbudde/algo-dsp/dsp/window"
	fft "github.com/cwbudde/algo-fft"
)

const (
	refSampleRate = 24000
	refHop        = 240

	refMelodyFFT       = 4096
	refMelodyMinMIDI   = 52.0
	refMelodyMaxMIDI   = 96.0
	refMelodyHarmonics = 8
	refMelodyTopHz     = 5000
	refMelodyVoicing   = 0.3
	refMelodyGateDB    = -50.0
	refNoteMinFrames   = 6
	refNoteGapFrames   = 3
	refNoteJump        = 0.6
	refNoteSnap        = 0.04
)

type refNote struct {
	Start    float64
	End      float64
	MIDI     int
	Strength float64
}

type refMelody struct {
	Pitch   []float64
	Voicing []float64
	Chroma  [12][]float64
	Notes   []refNote
}

func refMIDIHz(m float64) float64 { return 440 * math.Pow(2, (m-69)/12) }

func refAnalyzeMelody(channels [][]float64, onsets []float64) (*refMelody, error) {
	if len(channels) == 0 || len(channels[0]) == 0 {
		return nil, errors.New("empty audio")
	}

	length := len(channels[0])
	count := (length + refHop - 1) / refHop

	mono := make([]float64, length)

	for _, ch := range channels {
		for i, v := range ch {
			mono[i] += v / float64(len(channels))
		}
	}

	m := &refMelody{Pitch: make([]float64, count), Voicing: make([]float64, count)}
	for c := range m.Chroma {
		m.Chroma[c] = make([]float64, count)
	}

	plan, err := fft.NewPlanReal64(refMelodyFFT)
	if err != nil {
		return nil, err
	}

	w, err := window.Hann(refMelodyFFT, window.WithPeriodic())
	if err != nil {
		return nil, err
	}

	buf := make([]float64, refMelodyFFT)
	sp := make([]complex128, refMelodyFFT/2+1)
	mag := make([]float64, len(sp))
	binsPerHz := float64(refMelodyFFT) / refSampleRate
	lo, hi := int(100*binsPerHz), int(refMelodyTopHz*binsPerHz)

	pitchClass := make([]int, len(sp))
	for k := 1; k < len(sp); k++ {
		hz := float64(k) / binsPerHz
		pitchClass[k] = ((int(math.Round(12*math.Log2(hz/440)))+69)%12 + 12) % 12
	}

	at := func(k float64) float64 {
		i := int(k)
		if i+1 >= len(mag) {
			return 0
		}

		f := k - float64(i)

		return mag[i]*(1-f) + mag[i+1]*f
	}
	gate := math.Pow(10, refMelodyGateDB/20)
	used := make([]bool, len(sp))

	for frame := 0; frame < count; frame++ {
		center := frame * refHop

		energy, n := 0.0, 0.0
		for j := max(0, center-refHop); j < min(length, center+refHop); j++ {
			energy += mono[j] * mono[j]
			n++
		}

		if math.Sqrt(energy/math.Max(1, n)) < gate {
			continue
		}

		for i := range buf {
			j := center + i - refMelodyFFT/2

			buf[i] = 0
			if j >= 0 && j < length {
				buf[i] = mono[j] * w[i]
			}
		}

		if err := plan.Forward(sp, buf); err != nil {
			return nil, err
		}

		total, chroma := 0.0, [12]float64{}

		for k := lo; k <= hi; k++ {
			mag[k] = math.Hypot(real(sp[k]), imag(sp[k]))
			p := mag[k] * mag[k]
			total += p
			chroma[pitchClass[k]] += p
		}

		for k := hi + 1; k < len(mag); k++ {
			mag[k] = 0
		}

		if total < 1e-12 {
			continue
		}

		strongest := 0.0
		for _, v := range chroma {
			strongest = math.Max(strongest, v)
		}

		for c, v := range chroma {
			m.Chroma[c][frame] = v / strongest
		}

		best, bestMIDI := 0.0, 0.0

		for midi := refMelodyMinMIDI; midi <= refMelodyMaxMIDI; midi += 0.1 {
			f0, s, weight := refMIDIHz(midi), 0.0, 1.0
			for h := 1; h <= refMelodyHarmonics && f0*float64(h) < refMelodyTopHz; h++ {
				s += weight * at(f0*float64(h)*binsPerHz)
				weight *= 0.8
			}

			if s > best {
				best, bestMIDI = s, midi
			}
		}
		// Voicing: power inside the main lobes of the chosen harmonic series.
		clear(used)

		explained := 0.0

		f0 := refMIDIHz(bestMIDI)
		for h := 1; h <= refMelodyHarmonics && f0*float64(h) < refMelodyTopHz; h++ {
			k := int(math.Round(f0 * float64(h) * binsPerHz))
			for j := max(lo, k-2); j <= min(hi, k+2); j++ {
				if !used[j] {
					used[j] = true
					explained += mag[j] * mag[j]
				}
			}
		}

		voicing := math.Min(1, explained/total)
		if voicing >= refMelodyVoicing {
			m.Pitch[frame] = bestMIDI
			m.Voicing[frame] = voicing
		}
	}

	m.Pitch = refMedianVoiced(m.Pitch, 2)
	m.Notes = refSegmentNotes(m.Pitch, m.Voicing, onsets)

	return m, nil
}

func refMedianVoiced(x []float64, radius int) []float64 {
	y := make([]float64, len(x))
	window := []float64{}

	for i, v := range x {
		if v == 0 {
			continue
		}

		window = window[:0]

		for j := max(0, i-radius); j <= min(len(x)-1, i+radius); j++ {
			if x[j] != 0 {
				window = append(window, x[j])
			}
		}

		y[i] = refMedian(window)
	}

	return y
}

func refMedian(x []float64) float64 {
	y := append([]float64(nil), x...)
	sort.Float64s(y)

	return y[len(y)/2]
}

func refSegmentNotes(pitch, voicing []float64, onsets []float64) []refNote {
	step := float64(refHop) / refSampleRate

	onsetFrames := map[int]bool{}
	for _, e := range onsets {
		onsetFrames[int(math.Round(e/step))] = true
	}

	notes := []refNote{}
	start, unvoiced := -1, 0
	values, strengths := []float64{}, []float64{}
	flush := func(end int) {
		if start >= 0 && end-start >= refNoteMinFrames {
			s := 0.0
			for _, v := range strengths {
				s += v
			}

			notes = append(notes, refNote{float64(start) * step, float64(end) * step, int(math.Round(refMedian(values))), s / float64(len(strengths))})
		}

		start, values, strengths = -1, values[:0], strengths[:0]
	}
	deviates := func(i int, reference float64) bool {
		return i < len(pitch) && pitch[i] != 0 && math.Abs(pitch[i]-reference) > refNoteJump
	}

	for i := range pitch {
		if pitch[i] == 0 {
			if start >= 0 {
				unvoiced++
				if unvoiced >= refNoteGapFrames {
					flush(i - unvoiced + 1)
				}
			}

			continue
		}

		if start >= 0 {
			current := refMedian(values)
			changed := deviates(i, current) && deviates(i+1, current) && deviates(i+2, current)

			reattack := onsetFrames[i] && i-start >= refNoteMinFrames
			if changed || reattack {
				flush(i - unvoiced)
			}
		}

		unvoiced = 0

		if start < 0 {
			start = i
		}

		values = append(values, pitch[i])
		strengths = append(strengths, voicing[i])
	}

	flush(len(pitch) - unvoiced)
	// Prefer measured attack times over the frame grid when they are close.
	for i := range notes {
		best := refNoteSnap

		for _, e := range onsets {
			if d := math.Abs(e - notes[i].Start); d <= best && e < notes[i].End {
				best = d
				notes[i].Start = e
			}
		}
	}

	return notes
}
