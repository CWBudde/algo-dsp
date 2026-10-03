// Command structure_overview demonstrates measure/music/structure on a
// synthetic song. It builds an A B A B A′ B feature sequence (one vector per
// beat), computes the cosine self-similarity matrix, the Foote novelty curve,
// the boundaries and the phrase labels, prints them, and writes a label-free
// heat-map PNG: the SSM on top (similarity -1..1 on a dark-to-yellow ramp,
// grey grid lines every phrase, cyan lines at the detected boundaries) and
// the novelty curve below it.
//
// Usage:
//
//	structure_overview [-o file.png] [-px 6] [-phrase 16] [-seed 45]
//
// Without -o the PNG goes to structure_overview.png in the system temporary
// directory.
package main

import (
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"

	"github.com/cwbudde/algo-dsp/measure/music/structure"
)

const (
	dims        = 24   // feature dimensions per beat
	spread      = 0.6  // scatter of the beats around their section's base vector
	variantMix  = 0.45 // share of unrelated material in the A′ phrase
	panelHeight = 64   // height of the novelty panel in pixels
	panelGap    = 4    // gap between SSM and novelty panel in pixels
)

func main() {
	err := run(os.Args[1:], os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "structure_overview:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("structure_overview", flag.ContinueOnError)
	out := fs.String("o", filepath.Join(os.TempDir(), "structure_overview.png"), "output PNG path")
	px := fs.Int("px", 6, "pixels per beat")
	phrase := fs.Int("phrase", 16, "beats per phrase")
	seed := fs.Uint64("seed", 45, "random seed of the synthetic features")

	err := fs.Parse(args)
	if err != nil {
		return err
	}

	if *px < 1 || *phrase < 1 {
		return errors.New("-px and -phrase must be >= 1")
	}

	rows := song(rand.New(rand.NewPCG(*seed, 1)), *phrase)

	vectors, err := structure.Blocks([]structure.Block{{Rows: rows, Weight: 1}})
	if err != nil {
		return err
	}

	ssm, err := structure.SelfSimilarity(vectors)
	if err != nil {
		return err
	}

	novelty, err := structure.FooteNovelty(ssm, structure.DefaultHalfWidth)
	if err != nil {
		return err
	}

	boundaries, err := structure.Peaks(novelty, structure.DefaultPeakRadius, structure.DefaultPeakSigma)
	if err != nil {
		return err
	}

	phrases, err := structure.Label(ssm, *phrase, structure.DefaultSame, structure.DefaultVariant,
		structure.WithPrime("′"))
	if err != nil {
		return err
	}

	var report []string

	marks := make([]float64, len(boundaries))
	for i, b := range boundaries {
		marks[i] = float64(b.Index)
		report = append(report, fmt.Sprintf("boundary before beat %3d  novelty %.3f", b.Index, b.Novelty))
	}

	for i, p := range phrases {
		report = append(report, fmt.Sprintf("phrase %d  beats %3d-%3d  %-3s similarity to first %.3f",
			i, i**phrase, min(len(rows), (i+1)**phrase)-1, p.Label, p.Similarity))
	}

	f, err := os.Create(*out)
	if err != nil {
		return err
	}

	err = writePNG(f, ssm, novelty, *px, *phrase, marks)
	if cerr := f.Close(); err == nil {
		err = cerr
	}

	if err != nil {
		return fmt.Errorf("write %s: %w", *out, err)
	}

	report = append(report, "wrote "+*out, "")
	_, err = io.WriteString(stdout, strings.Join(report, "\n"))

	return err
}

// song returns the beat features of A B A B A′ B: every section scatters its
// beats around a random base vector, the second A repeats the first exactly
// and A′ mixes each A beat with unrelated material.
func song(rng *rand.Rand, phrase int) [][]float64 {
	section := func() [][]float64 {
		base := randomVector(rng)
		rows := make([][]float64, phrase)

		for i := range rows {
			noise := randomVector(rng)
			rows[i] = make([]float64, dims)

			for k := range rows[i] {
				rows[i][k] = base[k] + spread*noise[k]
			}
		}

		return rows
	}

	a, b := section(), section()
	variant := make([][]float64, phrase)

	for i, row := range a {
		r := randomVector(rng)
		variant[i] = make([]float64, dims)

		for k := range row {
			variant[i][k] = (1-variantMix)*row[k] + variantMix*r[k]
		}
	}

	var rows [][]float64

	for _, s := range [][][]float64{a, b, a, b, variant, b} {
		for _, r := range s {
			rows = append(rows, append([]float64(nil), r...))
		}
	}

	return rows
}

func randomVector(rng *rand.Rand) []float64 {
	v := make([]float64, dims)
	for i := range v {
		v[i] = rng.NormFloat64()
	}

	return v
}

// heat maps [0, 1] from darkness through purple, red and orange to yellow.
func heat(x float64) color.RGBA {
	stops := [][3]float64{{6, 3, 10}, {59, 15, 79}, {179, 22, 46}, {255, 123, 28}, {255, 216, 74}, {255, 243, 196}}
	x = math.Min(1, math.Max(0, x)) * float64(len(stops)-1)
	i := min(len(stops)-2, int(x))
	f := x - float64(i)

	var c [3]uint8
	for k := range c {
		c[k] = uint8(math.Round(stops[i][k] + f*(stops[i+1][k]-stops[i][k])))
	}

	return color.RGBA{c[0], c[1], c[2], 255}
}

// writePNG draws the self-similarity matrix s with px pixels per unit,
// similarity -1..1 mapped onto the heat ramp, grey grid lines every
// gridEvery units and cyan marks at fractional unit positions, and the
// novelty curve (0..1) as a bar strip below.
func writePNG(w io.Writer, s [][]float64, novelty []float64, px, gridEvery int, marks []float64) error {
	n := len(s) * px
	height := n + panelGap + panelHeight
	img := image.NewRGBA(image.Rect(0, 0, n, height))

	for i, row := range s {
		for j, v := range row {
			c := heat((v + 1) / 2)

			for y := i * px; y < (i+1)*px; y++ {
				for x := j * px; x < (j+1)*px; x++ {
					img.SetRGBA(x, y, c)
				}
			}
		}
	}

	background := heat(0)

	for y := n; y < height; y++ {
		for x := range n {
			img.SetRGBA(x, y, background)
		}
	}

	for j, v := range novelty {
		top := height - int(math.Round(v*panelHeight))
		c := heat(0.25 + 0.75*v)

		for y := top; y < height; y++ {
			for x := j * px; x < (j+1)*px-min(1, px-1); x++ {
				img.SetRGBA(x, y, c)
			}
		}
	}

	blend := func(x, y int, c color.RGBA, a float64) {
		o := img.RGBAAt(x, y)
		mix := func(p, q uint8) uint8 { return uint8(math.Round(float64(p)*(1-a) + float64(q)*a)) }
		img.SetRGBA(x, y, color.RGBA{mix(o.R, c.R), mix(o.G, c.G), mix(o.B, c.B), 255})
	}
	// line draws a vertical and a horizontal line through the SSM at pixel
	// p; the vertical one continues through the novelty panel.
	line := func(p int, c color.RGBA, a float64) {
		if p < 0 || p >= n {
			return
		}

		for k := range n {
			blend(p, k, c, a)
			blend(k, p, c, a)
		}

		for k := n + panelGap; k < height; k++ {
			blend(p, k, c, a)
		}
	}

	for u := gridEvery; u < len(s); u += gridEvery {
		line(u*px, color.RGBA{128, 128, 128, 255}, 0.35)
	}

	for _, m := range marks {
		line(int(math.Round(m*float64(px))), color.RGBA{54, 229, 255, 255}, 0.8)
	}

	return png.Encode(w, img)
}
