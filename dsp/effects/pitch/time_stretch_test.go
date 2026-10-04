package pitch

import (
	"context"
	"fmt"
	"math"
	"testing"
)

func TestStretchStreamDurationPitchAndStereo(t *testing.T) {
	for _, ratio := range []float64{0.5, 1, 1.5, 2} {
		t.Run(fmt.Sprint(ratio), func(t *testing.T) {
			n := 48000

			x := make([]float64, n)
			for i := range x {
				x[i] = 0.4 * math.Sin(2*math.Pi*440*float64(i)/48000)
			}

			s, err := NewStretchStream(48000, ratio, int64(n), 2, func(ch int, start int64, dst []float64) int {
				copy(dst, x[start:])

				if ch == 1 {
					for i := range dst {
						dst[i] *= -0.5
					}
				}

				return len(dst)
			})
			if err != nil {
				t.Fatal(err)
			}

			out := make([][]float64, 2)
			for ch := range out {
				out[ch] = make([]float64, s.OutputFrames())
			}

			next := make([]int64, 2)

			for {
				done, err := s.Step(context.Background(), func(ch int, at int64, y []float64) error {
					if at != next[ch] {
						t.Fatal("noncontiguous")
					}

					copy(out[ch][at:], y)
					next[ch] += int64(len(y))

					return nil
				})
				if err != nil {
					t.Fatal(err)
				}

				if done {
					break
				}
			}

			if next[0] != int64(math.Round(float64(n)*ratio)) {
				t.Fatal("duration")
			}

			crossings := 0

			start, end := len(out[0])/4, len(out[0])*3/4
			for i := start + 1; i < end; i++ {
				if out[0][i] >= 0 && out[0][i-1] < 0 {
					crossings++
				}

				if out[1][i] != -0.5*out[0][i] {
					t.Fatal("stereo phase")
				}

				if ratio == 1 && out[0][i] != x[i] {
					t.Fatal("identity")
				}
			}

			hz := float64(crossings) * 48000 / float64(end-start)
			if math.Abs(hz-440) > 4 {
				t.Fatalf("changed pitch %gHz", hz)
			}
		})
	}
}

func TestPublicTimeStretch(t *testing.T) {
	p, _ := NewPitchShifter(48000)
	_ = p.SetPitchRatio(2)

	out, err := p.TimeStretch(make([]float64, 100))
	if err != nil || len(out) != 200 {
		t.Fatal(err, len(out))
	}

	if _, err = p.TimeStretch([]float64{math.NaN()}); err == nil {
		t.Fatal("accepted NaN")
	}
}

func ExamplePitchShifter_TimeStretch() {
	p, _ := NewPitchShifter(48000)
	_ = p.SetPitchRatio(1.5)
	out, _ := p.TimeStretch(make([]float64, 4800))
	fmt.Println(len(out)) // Output: 7200
}
