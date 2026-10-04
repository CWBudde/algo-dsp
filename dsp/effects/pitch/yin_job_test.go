package pitch

import (
	"math"
	"testing"
)

func TestYINJobExactDetectorParity(t *testing.T) {
	for _, rate := range []float64{8000, 48000, 384000} {
		d, err := NewYINDetector(rate)
		if err != nil {
			t.Fatal(err)
		}

		j, err := NewYINJob(d)
		if err != nil {
			t.Fatal(err)
		}

		frame := make([]float64, d.FrameSize())
		for _, kind := range []string{"tone", "silence", "mixed", "noise"} {
			for i := range frame {
				switch kind {
				case "tone":
					frame[i] = .4 * math.Sin(2*math.Pi*440*float64(i)/rate)
				case "silence":
					frame[i] = 0
				case "mixed":
					frame[i] = .2*math.Sin(2*math.Pi*220*float64(i)/rate) + .1*math.Sin(2*math.Pi*440*float64(i)/rate)
				case "noise":
					frame[i] = float64((i*7919)%65521)/65521 - .5
				}
			}

			want, err := d.Detect(frame)
			if err != nil {
				t.Fatal(err)
			}

			budgets := []int{1, 7, 4096, 65536}
			if rate == 384000 {
				budgets = []int{4096, 65536}
			}

			for _, budget := range budgets {
				if err := j.Begin(frame); err != nil {
					t.Fatal(err)
				}

				for {
					done, err := j.Step(budget)
					if err != nil {
						t.Fatal(err)
					}

					if done {
						break
					}
				}

				got, err := j.Result()
				if err != nil {
					t.Fatal(err)
				}

				if got != want {
					t.Fatalf("rate%g kind%s budget%d got%+v want%+v", rate, kind, budget, got, want)
				}
			}
		}
	}
}

func TestYINJobOwnershipValidationResetAndAllocation(t *testing.T) {
	d, _ := NewYINDetector(8000)
	j, _ := NewYINJob(d)

	frame := make([]float64, d.FrameSize())
	for i := range frame {
		frame[i] = .2 * math.Sin(2*math.Pi*200*float64(i)/8000)
	}

	if err := j.Begin(frame); err != nil {
		t.Fatal(err)
	}

	if done, err := j.Step(1); err != nil || done {
		t.Fatal("one scalar operation completed whole frame")
	}

	bad := append([]float64(nil), frame...)

	bad[len(bad)-1] = math.NaN()
	if err := j.Begin(bad); err == nil {
		t.Fatal("unsafe frame accepted")
	}

	if j.index != 1 {
		t.Fatal("invalid begin changed pending analysis")
	}

	if _, err := j.Result(); err == nil {
		t.Fatal("pending result accepted")
	}

	if _, err := j.Step(0); err == nil {
		t.Fatal("zero budget accepted")
	}

	if err := j.Begin(frame[:10]); err == nil {
		t.Fatal("short frame accepted")
	}

	clear(frame)

	for {
		done, err := j.Step(7)
		if err != nil {
			t.Fatal(err)
		}

		if done {
			break
		}
	}

	got, _ := j.Result()
	if !got.Voiced || math.Abs(got.FrequencyHz-200) > 2 {
		t.Fatal(got)
	}

	if allocations := testing.AllocsPerRun(3, func() {
		j.Reset()

		if err := j.Begin(frame); err != nil {
			panic(err)
		}

		for {
			done, err := j.Step(4096)
			if err != nil {
				panic(err)
			}

			if done {
				break
			}
		}
	}); allocations != 0 {
		t.Fatalf("allocations%g", allocations)
	}

	if _, err := NewYINJob(nil); err == nil {
		t.Fatal("nil detector accepted")
	}

	oversized, _ := NewYINDetector(384000, WithYINFrequencyRange(10, 1600))
	if _, err := NewYINJob(oversized); err == nil {
		t.Fatal("unbounded detector accepted")
	}
}

func BenchmarkYINJobStep(b *testing.B) {
	d, _ := NewYINDetector(48000)
	j, _ := NewYINJob(d)

	frame := make([]float64, d.FrameSize())
	for i := range frame {
		frame[i] = .3 * math.Sin(2*math.Pi*440*float64(i)/48000)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if err := j.Begin(frame); err != nil {
			b.Fatal(err)
		}

		for {
			done, err := j.Step(4096)
			if err != nil {
				b.Fatal(err)
			}

			if done {
				break
			}
		}
	}
}

func TestYINJobConfiguredDetectorParityAndOwnership(t *testing.T) {
	for _, interpolate := range []bool{false, true} {
		for _, threshold := range []float64{.01, .3} {
			d, err := NewYINDetector(8000, WithYINThreshold(threshold), WithYINParabolicInterpolation(interpolate), WithYINFrequencyRange(80, 900), WithYINSilenceThresholdDB(-40))
			if err != nil {
				t.Fatal(err)
			}

			job, err := NewYINJob(d)
			if err != nil {
				t.Fatal(err)
			}

			for _, amplitude := range []float64{0, .001, .4} {
				frame := make([]float64, d.FrameSize())
				for i := range frame {
					frame[i] = amplitude * (math.Sin(2*math.Pi*237*float64(i)/8000) + .7*math.Sin(2*math.Pi*474*float64(i)/8000))
				}

				want, err := d.Detect(frame)
				if err != nil {
					t.Fatal(err)
				}

				if err := job.Begin(frame); err != nil {
					t.Fatal(err)
				}

				for {
					done, err := job.Step(13)
					if err != nil {
						t.Fatal(err)
					}

					if done {
						break
					}
				}

				got, err := job.Result()
				if err != nil || got != want {
					t.Fatalf("interp%v threshold%g got%+v want%+v err%v", interpolate, threshold, got, want, err)
				}
			}
		}
	}
}
