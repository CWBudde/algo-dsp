package melody

import (
	"fmt"
	"math"
	"os"
	"sort"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/cqt"
	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
)

// trackError summarises the deviation of a pitch track from the true pitch
// over the evaluated frames.
type trackError struct {
	mean, p95, max float64 // over the voiced evaluated frames, in semitones
	voiced         float64 // share of the evaluated frames that are voiced
}

func (e trackError) String() string {
	return fmt.Sprintf("mean %.3f p95 %.2f max %.2f voiced %.2f", e.mean, e.p95, e.max, e.voiced)
}

// measureTrack compares est with truth, the true MIDI pitch per frame; frames
// where truth is 0 are not evaluated.
func measureTrack(est, truth []float64) trackError {
	var errs []float64

	frames := 0

	for i, want := range truth {
		if want == 0 {
			continue
		}

		frames++

		if i < len(est) && est[i] != 0 {
			errs = append(errs, math.Abs(est[i]-want))
		}
	}

	if len(errs) == 0 {
		return trackError{mean: math.Inf(1), p95: math.Inf(1), max: math.Inf(1)}
	}

	sort.Float64s(errs)

	sum := 0.0
	for _, e := range errs {
		sum += e
	}

	return trackError{
		mean:   sum / float64(len(errs)),
		p95:    errs[int(math.Ceil(0.95*float64(len(errs))))-1],
		max:    errs[len(errs)-1],
		voiced: float64(len(errs)) / float64(frames),
	}
}

// tone is a harmonic tone at a fractional MIDI pitch: harmonics 1..count
// with amplitude amp/h, linear fades of fade seconds.
type tone struct {
	start, end, midi float64
}

func addTone(x []float64, sampleRate float64, n tone, count int, amp, fade float64) {
	f0 := pitch.MIDIToFrequency(n.midi, pitch.DefaultReferenceHz)
	fadeN := fade * sampleRate
	start, end := int(n.start*sampleRate), int(n.end*sampleRate)

	for i := start; i < end && i < len(x); i++ {
		env := math.Min(1, math.Min(float64(i-start)/fadeN, float64(end-i)/fadeN))
		t := float64(i-start) / sampleRate

		for h := 1; h <= count; h++ {
			x[i] += env * amp / float64(h) * math.Sin(2*math.Pi*f0*float64(h)*t)
		}
	}
}

// glideSignal is a harmonic tone (harmonics 1..6, amplitude 0.3/h) that
// holds from for 0.3 s, glides exponentially (linearly in MIDI) to to over
// glide seconds and holds again, starting at 0.1 s. The truth covers the
// glide only; the holds give it context away from the note edges.
func glideSignal(sampleRate float64, hop int, from, to, glide float64) ([]float64, []float64) {
	const start, hold = 0.1, 0.3

	x := make([]float64, int((start+2*hold+glide+0.1)*sampleRate))
	midiAt := func(t float64) float64 {
		switch {
		case t < start+hold:
			return from
		case t < start+hold+glide:
			return from + (to-from)*(t-start-hold)/glide
		default:
			return to
		}
	}

	phase := 0.0
	fade := 0.005 * sampleRate
	first, last := int(start*sampleRate), int((start+2*hold+glide)*sampleRate)

	for i := first; i < last; i++ {
		env := math.Min(1, math.Min(float64(i-first)/fade, float64(last-i)/fade))

		phase += 2 * math.Pi * pitch.MIDIToFrequency(midiAt(float64(i)/sampleRate), pitch.DefaultReferenceHz) / sampleRate
		for h := 1; h <= 6; h++ {
			x[i] += env * 0.3 / float64(h) * math.Sin(float64(h)*phase)
		}
	}

	truth := make([]float64, (len(x)+hop-1)/hop)

	for i := range truth {
		if t := float64(i*hop) / sampleRate; t >= start+hold && t <= start+hold+glide {
			truth[i] = midiAt(t)
		}
	}

	return x, truth
}

// toneSignal renders tones and returns the true pitch of every frame at
// least margin seconds inside a tone.
func toneSignal(sampleRate float64, hop int, seconds, margin float64, tones []tone, count int, amp, fade float64,
) ([]float64, []float64) {
	x := make([]float64, int(seconds*sampleRate))
	for _, n := range tones {
		addTone(x, sampleRate, n, count, amp, fade)
	}

	truth := make([]float64, (len(x)+hop-1)/hop)

	for i := range truth {
		t := float64(i*hop) / sampleRate
		for _, n := range tones {
			if t >= n.start+margin && t <= n.end-margin {
				truth[i] = n.midi
			}
		}
	}

	return x, truth
}

// notesCorrect counts the tones matched by a detected note of the nearest
// MIDI number that overlaps it.
func notesCorrect(got []Note, want []tone) int {
	correct := 0

	for _, w := range want {
		for _, g := range got {
			if g.MIDI == int(math.Round(w.midi)) && g.Start < w.end && g.End > w.start {
				correct++

				break
			}
		}
	}

	return correct
}

// detune returns tones shifted by a fixed pattern of off-grid offsets
// (-0.46..+0.41 semitone), so that neither the FFT bins nor the CQT bins
// line up with the partials.
func detune(tones []tone) []tone {
	offsets := []float64{0.27, -0.33, 0.41, -0.18, 0.12, -0.46}
	out := make([]tone, len(tones))

	for i, n := range tones {
		n.midi += offsets[i%len(offsets)]
		out[i] = n
	}

	return out
}

var (
	// lowTones are E2..C3 (82 to 131 Hz), below the default candidates.
	lowTones = []tone{
		{0.10, 0.50, 40}, // E2
		{0.60, 1.00, 43}, // G2
		{1.10, 1.50, 45}, // A2
		{1.60, 2.00, 41}, // F2
		{2.10, 2.50, 48}, // C3
		{2.60, 3.00, 47}, // B2
	}
	// bassTones are E1..E2 (41 to 82 Hz), for BassPreset.
	bassTones = []tone{
		{0.10, 0.60, 28}, // E1
		{0.70, 1.20, 31}, // G1
		{1.30, 1.80, 33}, // A1
		{1.90, 2.40, 35}, // B1
		{2.50, 3.00, 30}, // F#1
		{3.10, 3.60, 40}, // E2
	}
)

// lowOptions extends the candidates down to C2 and narrows the band to
// 50 Hz..2 kHz, so that the CQT spans 4.9 octaves, which the default hop
// allows (C2 to 5 kHz would need 7 octaves and fail with cqt.ErrHop).
func lowOptions() []Option {
	return []Option{WithMIDIRange(36, 72), WithFrequencyRange(50, 2000)}
}

type accuracyCase struct {
	name  string
	x     []float64
	truth []float64
	base  []Option
	tones []tone // nil for glides
}

func accuracyCases() []accuracyCase {
	slow, slowTruth := glideSignal(testRate, DefaultHop, 57, 69, 2)
	fast, fastTruth := glideSignal(testRate, DefaultHop, 57, 69, 0.5)
	high, highTruth := glideSignal(testRate, DefaultHop, 64, 76, 0.5)

	cases := []accuracyCase{
		{name: "glide A3-A4 2s", x: slow, truth: slowTruth},
		{name: "glide A3-A4 0.5s", x: fast, truth: fastTruth},
		{name: "glide E4-E5 0.5s", x: high, truth: highTruth},
	}

	for _, set := range []struct {
		name         string
		tones        []tone
		base         []Option
		seconds, mrg float64
		count        int
		amp, fade    float64
	}{
		{"low E2-C3", lowTones, lowOptions(), 3.2, 0.08, 6, 0.3, 0.005},
		{"low E2-C3 detuned", detune(lowTones), lowOptions(), 3.2, 0.08, 6, 0.3, 0.005},
		{"bass E1-E2", bassTones, BassPreset(), 3.8, 0.1, 10, 0.25, 0.01},
		{"bass E1-E2 detuned", detune(bassTones), BassPreset(), 3.8, 0.1, 10, 0.25, 0.01},
	} {
		x, truth := toneSignal(testRate, DefaultHop, set.seconds, set.mrg, set.tones, set.count, set.amp, set.fade)
		cases = append(cases, accuracyCase{name: set.name, x: x, truth: truth, base: set.base, tones: set.tones})
	}

	return cases
}

// TestCQTAtLeastAsAccurateAsSTFT compares the STFT default with
// WithCQT(DefaultCQTBinsPerOctave) on synthetic glides and low notes at
// 24 kHz with the default hop. The error is |estimated - true MIDI| over the
// voiced frames of the glide, or of the notes away from their edges. The CQT
// must not be worse in mean or p95 error, voiced share or correct notes; no
// slack is allowed.
//
// Recorded errors (mean / p95 / max semitones, voiced share, notes correct):
//
//	case                STFT                            CQT 36
//	glide A3-A4 2s      0.034 / 0.08 / 0.10, 1.00       0.024 / 0.04 / 0.10, 1.00
//	glide A3-A4 0.5s    0.063 / 0.22 / 0.38, 1.00       0.029 / 0.08 / 0.10, 1.00
//	glide E4-E5 0.5s    0.055 / 0.24 / 0.38, 1.00       0.029 / 0.08 / 0.10, 1.00
//	low E2-C3           0.033 / 0.10 / 0.10, 1.00, 6/6  0.000 / 0.00 / 0.00, 1.00, 6/6
//	low E2-C3 detuned   0.025 / 0.04 / 0.04, 1.00, 6/6  0.025 / 0.04 / 0.04, 1.00, 6/6
//	bass E1-E2          0.000 / 0.00 / 0.00, 1.00, 6/6  0.000 / 0.00 / 0.00, 1.00, 6/6
//	bass E1-E2 detuned  0.045 / 0.08 / 0.08, 1.00, 6/6  0.025 / 0.04 / 0.04, 1.00, 6/6
//
// The candidate grid is 0.1 semitone, so 0.025 is the floor for the detuned
// cases (both front ends reach it on the low notes). The undetuned low and
// bass notes lie on CQT bin centres, which favours the CQT; the detuned
// ones do not. The largest gain is on fast glides, where the 171 ms FFT
// frame spans 4 semitones of the glide while the CQT kernels of the upper
// harmonics are short.
func TestCQTAtLeastAsAccurateAsSTFT(t *testing.T) {
	t.Parallel()

	for _, tc := range accuracyCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stftRes, err := Analyze(tc.x, testRate, tc.base...)
			if err != nil {
				t.Fatal(err)
			}

			cqtRes, err := Analyze(tc.x, testRate, append(append([]Option(nil), tc.base...), WithCQT(DefaultCQTBinsPerOctave))...)
			if err != nil {
				t.Fatal(err)
			}

			stftErr := measureTrack(stftRes.Pitch, tc.truth)
			cqtErr := measureTrack(cqtRes.Pitch, tc.truth)

			t.Logf("STFT: %v", stftErr)
			t.Logf("CQT:  %v", cqtErr)

			if cqtErr.mean > stftErr.mean {
				t.Errorf("CQT mean error %.4f > STFT %.4f", cqtErr.mean, stftErr.mean)
			}

			if cqtErr.p95 > stftErr.p95 {
				t.Errorf("CQT p95 error %.4f > STFT %.4f", cqtErr.p95, stftErr.p95)
			}

			if cqtErr.voiced < stftErr.voiced {
				t.Errorf("CQT voiced share %.3f < STFT %.3f", cqtErr.voiced, stftErr.voiced)
			}

			if tc.tones != nil {
				s, c := notesCorrect(stftRes.Notes, tc.tones), notesCorrect(cqtRes.Notes, tc.tones)
				t.Logf("notes correct: STFT %d/%d (%d notes), CQT %d/%d (%d notes)",
					s, len(tc.tones), len(stftRes.Notes), c, len(tc.tones), len(cqtRes.Notes))

				if c < s || len(cqtRes.Notes) != len(tc.tones) {
					t.Errorf("CQT notes %+v", cqtRes.Notes)
				}
			}
		})
	}
}

// TestCQTResolutionSweep records how the CQT resolution and filter scale
// affect the mean error. It is slow and informational, so it runs only with
// MELODY_CQT_SWEEP=1:
//
//	MELODY_CQT_SWEEP=1 go test -run CQTResolutionSweep -v ./measure/music/melody/
//
// Recorded mean errors in semitones (voiced share 1.00 and every note
// correct unless noted; "-" fails with cqt.ErrHop, because at 12 bins per
// octave the extra bin below the lowest candidate is a semitone and the
// default band then needs 6 octaves):
//
//	bpo scale  A3-A4 2s  A3-A4 .5s  E4-E5 .5s  low   low det.  bass  bass det.
//	STFT       0.034     0.063      0.055      0.033 0.025     0.000 0.045
//	12  1      -         -          -          -     -         0.000 0.025
//	24  0.5    0.026     0.030      0.028      0.000 0.025     0.000 0.025
//	24  1      0.024     0.030      0.027      0.000 0.025     0.000 0.025
//	36  0.5    0.024     0.030      0.029      0.000 0.025     0.000 0.025
//	36  1      0.024     0.029      0.029      0.000 0.025     0.000 0.025
//	48  1      0.024     0.051 v.92 0.029      0.000 0.025     0.000 0.025
//	60  0.75   0.024     0.054 v.63 0.029      0.000 0.025     0.000 0.025
//	60  1      0.024     0.118 v.25 0.047 v.80 0.000 0.025     0.000 0.025
//
// 48 and 60 bins per octave lose voiced frames on the fast glides: their
// longer kernels smear the partials over more than ±2 bins. 24 bins are as
// accurate as 36 here but put every other bin between two semitones for
// chroma. 36 at the cqt default filter scale 1 is therefore the default; it
// also keeps the ±2-bin voicing window on the kernels' main lobe (a filter
// scale s widens the main lobe to ±2/s bins).
//
// Interpolation, from earlier runs of the same sweep at 36 bins per octave
// and scale 1 (the first two without the extra bin below the lowest
// candidate): the linear interpolation of the STFT front end gives 0.059 on
// the slow glide (0.245 at 12 bins), because it peaks only at bin centres; a
// cubic
// Catmull-Rom gives 0.032 but 0.029 on the detuned low notes, above the
// STFT's 0.025; Lanczos-3 without the extra bin below the lowest candidate
// gives 0.017 on the undetuned bass notes, whose E1 then sits on bin 0 with
// half of its taps outside the transform.
func TestCQTResolutionSweep(t *testing.T) {
	if os.Getenv("MELODY_CQT_SWEEP") == "" {
		t.Skip("set MELODY_CQT_SWEEP=1 to run")
	}

	cases := accuracyCases()
	row := func(label string, extra []Option) {
		line := fmt.Sprintf("%-10s", label)

		for _, tc := range cases {
			res, err := Analyze(tc.x, testRate, append(append([]Option(nil), tc.base...), extra...)...)
			if err != nil {
				line += " ERR"

				continue
			}

			e := measureTrack(res.Pitch, tc.truth)
			line += fmt.Sprintf(" %.3f(v%.2f", e.mean, e.voiced)

			if tc.tones != nil {
				line += fmt.Sprintf(" n%d/%d", notesCorrect(res.Notes, tc.tones), len(tc.tones))
			}

			line += ")"
		}

		t.Log(line)
	}

	row("STFT", nil)

	for _, bpo := range []int{12, 24, 36, 48, 60} {
		for _, scale := range []float64{0.5, 0.75, 1} {
			row(fmt.Sprintf("%d %.2f", bpo, scale), []Option{WithCQT(bpo, cqt.WithFilterScale(scale))})
		}
	}
}
