package streamframe

import (
	"errors"
	"math"
	"testing"
)

func TestSchedulingNormalizationPartitionsAndReset(t *testing.T) {
	window := []float64{.5, 1, 1, .5}
	transform := func(input, output []float64) error {
		for i, x := range input {
			output[i] = x * window[i] * window[i]
		}

		return nil
	}

	a, err := New(window, 2, transform)
	if err != nil {
		t.Fatal(err)
	}

	b, _ := New(window, 2, transform)

	input := make([]float64, 1000)
	for i := range input {
		input[i] = math.Sin(float64(i) * .13)
	}

	x, y := append([]float64(nil), input...), append([]float64(nil), input...)
	if err := a.ProcessInPlace(x, 1); err != nil {
		t.Fatal(err)
	}

	for pos := range y {
		if err := b.ProcessInPlace(y[pos:pos+1], 1); err != nil {
			t.Fatal(err)
		}
	}

	for i, v := range x {
		if v != y[i] {
			t.Fatalf("partition%d", i)
		}

		if i >= a.Latency() && math.Abs(v-input[i-a.Latency()]) > 1e-14 {
			t.Fatalf("normalization%d", i)
		}
	}

	a.Reset()

	z := append([]float64(nil), input...)

	_ = a.ProcessInPlace(z, 1)
	for i, v := range x {
		if v != z[i] {
			t.Fatal("reset mismatch")
		}
	}

	buf := make([]float64, 32)

	if n := testing.AllocsPerRun(5, func() { a.Reset(); _ = a.ProcessInPlace(buf, .5) }); n != 0 {
		t.Fatalf("allocations%g", n)
	}
}

func TestInvalidConfigurationAndTransformFailure(t *testing.T) {
	for _, item := range []struct {
		window    []float64
		hop       int
		transform Transform
	}{{nil, 1, nil}, {[]float64{1, 1}, 0, func([]float64, []float64) error { return nil }}, {[]float64{1, 1}, 2, func([]float64, []float64) error { return nil }}, {[]float64{1, 1}, 1, nil}} {
		if _, err := New(item.window, item.hop, item.transform); err == nil {
			t.Error("invalid configuration accepted")
		}
	}

	sentinel := errors.New("FFT failed")

	engine, _ := New([]float64{1, 1}, 1, func([]float64, []float64) error { return sentinel })
	if err := engine.ProcessInPlace([]float64{1, 2}, 1); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
}
