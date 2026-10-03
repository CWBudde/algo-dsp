package onset_test

import (
	"errors"
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/measure/music/features"
	"github.com/cwbudde/algo-dsp/measure/music/onset"
)

func TestClassifyDrums(t *testing.T) {
	t.Parallel()

	channels := [][]float64{drumFixture()}
	f := extract(t, channels)

	events, err := onset.Detect(channels, f)
	if err != nil {
		t.Fatal(err)
	}

	labelled, err := onset.ClassifyDrums(events, f)
	if err != nil {
		t.Fatal(err)
	}

	want := []onset.Kind{onset.KindKick, onset.KindSnare, onset.KindHat}
	if len(labelled) != len(want) {
		t.Fatalf("drum events %+v", labelled)
	}

	for i, k := range want {
		if labelled[i].Kind != k {
			t.Fatalf("event %d is %v, want %v (%+v)", i, labelled[i].Kind, k, labelled)
		}
	}

	if events[0].Kind != onset.KindUnknown {
		t.Fatal("ClassifyDrums modified its input")
	}
}

func syntheticFrames(bands [][]float64) *features.Frames {
	return &features.Frames{Timing: features.Timing{SampleRate: 100, Hop: 1}, Bands: bands}
}

func TestClassifyDrumsRules(t *testing.T) {
	t.Parallel()

	// Three bands, one frame each: 0.1 s = frame 10.
	mk := func(a, b, c float64) [][]float64 {
		bands := [][]float64{make([]float64, 20), make([]float64, 20), make([]float64, 20)}
		bands[0][10], bands[1][10], bands[2][10] = a, b, c

		return bands
	}

	rules := onset.DrumRules{
		Frames: 1, KickBands: []int{0}, KickShare: 0.5,
		HatBands: []int{2}, HatShare: 0.5, BodyBands: []int{0}, BodyMax: 0.2,
	}

	tests := []struct {
		name    string
		a, b, c float64
		want    onset.Kind
	}{
		{"kick", 1, 0.1, 0.1, onset.KindKick},
		{"hat", 0.1, 0.1, 1, onset.KindHat},
		{"snare", 0.5, 1, 0.5, onset.KindSnare},
		{"hat with body", 0.6, 0, 1, onset.KindSnare},
		{"silent", 0, 0, 0, onset.KindSnare},
	}

	for _, tc := range tests {
		got, err := onset.ClassifyDrums([]onset.Event{{Time: 0.1, Strength: 1}},
			syntheticFrames(mk(tc.a, tc.b, tc.c)), onset.WithDrumRules(rules))
		if err != nil {
			t.Fatal(err)
		}

		if got[0].Kind != tc.want {
			t.Errorf("%s: %v, want %v", tc.name, got[0].Kind, tc.want)
		}
	}

	// Events beyond the last frame have no power.
	got, err := onset.ClassifyDrums([]onset.Event{{Time: 5}}, syntheticFrames(mk(1, 0, 0)), onset.WithDrumRules(rules))
	if err != nil || got[0].Kind != onset.KindSnare {
		t.Fatalf("out-of-range event: %+v %v", got, err)
	}
}

func TestClassifyDrumsErrors(t *testing.T) {
	t.Parallel()

	three := [][]float64{make([]float64, 4), make([]float64, 4), make([]float64, 4)}
	five := append(append([][]float64(nil), three...), make([]float64, 4), make([]float64, 4))
	ragged := append(append([][]float64(nil), five[:4]...), make([]float64, 3))
	badTiming := syntheticFrames(five)
	badTiming.Hop = 0
	nanRules := onset.DefaultDrumRules()
	nanRules.KickShare = math.NaN()
	noFrames := onset.DefaultDrumRules()
	noFrames.Frames = 0

	tests := []struct {
		name string
		f    *features.Frames
		opts []onset.DrumOption
		want error
	}{
		{"nil frames", nil, nil, onset.ErrInvalidArgument},
		{"no bands", syntheticFrames(nil), nil, onset.ErrInvalidArgument},
		{"default rules need five bands", syntheticFrames(three), nil, onset.ErrInvalidArgument},
		{"ragged", syntheticFrames(ragged), nil, onset.ErrInvalidArgument},
		{"timing", badTiming, nil, features.ErrInvalidConfig},
		{"nil option", syntheticFrames(five), []onset.DrumOption{nil}, onset.ErrNilOption},
		{"nan", syntheticFrames(five), []onset.DrumOption{onset.WithDrumRules(nanRules)}, onset.ErrInvalidArgument},
		{"frames", syntheticFrames(five), []onset.DrumOption{onset.WithDrumRules(noFrames)}, onset.ErrInvalidArgument},
	}

	for _, tc := range tests {
		_, err := onset.ClassifyDrums(nil, tc.f, tc.opts...)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
}
