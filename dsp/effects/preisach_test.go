package effects

import (
	"math"
	"testing"
)

func TestPreisachRelayGrid(t *testing.T) {
	for _, resolution := range []int{2, 3, 8, 31, 128} {
		p, err := NewPreisach(resolution)
		if err != nil {
			t.Fatal(err)
		}

		trial := newRelayGrid(resolution)

		input := []float32{0, 0, -1, 0, .5, .25, .75, 0, -.125, -.75, -.5, 1, 0}
		for i := -resolution; i <= resolution; i++ {
			input = append(input, float32((float64(i)+.5)/float64(resolution)))
		}

		for i := resolution; i >= -resolution; i-- {
			input = append(input, float32((float64(i)+.5)/float64(resolution)))
		}

		seed := uint32(1)
		for range 1024 {
			seed = seed*1664525 + 1013904223
			input = append(input, 2.5*(float32(seed>>8)/(1<<24))-1.25)
		}

		for i, sample := range input {
			want := trial.step(sample)
			if got := p.ProcessSample(sample); math.Float32bits(got) != math.Float32bits(want) {
				t.Fatalf("resolution %d, sample %d (%g): got %g, want %g", resolution, i, sample, got, want)
			}
		}

		p.Reset()

		trial = newRelayGrid(resolution)
		if got, want := p.ProcessSample(0), trial.step(0); got != want {
			t.Fatalf("resolution %d after reset: got %g, want %g", resolution, got, want)
		}
	}
}

// relayGrid represents every relay individually, including the disabled
// suffixes. It checks the row-prefix representation against the full grid.
type relayGrid struct {
	resolution int
	rows       [][]bool
	total      int
}

func newRelayGrid(resolution int) relayGrid {
	g := relayGrid{resolution: resolution, rows: make([][]bool, resolution-1)}
	for i := range g.rows {
		g.rows[i] = make([]bool, resolution-1-i)
		for j := range g.rows[i] {
			g.rows[i][j] = true
		}

		g.total += len(g.rows[i])
	}

	return g
}

func (g *relayGrid) step(input float32) float32 {
	if math.IsNaN(float64(input)) {
		input = 0
	}

	q := int(math.RoundToEven(float64(g.resolution) * float64(min(max(input, -1), 1))))
	x, y := (g.resolution+q)/2, (g.resolution-q)/2

	var positives int

	for i, row := range g.rows {
		for j := range row {
			switch {
			case x <= 0:
				row[j] = false
			case i >= y:
				row[j] = true
			case j >= x:
				row[j] = false
			}

			if row[j] {
				positives++
			}
		}
	}

	if x <= 0 {
		return -1
	}

	if y <= 0 {
		return 1
	}

	return float32(2*float64(float32(1/float64(g.total)))*float64(positives) - 1)
}

func TestPreisachBoundsAndInvalidInput(t *testing.T) {
	for _, resolution := range []int{-1, 0, 1, 32769} {
		if _, err := NewPreisach(resolution); err == nil {
			t.Fatalf("accepted invalid resolution %d", resolution)
		}
	}

	p, err := NewPreisach(128)
	if err != nil {
		t.Fatal(err)
	}

	g := newRelayGrid(128)
	for _, input := range []float32{float32(math.Inf(-1)), 0, float32(math.NaN()), float32(math.Inf(1)), 0} {
		if got, want := p.ProcessSample(input), g.step(input); got != want {
			t.Fatalf("input %g: got %g, want %g", input, got, want)
		}
	}

	if allocations := testing.AllocsPerRun(100, func() {
		p.ProcessSample(.25)
		p.ProcessSample(-.25)
	}); allocations != 0 {
		t.Fatalf("allocated %g times", allocations)
	}
}
