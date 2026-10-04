package stereo

import (
	"fmt"
	"math"
	"reflect"
	"testing"
)

func TestAnalyzerCorrelationPointsAndSlidingWindow(t *testing.T) {
	for _, sign := range []float32{1, -1} {
		a, _ := NewAnalyzer(4, 4)

		input := []float32{1, sign, .5, .5 * sign, -1, -sign, -.5, -.5 * sign}
		if err := a.ProcessInterleaved32(input, 2); err != nil {
			t.Fatal(err)
		}

		if math.Abs(a.Correlation()-float64(sign)) > 1e-15 {
			t.Fatal(a.Correlation())
		}

		points := make([]Point, 4)
		if n := a.PointsInto(points); n != 4 {
			t.Fatal(n)
		}

		for i, p := range points {
			l, r := float64(input[2*i]), float64(input[2*i+1])
			if p != (Point{(l + r) * math.Sqrt(.5), (l - r) * math.Sqrt(.5)}) {
				t.Fatal(p)
			}
		}

		short := make([]Point, 2)
		a.PointsInto(short)

		if !reflect.DeepEqual(short, points[2:]) {
			t.Fatal("short chronological subset")
		}

		if err := a.ProcessInterleaved32(make([]float32, 8), 2); err != nil || a.Correlation() != 0 {
			t.Fatal("sliding silence", a.Correlation(), err)
		}

		a.Reset()

		if a.Correlation() != 0 || a.PointsInto(points) != 0 {
			t.Fatal("reset")
		}

		_ = a.ProcessInterleaved32([]float32{.5, -.5}, 1)
		if math.Abs(a.Correlation()-1) > 1e-15 {
			t.Fatal("mono")
		}
	}
}

func TestAnalyzerPartitionsAtomicErrorsAndZeroAllocations(t *testing.T) {
	input := make([]float32, 400)
	for i := range 100 {
		input[i*4] = float32(math.Sin(float64(i) * .2))
		input[i*4+1] = float32(math.Cos(float64(i) * .13))
	}

	one, _ := NewAnalyzer(31, 7)
	two, _ := NewAnalyzer(31, 7)

	_ = one.ProcessInterleaved32(input, 4)
	for i := 0; i < len(input); i += 12 {
		_ = two.ProcessInterleaved32(input[i:min(i+12, len(input))], 4)
	}

	if !reflect.DeepEqual(one, two) {
		t.Fatal("partition")
	}

	before := *two
	left := append([]float64(nil), two.left...)

	points := append([]Point(nil), two.points...)
	for _, bad := range []struct {
		x        []float32
		channels int
	}{{nil, 0}, {[]float32{1}, 2}, {[]float32{.5, .5, float32(math.NaN()), 0}, 4}, {[]float32{float32(math.Inf(1))}, 1}} {
		if err := two.ProcessInterleaved32(bad.x, bad.channels); err == nil || !reflect.DeepEqual(before, *two) || !reflect.DeepEqual(left, two.left) || !reflect.DeepEqual(points, two.points) {
			t.Fatal("atomic validation", bad)
		}
	}

	for _, bad := range [][2]int{{0, 1}, {1, 0}, {1<<20 + 1, 1}, {1, 4097}} {
		if _, err := NewAnalyzer(bad[0], bad[1]); err == nil {
			t.Fatal(bad)
		}
	}

	var zero Analyzer
	if err := zero.ProcessInterleaved32(nil, 1); err == nil {
		t.Fatal("zero")
	}

	var nilAnalyzer *Analyzer
	if err := nilAnalyzer.ProcessInterleaved32(nil, 1); err == nil || nilAnalyzer.Correlation() != 0 || nilAnalyzer.PointsInto(nil) != 0 {
		t.Fatal("nil")
	}

	nilAnalyzer.Reset()

	if allocs := testing.AllocsPerRun(5, func() {
		two.Reset()
		_ = two.ProcessInterleaved32(input, 4)
		_ = two.Correlation()
		_ = two.PointsInto(points)
	}); allocs != 0 {
		t.Fatal(allocs)
	}
}

func ExampleAnalyzer() {
	a, _ := NewAnalyzer(4, 4)
	_ = a.ProcessInterleaved32([]float32{1, -1, .5, -.5}, 2)
	fmt.Printf("correlation %.0f\n", a.Correlation())
	// Output: correlation -1
}

func BenchmarkAnalyzerInterleaved(b *testing.B) {
	a, _ := NewAnalyzer(4800, 64)
	input := make([]float32, 1024)

	b.ReportAllocs()

	for b.Loop() {
		_ = a.ProcessInterleaved32(input, 2)
	}
}

func TestAnalyzerRecoversQuietCorrelationAfterHugeSignal(t *testing.T) {
	a, _ := NewAnalyzer(3, 3)
	for _, input := range [][]float32{{1e30, 1e30, 2e30, 2e30, 3e30, 3e30}, {1e-30, -1e-30, 2e-30, -2e-30, 3e-30, -3e-30}, {0, 0, 0, 0, 0, 0}} {
		if err := a.ProcessInterleaved32(input, 2); err != nil {
			t.Fatal(err)
		}

		want := 1.0
		if input[0] < 1 {
			want = -1
		}

		if input[0] == 0 {
			want = 0
		}

		if math.Abs(a.Correlation()-want) > 1e-15 {
			t.Fatalf("got%g want%g", a.Correlation(), want)
		}
	}
}
