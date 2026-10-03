package rhythm_test

import (
	"errors"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/cwbudde/algo-dsp/measure/music/rhythm"
)

type gridCase struct {
	name     string
	rhythm   storyRhythm
	duration float64
}

func gridCases() []gridCase {
	measured, err := rhythm.BeatGrid(0.217, 97.3, 40)
	if err != nil {
		panic(err)
	}

	return []gridCase{
		{name: "pixelparade", rhythm: storyRhythm{BPM: 105, BeatOrigin: 0.038}, duration: 30},
		{name: "measured-beats", rhythm: storyRhythm{BPM: 97.3, BeatOrigin: 0.217, Beats: measured, Downbeat: 2}, duration: 40},
		{name: "downbeat-1", rhythm: storyRhythm{BPM: 128, BeatOrigin: 0.5, Downbeat: 1}, duration: 12},
		{name: "downbeat-3", rhythm: storyRhythm{BPM: 140.25, BeatOrigin: 0.001, Downbeat: 3}, duration: 17.3},
		{name: "downbeat-6", rhythm: storyRhythm{BPM: 88, BeatOrigin: 1.25, Downbeat: 6}, duration: 9},
		{name: "downbeat-4", rhythm: storyRhythm{BPM: 120, BeatOrigin: 0, Downbeat: 4}, duration: 8},
		{name: "empty", rhythm: storyRhythm{BPM: 120, BeatOrigin: 5}, duration: 0},
	}
}

func newGrid(t *testing.T, r storyRhythm, duration float64) rhythm.Grid {
	t.Helper()

	var opts []rhythm.GridOption
	if len(r.Beats) > 0 {
		opts = append(opts, rhythm.WithBeats(r.Beats))
	}

	g, err := rhythm.NewGrid(r.BPM, r.BeatOrigin, r.Downbeat, duration, opts...)
	if err != nil {
		t.Fatal(err)
	}

	return g
}

func sameBits(a, b float64) bool { return math.Float64bits(a) == math.Float64bits(b) }

func sameSlice(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if !sameBits(a[i], b[i]) {
			return false
		}
	}

	return true
}

// TestGridParity compares every value and method of rhythm.Grid with the
// verbatim copy of AudioVisualizer's story grid bit for bit.
func TestGridParity(t *testing.T) {
	t.Parallel()

	for _, tc := range gridCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			want := newStoryGrid(tc.rhythm, tc.duration)
			got := newGrid(t, tc.rhythm, tc.duration)

			if !sameBits(got.BPM(), want.BPM) || !sameBits(got.Origin(), want.OriginSeconds) ||
				!sameBits(got.BeatSeconds(), want.BeatSeconds) || !sameBits(got.SlotSeconds(), want.SixteenthSeconds) ||
				!sameBits(got.BarSeconds(), want.BarSeconds) || got.BeatsPerBar() != want.BeatsPerBar ||
				got.Downbeat() != want.Downbeat || !sameBits(got.Duration(), want.Duration) {
				t.Fatalf("grid values differ:\n got %+v\nwant %+v", got, want)
			}

			if !sameSlice(got.Beats(), want.Beats) || !sameSlice(got.BarStarts(), want.BarStarts) || got.Bars() != want.Bars() {
				t.Fatalf("beats or bar starts differ:\n got %v %v\nwant %v %v", got.Beats(), got.BarStarts(), want.Beats, want.BarStarts)
			}

			rng := rand.New(rand.NewPCG(45, uint64(len(tc.name))))
			for range 2000 {
				x := -3 + (tc.duration+6)*rng.Float64()
				n := rng.IntN(400) - 50

				if got.Slot(x) != want.Slot(x) || got.Bar(x) != want.Bar(x) ||
					!sameBits(got.BarPosition(x), want.BarPosition(x)) ||
					!sameBits(got.SlotTime(n), want.SlotTime(n)) || !sameBits(got.BarStart(n), want.BarStart(n)) ||
					!sameBits(got.BeatStart(n), want.BeatStart(n)) {
					t.Fatalf("t=%v n=%d: methods differ", x, n)
				}

				if got.Tick(x, storyPPQ) != want.Tick(x, false) || got.Tick(x-got.Origin(), storyPPQ) != want.Tick(x, true) {
					t.Fatalf("t=%v: Tick differs", x)
				}
			}

			// Exact grid lines (and a hair before them) exercise the rounding
			// and the 1e-9 bar tolerance.
			for n := -8; n < 200; n++ {
				for _, x := range []float64{want.SlotTime(n), want.BarStart(n), math.Nextafter(want.BarStart(n), math.Inf(-1))} {
					if got.Slot(x) != want.Slot(x) || got.Bar(x) != want.Bar(x) || !sameBits(got.BarPosition(x), want.BarPosition(x)) {
						t.Fatalf("grid line n=%d t=%v differs", n, x)
					}
				}
			}
		})
	}
}

// TestGridPickupAndDownbeat checks the pickup bar and the downbeat offset.
func TestGridPickupAndDownbeat(t *testing.T) {
	t.Parallel()

	// 120 BPM: beat 0.5 s, bar 2 s. The first downbeat is beat 3 at 2.0 s,
	// so bar 0 is a pickup from 0.5 s to 2.0 s (beats 0..2) that starts a
	// full bar before the downbeat, at 0.0 s.
	g, err := rhythm.NewGrid(120, 0.5, 3, 10)
	if err != nil {
		t.Fatal(err)
	}

	if g.BarStart(0) != 0 || g.BarStart(1) != 2 || g.Bars() != 5 {
		t.Fatalf("bar starts %v, %d bars", g.BarStarts(), g.Bars())
	}

	for _, c := range []struct {
		t    float64
		bar  int
		slot int
	}{
		{0.5, 0, 0},    // beat 0: pickup bar
		{1.875, 0, 11}, // last pickup sixteenth
		{2.0, 1, 12},   // first downbeat
		{3.99, 1, 28},  // end of bar 1
		{4.0, 2, 28},   // bar 2
		{-1, 0, -12},   // before the grid: clamped bar
	} {
		if g.Bar(c.t) != c.bar || g.Slot(c.t) != c.slot {
			t.Fatalf("t=%v: bar %d slot %d, want bar %d slot %d", c.t, g.Bar(c.t), g.Slot(c.t), c.bar, c.slot)
		}
	}

	if p := g.BarPosition(1.5); p != 0.75 {
		t.Fatalf("bar position %v, want 0.75", p)
	}

	// Downbeat 0 and 4 (one bar later) give the same bars without pickup.
	for _, db := range []int{0, 4} {
		g, err := rhythm.NewGrid(120, 0.5, db, 10)
		if err != nil {
			t.Fatal(err)
		}

		if g.BarStart(0) != 0.5 || g.Bar(0.5) != 0 || g.Bar(2.5) != 1 {
			t.Fatalf("downbeat %d: bar starts %v", db, g.BarStarts())
		}
	}

	// 3/4 with eighth-note slots: bar 1.5 s, slot 0.25 s; downbeat on beat 1
	// puts the pickup bar at 0.5 - 1.0 = -0.5 s.
	g, err = rhythm.NewGrid(120, 0.5, 1, 4, rhythm.WithBeatsPerBar(3), rhythm.WithSubdivisions(2))
	if err != nil {
		t.Fatal(err)
	}

	if g.BarSeconds() != 1.5 || g.SlotSeconds() != 0.25 || g.SlotsPerBar() != 6 || g.BarStart(0) != -0.5 ||
		g.Bar(1.0) != 1 || g.Slot(1.0) != 2 || g.Subdivisions() != 2 || g.BeatsPerBar() != 3 {
		t.Fatalf("3/4 grid: %+v", g)
	}
}

// TestGridJitterIsStable checks that times jittered by up to ±30 ms keep
// their slot at 105 BPM (sixteenth 143 ms).
func TestGridJitterIsStable(t *testing.T) {
	t.Parallel()

	g, err := rhythm.NewGrid(105, 0.038, 0, 60)
	if err != nil {
		t.Fatal(err)
	}

	rng := rand.New(rand.NewPCG(30, 30))
	for slot := range 400 {
		x := g.SlotTime(slot) + 0.06*rng.Float64() - 0.03
		if g.Slot(x) != slot {
			t.Fatalf("slot %d jittered to %v maps to %d", slot, x, g.Slot(x))
		}
	}
}

func TestGridBeatsFromBeatGrid(t *testing.T) {
	t.Parallel()

	beats, err := rhythm.BeatGrid(0.1, 120, 3)
	if err != nil {
		t.Fatal(err)
	}

	g, err := rhythm.NewGrid(120, 0.1, 0, 3, rhythm.WithBeats(beats))
	if err != nil {
		t.Fatal(err)
	}

	got := g.Beats()
	if !sameSlice(got, beats) {
		t.Fatalf("beats %v, want %v", got, beats)
	}

	got[0] = 99

	if g.Beats()[0] != 0.1 {
		t.Fatal("Beats returned shared storage")
	}

	plain, err := rhythm.NewGrid(120, 0.1, 0, 3)
	if err != nil {
		t.Fatal(err)
	}

	if len(plain.Beats()) != 6 || plain.Beats()[5] != plain.BeatStart(5) {
		t.Fatalf("generated beats %v", plain.Beats())
	}
}

func TestGridErrors(t *testing.T) {
	t.Parallel()

	nan, inf := math.NaN(), math.Inf(1)

	for _, c := range []struct {
		name     string
		bpm, org float64
		downbeat int
		duration float64
		opts     []rhythm.GridOption
	}{
		{"zero bpm", 0, 0, 0, 10, nil},
		{"nan bpm", nan, 0, 0, 10, nil},
		{"inf bpm", inf, 0, 0, 10, nil},
		{"subnormal bpm", math.SmallestNonzeroFloat64, 0, 0, 10, nil},
		{"nan origin", 120, nan, 0, 10, nil},
		{"negative downbeat", 120, 0, -1, 10, nil},
		{"negative duration", 120, 0, 0, -1, nil},
		{"inf duration", 120, 0, 0, inf, nil},
		{"too many beats", 1e-3, -1e12, 0, 1e12, nil},
		{"beats per bar", 120, 0, 0, 10, []rhythm.GridOption{rhythm.WithBeatsPerBar(0)}},
		{"subdivisions", 120, 0, 0, 10, []rhythm.GridOption{rhythm.WithSubdivisions(0)}},
		{"nan beat", 120, 0, 0, 10, []rhythm.GridOption{rhythm.WithBeats([]float64{0, nan})}},
	} {
		_, err := rhythm.NewGrid(c.bpm, c.org, c.downbeat, c.duration, c.opts...)
		if !errors.Is(err, rhythm.ErrInvalidArgument) {
			t.Errorf("%s: error %v, want ErrInvalidArgument", c.name, err)
		}
	}

	_, err := rhythm.NewGrid(120, 0, 0, 10, nil)
	if !errors.Is(err, rhythm.ErrNilOption) {
		t.Errorf("nil option: error %v", err)
	}

	if err := (rhythm.Grid{}).Validate(); !errors.Is(err, rhythm.ErrInvalidArgument) {
		t.Errorf("zero grid validates: %v", err)
	}

	g, err := rhythm.NewGrid(120, 0, 0, 10)
	if err != nil || g.Validate() != nil {
		t.Fatalf("valid grid: %v / %v", err, g.Validate())
	}
}
