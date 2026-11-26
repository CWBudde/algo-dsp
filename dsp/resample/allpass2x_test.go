package resample

import (
	"fmt"
	"math"
	"testing"
)

// These vectors were computed independently from the two allpass transfer
// paths, with the coefficients and each section output rounded to float32.
func TestAllpassUpsampler2x32Impulse(t *testing.T) {
	tests := []struct {
		name       string
		count      int
		transition float64
		want       [][2]uint32
	}{
		{"four coefficients", 4, 0.249, [][2]uint32{
			{0x3c895be1, 0x3e02bb00},
			{0x3edbb781, 0x3f4cd394},
			{0x3f4ffae4, 0x3e7fd7ef},
			{0xbeb93b1f, 0xbea9a0ef},
			{0x3e13bd69, 0x3e8b407d},
			{0xbd69143b, 0xbe5421b4},
		}},
		{"odd coefficient count", 3, 0.1, [][2]uint32{
			{0x3dd00284, 0x3edbf02c},
			{0x3f536632, 0x3f50c2cd},
			{0x3e6c38b8, 0xbeb35a80},
		}},
		{"empty odd path", 1, 0.1, [][2]uint32{
			{0x3f0b9a55, 0x3f800000},
			{0x3f33df0b, 0},
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			up, err := NewAllpassUpsampler2x32(tc.count, tc.transition)
			if err != nil {
				t.Fatal(err)
			}

			for range 2 {
				input := float32(1)
				for i, want := range tc.want {
					first, second := up.ProcessSample(input)

					got := [2]uint32{math.Float32bits(first), math.Float32bits(second)}
					if got != want {
						t.Fatalf("sample %d: bits %08x, want %08x", i, got, want)
					}

					input = 0
				}

				up.Reset()
			}
		})
	}
}

func TestAllpassDownsampler2x32AsymmetricInput(t *testing.T) {
	down, err := NewAllpassDownsampler2x32(4, 0.01)
	if err != nil {
		t.Fatal(err)
	}

	// Distinct samples within each pair distinguish the even and odd paths.
	input := [...][2]float32{{1, 0}, {-0.25, 0.5}, {0, -0.75}, {0.125, 0}, {}, {}, {}, {}}
	want := [...]uint32{0x3e9b59f9, 0x3e9435d8, 0xbdf9841e, 0xbe112bee, 0xbda6577a, 0x3e010b29, 0xbde4f12a, 0x3db74e3c}

	for range 2 {
		for i, pair := range input {
			got := math.Float32bits(down.ProcessSample(pair[0], pair[1]))
			if got != want[i] {
				t.Fatalf("pair %d: bits %08x, want %08x", i, got, want[i])
			}
		}

		down.Reset()
	}
}

func TestAllpassDownsampler2x32RateResponse(t *testing.T) {
	for _, rate := range []int{44100, 48000, 96000} {
		t.Run(fmt.Sprint(rate), func(t *testing.T) {
			passband := allpassDownsampleToneGain(t, rate, 1000)
			if math.Abs(passband-1) > 0.0001 {
				t.Errorf("1 kHz gain = %.8f, want unity", passband)
			}

			stopband := allpassDownsampleToneGain(t, rate, 0.75*float64(rate))
			if math.Abs(stopband-0.01104152) > 0.00001 {
				t.Errorf("alias gain = %.8f, want approximately 0.01104152", stopband)
			}
		})
	}
}

func allpassDownsampleToneGain(t *testing.T, outputRate int, hz float64) float64 {
	t.Helper()

	down, err := NewAllpassDownsampler2x32(4, 0.01)
	if err != nil {
		t.Fatal(err)
	}

	const warmup = 256

	omega := 2 * math.Pi * hz / float64(2*outputRate)
	count := 4 * outputRate / 100

	var power float64

	for i := range warmup + count {
		first := float32(math.Cos(float64(2*i) * omega))
		second := float32(math.Cos(float64(2*i+1) * omega))

		y := float64(down.ProcessSample(first, second))
		if i >= warmup {
			power += y * y
		}
	}

	return math.Sqrt(2 * power / float64(count))
}

func TestAllpass2x32DesignValidation(t *testing.T) {
	tests := []struct {
		name       string
		count      int
		transition float64
	}{
		{"no coefficients", 0, 0.1},
		{"negative width", 4, -0.1},
		{"zero width", 4, 0},
		{"width upper bound", 4, 0.5},
		{"non-finite width", 4, math.NaN()},
		{"unstable rounded coefficients", 16, math.SmallestNonzeroFloat64},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewAllpassUpsampler2x32(tc.count, tc.transition); err == nil {
				t.Error("upsampler accepted invalid design")
			}

			if _, err := NewAllpassDownsampler2x32(tc.count, tc.transition); err == nil {
				t.Error("downsampler accepted invalid design")
			}
		})
	}
}

func TestAllpass2x32ProcessSampleAllocations(t *testing.T) {
	up, err := NewAllpassUpsampler2x32(4, 0.249)
	if err != nil {
		t.Fatal(err)
	}

	down, err := NewAllpassDownsampler2x32(4, 0.01)
	if err != nil {
		t.Fatal(err)
	}

	allocs := testing.AllocsPerRun(100, func() {
		first, second := up.ProcessSample(0.5)
		down.ProcessSample(first, second)
	})
	if allocs != 0 {
		t.Fatalf("upsampling and downsampling allocate %g objects per sample", allocs)
	}
}

func BenchmarkAllpass2x32(b *testing.B) {
	up, err := NewAllpassUpsampler2x32(4, 0.249)
	if err != nil {
		b.Fatal(err)
	}

	down, err := NewAllpassDownsampler2x32(4, 0.01)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()

	for b.Loop() {
		first, second := up.ProcessSample(0.5)
		down.ProcessSample(first, second)
	}
}

func ExampleNewAllpassUpsampler2x32() {
	up, err := NewAllpassUpsampler2x32(4, 0.249)
	if err != nil {
		return
	}

	first, second := up.ProcessSample(1)
	fmt.Printf("%.4f %.4f\n", first, second)
	// Output: 0.0168 0.1277
}

func ExampleNewAllpassDownsampler2x32() {
	down, err := NewAllpassDownsampler2x32(4, 0.01)
	if err != nil {
		return
	}

	fmt.Printf("%.4f\n", down.ProcessSample(1, 0))
	// Output: 0.3034
}
