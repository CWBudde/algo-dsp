package motif

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/cwbudde/algo-dsp/measure/music/melody"
)

// pixelParade is testdata/pixelparade.json: AudioVisualizer's motif inputs
// and outputs for its PixelParade song, taken from analysis/story.json and
// analysis/features.json. Notes are [start, end, slot, slots, midi,
// strength]; energy holds the value of every energy query Score makes.
type pixelParade struct {
	Grid struct {
		BPM      float64 `json:"bpm"`
		Origin   float64 `json:"origin"`
		Downbeat int     `json:"downbeat"`
		Duration float64 `json:"duration"`
	} `json:"grid"`
	Lead       [][]float64   `json:"lead"`
	Bass       [][]float64   `json:"bass"`
	BeatChroma [][12]float64 `json:"beatChroma"`
	Sections   []struct {
		Name  string  `json:"name"`
		Start float64 `json:"start"`
		End   float64 `json:"end"`
	} `json:"sections"`
	Energy []struct {
		Track string  `json:"track"`
		Start float64 `json:"start"`
		End   float64 `json:"end"`
		Value float64 `json:"value"`
	} `json:"energy"`
	Motifs     json.RawMessage `json:"motifs"`
	Leitmotifs []string        `json:"leitmotifs"`
}

func loadPixelParade(t *testing.T) (song, pixelParade) {
	t.Helper()

	data, err := os.ReadFile("testdata/pixelparade.json")
	if err != nil {
		t.Fatal(err)
	}

	var fx pixelParade

	err = json.Unmarshal(data, &fx)
	if err != nil {
		t.Fatal(err)
	}

	g, r := newGrid(t, fx.Grid.BPM, fx.Grid.Origin, fx.Grid.Downbeat, fx.Grid.Duration)
	notes := func(rows [][]float64) []melody.CleanNote {
		out := make([]melody.CleanNote, len(rows))
		for i, row := range rows {
			out[i] = melody.CleanNote{Start: row[0], End: row[1], Slot: int(row[2]), Slots: int(row[3]), MIDI: int(row[4]), Strength: row[5]}
		}

		return out
	}

	type query struct {
		track      string
		start, end float64
	}

	energy := map[query]float64{}
	for _, e := range fx.Energy {
		energy[query{e.Track, e.Start, e.End}] = e.Value
	}

	s := song{name: "PixelParade", grid: g, ref: r, lead: notes(fx.Lead), bass: notes(fx.Bass), chroma: fx.BeatChroma}
	for _, c := range fx.Sections {
		s.sections = append(s.sections, Span{Name: c.Name, Start: c.Start, End: c.End})
	}

	// The app scores lead motifs with the "other" stem's energy, bass
	// motifs with the bass stem's and chroma motifs with "other" again.
	s.energy = func(source string) func(t0, t1 float64) float64 {
		track := "other"
		if source == "bass" {
			track = "bass"
		}

		return func(t0, t1 float64) float64 {
			v, ok := energy[query{track, t0, t1}]
			if !ok {
				t.Errorf("energy query %s [%v, %v) not in the fixture", track, t0, t1)

				return math.NaN()
			}

			return v
		}
	}

	return s, fx
}

// TestPixelParadeReproducesStory runs the story pipeline on the real song
// and compares the motifs with AudioVisualizer's analysis/story.json byte
// for byte, and with the reference copy.
func TestPixelParadeReproducesStory(t *testing.T) {
	t.Parallel()

	s, fx := loadPixelParade(t)
	got := pipeline(t, s)

	if want := refPipeline(s, refDefaultMotifParams()); !reflect.DeepEqual(toRef(got), want) {
		t.Fatal("pipeline differs from the reference copy")
	}

	data, err := json.Marshal(toRef(got))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data, fx.Motifs) {
		t.Fatalf("motifs differ from story.json:\ngot  %s\nwant %s", data, fx.Motifs)
	}

	leitmotifs := make([]string, 0, len(fx.Leitmotifs))

	for r := 1; r <= len(got); r++ {
		for _, m := range got {
			if m.Leitmotif && m.Rank == r {
				leitmotifs = append(leitmotifs, m.ID)
			}
		}
	}

	if !reflect.DeepEqual(leitmotifs, fx.Leitmotifs) {
		t.Fatalf("leitmotifs %v, story.json has %v", leitmotifs, fx.Leitmotifs)
	}

	t.Logf("%d motifs, leitmotifs %v", len(got), leitmotifs)
}
