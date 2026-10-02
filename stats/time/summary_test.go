package time

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestSummaryMatchesCalculate(t *testing.T) {
	negativeZero := math.Copysign(0, -1)
	quietNaN := math.Float64frombits(0x7ff8123456789abc)
	signalingNaN := math.Float64frombits(0x7ff0123456789abc)

	tests := []struct {
		name   string
		signal []float64
	}{
		{"empty", nil},
		{"empty_non_nil", []float64{}},
		{"single", []float64{-3.5}},
		{"finite", []float64{0.5, -2, 3, -2, 0, 3}},
		{"negative_zero_first", []float64{negativeZero, 0, negativeZero}},
		{"positive_zero_first", []float64{0, negativeZero, 0}},
		{"nan_first", []float64{quietNaN, -2, 3}},
		{"nan_later", []float64{999, quietNaN, -2, 0.5}},
		{"signaling_nan_first", []float64{signalingNaN, -2, 3}},
		{"signaling_nan_later", []float64{3, signalingNaN, -2}},
		{"negative_nan_first", []float64{math.Float64frombits(0xfff8123456789abc), -2, 3}},
		{"positive_infinity", []float64{0, math.Inf(1), -2}},
		{"negative_infinity", []float64{0, math.Inf(-1), 2}},
		{"both_infinities", []float64{math.Inf(-1), 2, math.Inf(1)}},
		{"infinity_and_nan", []float64{math.Inf(1), quietNaN}},
		{"extreme_finite", []float64{math.MaxFloat32, -math.MaxFloat32, math.SmallestNonzeroFloat32}},
		{"float64_extremes", []float64{math.MaxFloat64, -math.MaxFloat64, math.SmallestNonzeroFloat64}},
		{"random", summaryRandomSignal(4096)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runSummaryParity(t, test.signal)
		})
	}
}

func runSummaryParity(t *testing.T, signal []float64) {
	t.Helper()
	checkSummary(t, Summary(signal), Calculate(signal))
	float32Signal := make([]float32, len(signal))

	converted := make([]float64, len(signal))
	for i, value := range signal {
		float32Signal[i] = float32(value)
		converted[i] = float64(float32Signal[i])
	}

	checkSummary(t, Summary(float32Signal), Calculate(converted))
}

func checkSummary(t *testing.T, got SummaryStats, want Stats) {
	t.Helper()

	for _, field := range []struct {
		name string
		got  float64
		want float64
	}{
		{"Min", got.Min, want.Min}, {"Max", got.Max, want.Max}, {"Energy", got.Energy, want.Energy},
	} {
		// Energy NaN payloads can vary with arithmetic contraction. Extrema
		// must preserve their bits, including NaN payloads and signed zero.
		if field.name == "Energy" && math.IsNaN(field.got) && math.IsNaN(field.want) {
			continue
		}

		if math.Float64bits(field.got) != math.Float64bits(field.want) {
			t.Errorf("%s bits = %016x, want %016x", field.name, math.Float64bits(field.got), math.Float64bits(field.want))
		}
	}
}

func summaryRandomSignal(length int) []float64 {
	rng := rand.New(rand.NewPCG(42, 7))

	signal := make([]float64, length)
	for i := range signal {
		signal[i] = math.Ldexp(2*rng.Float64()-1, rng.IntN(48)-24)
	}

	return signal
}

func TestSummaryEnergyFloat64Precision(t *testing.T) {
	signal := []float32{math.MaxFloat32, math.MaxFloat32}

	want := 2 * float64(math.MaxFloat32) * float64(math.MaxFloat32)
	if got := Summary(signal).Energy; got != want || math.IsInf(got, 0) {
		t.Fatalf("Energy = %g, want finite float64 result %g", got, want)
	}
}

func TestSummaryFloat32NaNPayloads(t *testing.T) {
	for _, payload := range []uint32{0x7fc12345, 0xffc54321, 0x7f812345} {
		for _, first := range []bool{false, true} {
			signal := []float32{999, -2, math.Float32frombits(payload), 0.5}
			if first {
				signal[0], signal[2] = signal[2], signal[0]
			}

			converted := make([]float64, len(signal))
			for i, value := range signal {
				converted[i] = float64(value)
			}

			checkSummary(t, Summary(signal), Calculate(converted))
		}
	}
}

func TestSummaryNamedSampleType(t *testing.T) {
	type sample float32

	got := Summary([]sample{1, -2, 3})
	if got != (SummaryStats{Min: -2, Max: 3, Energy: 14}) {
		t.Fatalf("Summary of named sample type = %+v", got)
	}
}

func TestSummaryZeroAllocations(t *testing.T) {
	signal64 := summaryRandomSignal(4096)

	signal32 := make([]float32, len(signal64))
	for i, value := range signal64 {
		signal32[i] = float32(value)
	}

	tests := []struct {
		name string
		scan func()
	}{
		{"float32", func() { Summary(signal32) }},
		{"float64", func() { Summary(signal64) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if allocs := testing.AllocsPerRun(100, test.scan); allocs != 0 {
				t.Fatalf("Summary allocates %.0f times, want zero", allocs)
			}
		})
	}
}
