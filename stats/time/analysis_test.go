package time

import (
	"fmt"
	"math"
	"reflect"
	"testing"
)

func TestAnalysisPartitionsReferenceAndAtomicErrors(t *testing.T) {
	input := []float64{.5, -.5, 0, 1, -2, 0, .25}
	for _, partition := range []int{1, 2, 7} {
		a, err := NewAccumulator(int64(len(input)))
		if err != nil {
			t.Fatal(err)
		}

		for offset := 0; offset < len(input); offset += partition {
			if err := a.Add(input[offset:min(offset+partition, len(input))]); err != nil {
				t.Fatal(err)
			}
		}

		result := a.Result()
		if result.Samples != 7 || result.ZeroCrossings != 2 || result.ClippedSamples != 2 || result.Peak != 2 || math.Abs(result.DC+.75/7) > 1e-15 || math.Abs(result.RMS-math.Sqrt(5.5625/7)) > 1e-15 || math.Abs(result.CrestFactor-2/result.RMS) > 1e-15 {
			t.Fatal(result)
		}

		for _, bad := range [][]float64{{math.NaN()}, {math.Inf(1)}, {1}} {
			if err := a.Add(bad); err == nil || a.Result() != result {
				t.Fatal("failed Add changed result")
			}
		}

		a.Reset()

		if a.Result() != (Analysis{}) {
			t.Fatal("reset")
		}

		if err := a.Add32([]float32{1, -1}); err != nil || a.Result().RMS != 1 || a.Result().ZeroCrossings != 1 {
			t.Fatal("Add32", err, a.Result())
		}
	}

	if _, err := NewAccumulator(0); err == nil {
		t.Fatal("limit")
	}

	var zero Accumulator
	if err := zero.Add(nil); err == nil {
		t.Fatal("zero state")
	}

	var nilAccumulator *Accumulator
	if err := nilAccumulator.Add32(nil); err == nil || nilAccumulator.Result() != (Analysis{}) {
		t.Fatal("nil state")
	}

	nilAccumulator.Reset()
}

func TestAnalysisFiniteExtremesSilenceNonfiniteAtomicAndAllocations(t *testing.T) {
	a, _ := NewAccumulator(10000)
	for _, input := range [][]float64{{math.MaxFloat64, -math.MaxFloat64}, {0, 0, 0}, {math.SmallestNonzeroFloat64, -math.SmallestNonzeroFloat64}} {
		a.Reset()

		if err := a.Add(input); err != nil {
			t.Fatal(err)
		}

		r := a.Result()
		if math.IsNaN(r.RMS) || math.IsInf(r.RMS, 0) || math.IsNaN(r.DC) || math.IsInf(r.DC, 0) {
			t.Fatal(r)
		}
	}

	a.Reset()
	_ = a.Add([]float64{.5})

	before := a.Result()
	if err := a.Add([]float64{1, math.NaN()}); err == nil || a.Result() != before {
		t.Fatal("finite preflight atomicity")
	}

	if err := a.Add32([]float32{1, float32(math.Inf(1))}); err == nil || a.Result() != before {
		t.Fatal("finite32 preflight atomicity")
	}

	if allocs := testing.AllocsPerRun(5, func() { a.Reset(); _ = a.Add([]float64{.5, -.5}); _ = a.Result() }); allocs != 0 {
		t.Fatal(allocs)
	}
}

func TestClipDetectorBoundaryRunsValidationAndReset(t *testing.T) {
	type run struct{ start, end int64 }

	d, _ := NewClipDetector(1)
	runs := []run{}

	emit := func(start, end int64) { runs = append(runs, run{start, end}) }
	if err := d.Process32([]float32{0, 1, -2}, 10, emit); err != nil {
		t.Fatal(err)
	}

	if err := d.Process([]float64{1, 0, -1}, 13, emit); err != nil {
		t.Fatal(err)
	}

	for _, bad := range []struct {
		x      []float64
		offset int64
	}{{[]float64{0, math.NaN()}, 16}, {nil, 17}, {nil, -1}, {[]float64{1}, math.MaxInt64}} {
		before := *d

		n := len(runs)
		if err := d.Process(bad.x, bad.offset, emit); err == nil || *d != before || len(runs) != n {
			t.Fatal("atomic failure", bad)
		}
	}

	d.Flush(emit)
	d.Flush(emit)

	if !reflect.DeepEqual(runs, []run{{11, 14}, {15, 16}}) {
		t.Fatal(runs)
	}

	d.Reset()

	runs = nil

	if err := d.Process([]float64{-1, 0}, 0, emit); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(runs, []run{{0, 1}}) {
		t.Fatal(runs)
	}

	for _, threshold := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, err := NewClipDetector(threshold); err == nil {
			t.Fatal(threshold)
		}
	}

	if err := d.Process(nil, 2, nil); err == nil {
		t.Fatal("nil callback")
	}

	var nilDetector *ClipDetector
	if err := nilDetector.Process(nil, 0, emit); err == nil {
		t.Fatal("nil detector")
	}

	nilDetector.Reset()
	nilDetector.Flush(emit)

	if allocs := testing.AllocsPerRun(5, func() {
		d.Reset()
		_ = d.Process32([]float32{0, .5}, 0, func(int64, int64) {})
		d.Flush(func(int64, int64) {})
	}); allocs != 0 {
		t.Fatal(allocs)
	}
}

func ExampleAccumulator() {
	a, _ := NewAccumulator(4)
	_ = a.Add32([]float32{.5, -.5, .5, -.5})
	r := a.Result()
	// Adding +0 turns a -0 DC (arm64 fuses the mean's multiply-add) into +0.
	fmt.Printf("peak %.1f RMS %.1f DC %.1f crossings %d\n", r.Peak, r.RMS, math.Round(r.DC*10)/10+0, r.ZeroCrossings)
	// Output: peak 0.5 RMS 0.5 DC 0.0 crossings 3
}

func ExampleClipDetector() {
	d, _ := NewClipDetector(1)
	emit := func(start, end int64) { fmt.Println(start, end) }
	_ = d.Process32([]float32{0, 1, -1}, 0, emit)
	_ = d.Process32([]float32{1, 0}, 3, emit)
	d.Flush(emit)
	// Output: 1 4
}

func BenchmarkAccumulatorAdd32(b *testing.B) {
	a, _ := NewAccumulator(1024)
	input := make([]float32, 1024)

	b.ReportAllocs()

	for b.Loop() {
		a.Reset()
		_ = a.Add32(input)
	}
}
