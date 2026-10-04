package spectrum

import (
	"math"
	"testing"
)

func TestPowerWindowsAndRecovery(t *testing.T) {
	a, _ := NewPowerAverager(2, 2)
	sum, _ := NewPowerAccumulator(2)

	out := make([]float64, 2)
	if err := a.ValuesInto(out); err != nil || out[0] != 0 {
		t.Fatal(err, out)
	}

	for _, power := range [][]float64{{2, 4}, {4, 8}, {8, 16}} {
		if err := a.Add(power); err != nil {
			t.Fatal(err)
		}

		if err := sum.Add(power); err != nil {
			t.Fatal(err)
		}
	}

	_ = a.ValuesInto(out)
	if out[0] != 6 || out[1] != 12 {
		t.Fatal(out)
	}

	_ = sum.ValuesInto(out)
	if out[0] != 14.0/3 || out[1] != 28.0/3 {
		t.Fatal(out)
	}

	_ = a.Add([]float64{1e150, 1e150})
	_ = a.Add([]float64{1, 1})
	_ = a.Add([]float64{1, 1})

	_ = a.ValuesInto(out)
	if out[0] != 1 || out[1] != 1 {
		t.Fatal("stale subtractive tail", out)
	}

	a.Reset()
	sum.Reset()

	_ = a.ValuesInto(out)
	if out[0] != 0 {
		t.Fatal(out)
	}

	_ = sum.ValuesInto(out)
	if out[0] != 0 {
		t.Fatal(out)
	}

	if n := testing.AllocsPerRun(10, func() {
		_ = a.Add([]float64{2, 4})
		_ = a.ValuesInto(out)
		_ = sum.Add([]float64{2, 4})
		_ = sum.ValuesInto(out)
	}); n != 0 {
		t.Fatalf("allocations%g", n)
	}
}

func TestSpectrumCalibrationAndSmoothingParity(t *testing.T) {
	bins := []complex64{3, 4i, 0, 0, 4}

	power := make([]float64, 5)
	if err := PowerInto(power, bins); err != nil {
		t.Fatal(err)
	}

	if err := PowerToDBInto(power, power, 8, 8); err != nil {
		t.Fatal(err)
	}

	if math.Abs(power[0]-20*math.Log10(3.0/8)) > 1e-12 || math.Abs(power[1]) > 1e-12 || math.Abs(power[4]-20*math.Log10(.5)) > 1e-12 || !math.IsInf(power[2], -1) {
		t.Fatal(power)
	}

	freq := []float64{31, 40, 60, 80, 100, 150, 200, 300, 400, 600, 800}
	values := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}

	dst := make([]float64, len(freq))
	for _, fraction := range []int{1, 3, 6, 12, 24} {
		want, err := SmoothFractionalOctave(freq, values, fraction)
		if err != nil {
			t.Fatal(err)
		}

		if err := SmoothFractionalOctaveInto(dst, freq, values, fraction); err != nil {
			t.Fatal(err)
		}

		for i := range dst {
			if dst[i] != want[i] {
				t.Fatal(dst, want)
			}
		}
	}

	if n := testing.AllocsPerRun(10, func() { _ = SmoothFractionalOctaveInto(dst, freq, values, 3) }); n != 0 {
		t.Fatal(n)
	}
}

func TestSpectrumStreamingValidationAtomic(t *testing.T) {
	for _, geometry := range [][2]int{{0, 1}, {1, 0}, {32769, 1024}, {1, 1025}} {
		if _, err := NewPowerAverager(geometry[0], geometry[1]); err == nil {
			t.Fatal("invalid geometry", geometry)
		}
	}

	if _, err := NewPowerAccumulator(0); err == nil {
		t.Fatal("zero bins accepted")
	}

	a, _ := NewPowerAverager(2, 2)
	acc, _ := NewPowerAccumulator(2)

	for _, power := range [][]float64{{1}, {1, -1}, {1, math.NaN()}, {1, math.Inf(1)}, {1, 1e300}} {
		if err := a.Add(power); err == nil {
			t.Fatal(power)
		}

		if err := acc.Add(power); err == nil {
			t.Fatal(power)
		}

		if a.count != 0 || acc.count != 0 {
			t.Fatal("invalid spectrum mutated state")
		}
	}

	if err := a.ValuesInto(nil); err == nil {
		t.Fatal("short average destination")
	}

	if err := acc.ValuesInto(nil); err == nil {
		t.Fatal("short sum destination")
	}

	dst := []float64{123, 456}
	if err := PowerInto(dst, []complex128{1, complex(math.NaN(), 0)}); err == nil || dst[0] != 123 {
		t.Fatal("power validation not atomic")
	}

	if err := PowerInto(nil, []complex128{1}); err == nil {
		t.Fatal("short power destination")
	}

	if err := PowerToDBInto(dst, []float64{1, 2}, 0, 2); err == nil {
		t.Fatal("zero window")
	}

	if err := PowerToDBInto(dst, []float64{1, -1}, 1, 2); err == nil {
		t.Fatal("negative power")
	}

	for _, freq := range [][]float64{nil, {0, 1}, {1, 1}, {1, math.NaN()}} {
		if err := SmoothFractionalOctaveInto(dst, freq, []float64{1, 2}, 3); err == nil {
			t.Fatal("invalid frequencies", freq)
		}
	}

	acc.count = 1 << 52
	if err := acc.Add([]float64{1, 1}); err == nil {
		t.Fatal("count overflow")
	}
}

func BenchmarkPowerAverager(b *testing.B) {
	a, _ := NewPowerAverager(4097, 16)
	src, dst := make([]float64, 4097), make([]float64, 4097)
	src[1] = 1

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_ = a.Add(src)
		_ = a.ValuesInto(dst)
	}
}
