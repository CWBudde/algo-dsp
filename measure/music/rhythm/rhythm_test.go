package rhythm_test

import (
	"errors"
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/measure/music/features"
	"github.com/cwbudde/algo-dsp/measure/music/onset"
	"github.com/cwbudde/algo-dsp/measure/music/rhythm"
)

var timing = features.Timing{SampleRate: SampleRate, Hop: Hop}

// adapterRhythm is how the AudioVisualizer's EstimateRhythm maps onto this
// package.
func adapterRhythm(t *testing.T, flux, low, eventTimes []float64, duration, prior float64) Rhythm {
	t.Helper()

	tempo, err := rhythm.EstimateTempo(rhythm.Novelty(flux, rhythm.DefaultNoveltyRadius), timing, rhythm.WithPrior(prior))
	if err != nil {
		t.Fatal(err)
	}

	r := Rhythm{}
	for _, c := range tempo.Candidates {
		r.Candidates = append(r.Candidates, TempoCandidate(c))
	}

	if tempo.BPM == 0 {
		return r
	}

	r.BPM = tempo.BPM
	r.BeatOrigin = rhythm.FitBeatPhase(low, timing, tempo.BPM, duration)
	r.Beats = rhythm.BeatGrid(r.BeatOrigin, tempo.BPM, duration)
	r.MedianOnsetErrorMS = rhythm.GridError(eventTimes, r.BeatOrigin, tempo.BPM, 4) * 1000

	return r
}

func requireRhythm(t *testing.T, got, want Rhythm) {
	t.Helper()

	if got.BPM != want.BPM || got.BeatOrigin != want.BeatOrigin || got.MedianOnsetErrorMS != want.MedianOnsetErrorMS {
		t.Fatalf("rhythm %v/%v/%v, want %v/%v/%v", got.BPM, got.BeatOrigin, got.MedianOnsetErrorMS,
			want.BPM, want.BeatOrigin, want.MedianOnsetErrorMS)
	}

	if len(got.Candidates) != len(want.Candidates) || len(got.Beats) != len(want.Beats) {
		t.Fatalf("%d candidates/%d beats, want %d/%d", len(got.Candidates), len(got.Beats), len(want.Candidates), len(want.Beats))
	}

	for i := range want.Candidates {
		if got.Candidates[i] != want.Candidates[i] {
			t.Fatalf("candidate %d = %+v, want %+v", i, got.Candidates[i], want.Candidates[i])
		}
	}

	for i := range want.Beats {
		if got.Beats[i] != want.Beats[i] {
			t.Fatalf("beat %d = %v, want %v", i, got.Beats[i], want.Beats[i])
		}
	}
}

type fixtureTrack struct {
	ref    *refTrack
	frames *features.Frames
	events []onset.Event
}

func analyse(t *testing.T, channels [][]float64) fixtureTrack {
	t.Helper()

	f, err := features.Extract(channels, features.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	events, err := onset.Detect(channels, f)
	if err != nil {
		t.Fatal(err)
	}

	events, err = onset.ClassifyDrums(events, f)
	if err != nil {
		t.Fatal(err)
	}

	ref := &refTrack{Duration: float64(len(channels[0])) / SampleRate, RMS: f.RMS, Flux: f.Flux}
	copy(ref.Bands[:], f.Bands)

	for _, e := range events {
		ref.Events = append(ref.Events, Event{Time: e.Time, Strength: e.Strength, Kind: e.Kind.String()})
	}

	return fixtureTrack{ref: ref, frames: f, events: events}
}

func times(events []Event) []float64 {
	out := make([]float64, len(events))
	for i, e := range events {
		out[i] = e.Time
	}

	return out
}

func pulseTrack() *refTrack {
	track := &refTrack{Duration: 20, RMS: make([]float64, 2000), Flux: make([]float64, 2000)}
	for b := range track.Bands {
		track.Bands[b] = make([]float64, 2000)
	}

	for i := 10; i < 2000; i += 50 {
		track.Flux[i] = 1
		track.Bands[0][i] = 1
		track.Events = append(track.Events, Event{Time: float64(i) * 0.01, Strength: 1})
	}

	return track
}

func TestRhythmParity(t *testing.T) {
	t.Parallel()

	music := analyse(t, musicFixture(8))
	silent := &refTrack{Duration: 1, RMS: make([]float64, 100), Flux: make([]float64, 100)}

	for b := range silent.Bands {
		silent.Bands[b] = make([]float64, 100)
	}

	tests := []struct {
		name  string
		track *refTrack
		prior float64
	}{
		{"music 120", music.ref, 120},
		{"music 105", music.ref, 105},
		{"music 90", music.ref, 90},
		{"pulse 120", pulseTrack(), 120},
		{"pulse 117", pulseTrack(), 117},
		{"silence", silent, 105},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			want := refEstimateRhythm(tc.track, tc.prior)
			got := adapterRhythm(t, tc.track.Flux, tc.track.Bands[0], times(tc.track.Events), tc.track.Duration, tc.prior)
			requireRhythm(t, got, want)
		})
	}
}

func TestDownbeatParity(t *testing.T) {
	t.Parallel()

	music := analyse(t, musicFixture(8))
	bass := analyse(t, [][]float64{musicFixture(8)[1]})
	r := refEstimateRhythm(music.ref, 120)

	accents := func(drums, bass []Event) []rhythm.Accent {
		var out []rhythm.Accent

		for _, e := range drums {
			if e.Kind == "kick" {
				out = append(out, rhythm.Accent{Time: e.Time, Weight: e.Strength})
			}
		}

		for _, e := range bass {
			out = append(out, rhythm.Accent{Time: e.Time, Weight: e.Strength})
		}

		return out
	}

	// Shifted beat grids exercise every phase.
	for shift := range 4 {
		rr := r
		rr.Beats = r.Beats[shift:]
		want := refEstimateDownbeat(rr, music.ref.Events, bass.ref.Events)
		got := rhythm.Downbeat(rr.Beats, accents(music.ref.Events, bass.ref.Events), 4, 0.06)

		if got != want {
			t.Fatalf("shift %d: downbeat %d, want %d", shift, got, want)
		}
	}
}

func TestTempoRecoversKnownPulseTrain(t *testing.T) {
	t.Parallel()

	track := pulseTrack()

	for _, opts := range [][]rhythm.Option{
		{rhythm.WithPrior(120)},
		{rhythm.WithPrior(118)},
		nil, // centred on the best broad-scan candidate
	} {
		tempo, err := rhythm.EstimateTempo(rhythm.Novelty(track.Flux, 30), timing, opts...)
		if err != nil {
			t.Fatal(err)
		}

		if math.Abs(tempo.BPM-120) > 0.1 {
			t.Fatalf("tempo %.3f", tempo.BPM)
		}

		if len(tempo.Candidates) != rhythm.DefaultCandidates {
			t.Fatalf("%d candidates", len(tempo.Candidates))
		}

		phase := rhythm.FitBeatPhase(track.Bands[0], timing, tempo.BPM, track.Duration)
		if e := rhythm.GridError(times(track.Events), phase, tempo.BPM, 4); e > 0.012 {
			t.Fatalf("bad grid fit %.2f ms", e*1000)
		}
	}
}

func TestTempoFromAudio(t *testing.T) {
	t.Parallel()

	// The fixture has kicks every 0.5 s and bursts on the off-beats.
	music := analyse(t, musicFixture(8))

	tempo, err := rhythm.EstimateTempo(rhythm.Novelty(music.frames.Flux, 30), music.frames.Timing)
	if err != nil {
		t.Fatal(err)
	}

	// 120 BPM or its double-time reading (off-beat bursts).
	if math.Abs(tempo.BPM-120) > 0.5 && math.Abs(tempo.BPM-240) > 1 {
		t.Fatalf("tempo %.2f, candidates %+v", tempo.BPM, tempo.Candidates)
	}
}

func TestSilenceHasNoTempo(t *testing.T) {
	t.Parallel()

	tempo, err := rhythm.EstimateTempo(make([]float64, 500), timing, rhythm.WithPrior(105))
	if err != nil {
		t.Fatal(err)
	}

	if tempo.BPM != 0 || tempo.Correlation != 0 {
		t.Fatalf("invented tempo for silence: %+v", tempo)
	}

	if beats := rhythm.BeatGrid(0, tempo.BPM, 5); beats != nil {
		t.Fatalf("beats %v", beats)
	}

	if rhythm.FitBeatPhase(make([]float64, 10), timing, 0, 1) != 0 || rhythm.GridError([]float64{1}, 0, 0, 4) != 0 {
		t.Fatal("zero tempo produced a grid")
	}
}

func TestTempoOptions(t *testing.T) {
	t.Parallel()

	x := rhythm.Novelty(pulseTrack().Flux, 30)

	tempo, err := rhythm.EstimateTempo(x, timing,
		rhythm.WithRange(100, 140, 1), rhythm.WithCandidates(2, 5), rhythm.WithRefinement(0, 0.01))
	if err != nil {
		t.Fatal(err)
	}

	if tempo.BPM != 120 || len(tempo.Candidates) != 2 || math.Abs(tempo.Candidates[0].BPM-tempo.Candidates[1].BPM) < 5 {
		t.Fatalf("%+v", tempo)
	}

	if tempo.Correlation != rhythm.TempoScore(x, timing, 120) {
		t.Fatal("correlation is not the TempoScore of the BPM")
	}
}

func TestTempoErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		timing features.Timing
		opts   []rhythm.Option
		want   error
	}{
		{"nil option", timing, []rhythm.Option{nil}, rhythm.ErrNilOption},
		{"range", timing, []rhythm.Option{rhythm.WithRange(100, 50, 1)}, rhythm.ErrInvalidArgument},
		{"range step", timing, []rhythm.Option{rhythm.WithRange(50, 100, 0)}, rhythm.ErrInvalidArgument},
		{"candidates", timing, []rhythm.Option{rhythm.WithCandidates(0, 2)}, rhythm.ErrInvalidArgument},
		{"merge", timing, []rhythm.Option{rhythm.WithCandidates(1, math.NaN())}, rhythm.ErrInvalidArgument},
		{"prior", timing, []rhythm.Option{rhythm.WithPrior(-1)}, rhythm.ErrInvalidArgument},
		{"refine", timing, []rhythm.Option{rhythm.WithRefinement(-1, 0.01)}, rhythm.ErrInvalidArgument},
		{"refine step", timing, []rhythm.Option{rhythm.WithRefinement(1, 0)}, rhythm.ErrInvalidArgument},
		{"timing", features.Timing{SampleRate: 1}, nil, features.ErrInvalidConfig},
	}

	for _, tc := range tests {
		_, err := rhythm.EstimateTempo(nil, tc.timing, tc.opts...)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}

	if rhythm.TempoScore([]float64{1, 2}, features.Timing{}, 120) != 0 {
		t.Error("TempoScore with invalid timing")
	}
}

func TestNovelty(t *testing.T) {
	t.Parallel()

	got := rhythm.Novelty([]float64{0, 0, 3, 0, 0}, 1)
	want := []float64{0, 0, 2, 0, 0}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Novelty = %v, want %v", got, want)
		}
	}

	if rhythm.Novelty([]float64{1, 2}, -5)[1] != 0 {
		t.Fatal("radius 0 novelty not zero")
	}
}

func TestDownbeatFollowsAccentedBeat(t *testing.T) {
	t.Parallel()

	var (
		beats   []float64
		accents []rhythm.Accent
	)

	for b := range 32 {
		time := 0.1 + float64(b)*0.5
		beats = append(beats, time)

		weight := 0.5
		if b%4 == 1 {
			weight = 1
		}

		accents = append(accents, rhythm.Accent{Time: time + 0.004, Weight: weight})
	}

	if got := rhythm.Downbeat(beats, accents, 4, 0.06); got != 1 {
		t.Fatalf("downbeat index %d, want 1", got)
	}

	if got := rhythm.Downbeat(beats, accents, 3, 0.06); got != 0 && got != 1 && got != 2 {
		t.Fatalf("3/4 downbeat %d", got)
	}

	if rhythm.Downbeat(beats[:3], accents, 4, 0.06) != 0 || rhythm.Downbeat(beats, accents, 0, 0.06) != 0 {
		t.Fatal("degenerate downbeat not 0")
	}

	if rhythm.Downbeat(beats, accents, 4, 0.001) != 0 {
		t.Fatal("accents outside the tolerance counted")
	}
}
