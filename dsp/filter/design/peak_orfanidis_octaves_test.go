package design

import (
	"errors"
	"fmt"
	"math"
	"math/cmplx"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

func TestPeakOrfanidisOctavesReference(t *testing.T) {
	// Independent 62-digit Decimal evaluation of the octave-bandwidth
	// equations, using trigonometric power series and gain inputs rounded
	// to the Single precision returned by the legacy plug-in converter.
	tests := []struct {
		name      string
		frequency float64
		gain      float64
		bandwidth float64
		rate      float64
		want      [5]float64
	}{
		{
			"boost", 1000, 3.981071710586548, 1, 88200,
			[5]float64{1.0376298536425422, -1.9710087896861738, 0.93838824398780835, -1.9700623252777113, 0.97507163322188797},
		},
		{
			"high frequency cut", 20000, 0.3162277638912201, 0.4000000059604645, 88200,
			[5]float64{0.81165704561472566, -0.21564306171541152, 0.65070184854858481, -0.24500135017630837, 0.49171718262420731},
		},
		{
			"legacy zero control", 50, 1.0011519193649292, 1, 88200,
			[5]float64{1.0000014487167501, -1.9974733041462012, 0.99748452657032530, -1.9974733026681544, 0.99748597380902855},
		},
		{
			"above Nyquist at a low rate", 18000, 1.9952622652053833, 1, 16000,
			[5]float64{1.8834444226040824, -0.19771840723384543, -1.6158706727999949, -0.17632506104791632, -0.75381959638184162},
		},
		{
			"high sample rate", 5400, 1.364583134651184, 2.25, 192000,
			[5]float64{1.0443598841506149, -1.7486308268988548, 0.73175593470746556, -1.7434306271148576, 0.77091561907408322},
		},
		{
			"wide cut", 20000, 0.25118863582611084, 18, 88200,
			[5]float64{0.25118877570583636, -0.000072581583362599125, -0.25111545127235558, -0.00029005340282191112, -0.99970920374705991},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := PeakOrfanidisOctaves(tc.frequency, tc.gain, tc.bandwidth, tc.rate)
			if err != nil {
				t.Fatal(err)
			}

			for i, got := range [5]float64{c.B0, c.B1, c.B2, c.A1, c.A2} {
				if d := math.Abs(got - tc.want[i]); !(d <= 1e-12) {
					t.Errorf("coefficient %d = %.17g, want %.17g", i, got, tc.want[i])
				}
			}

			assertStableSection(t, c)

			if got := cmplx.Abs(c.Response(0, tc.rate)); math.Abs(got-1) > 1e-9 {
				t.Errorf("DC gain = %.12g, want unity", got)
			}
		})
	}
}

func TestPeakOrfanidisOctavesUnity(t *testing.T) {
	c, err := PeakOrfanidisOctaves(1000, 1, 1, 88200)
	if err != nil || c != biquad.Identity() {
		t.Fatalf("unity gain returned %+v, %v", c, err)
	}
}

func TestPeakOrfanidisOctavesInvalid(t *testing.T) {
	tests := []struct {
		name      string
		frequency float64
		gain      float64
		bandwidth float64
		rate      float64
	}{
		{"zero rate", 1000, 2, 1, 0},
		{"nonfinite rate", 1000, 2, 1, math.Inf(1)},
		{"zero frequency", 0, 2, 1, 88200},
		{"nonfinite frequency", math.NaN(), 2, 1, 88200},
		{"zero gain", 1000, 0, 1, 88200},
		{"nonfinite gain", 1000, math.Inf(1), 1, 88200},
		{"zero bandwidth", 1000, 2, 0, 88200},
		{"nonfinite bandwidth", 1000, 2, math.NaN(), 88200},
		{"singular center", 44100, 2, 1, 88200},
		{"folded singular center", 5.5 * 48000, 2, 1e-15, 48000},
		{"folded singular center at Nyquist", 5.5 * 44100, 0.5, 1e-18, 44100},
		{"unreachable peak gain", 1707.32174325624, 75.67419969498, 7.63243707039, 48000},
		{"overflowing bandwidth warping", 1000, 2, math.MaxFloat64, 88200},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := PeakOrfanidisOctaves(tc.frequency, tc.gain, tc.bandwidth, tc.rate)
			if !errors.Is(err, ErrInvalidPeakParams) || c != biquad.Identity() {
				t.Fatalf("invalid design returned %+v, %v", c, err)
			}
		})
	}
}

func BenchmarkPeakOrfanidisOctaves(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		if _, err := PeakOrfanidisOctaves(1000, 2, 1, 88200); err != nil {
			b.Fatal(err)
		}
	}
}

func ExamplePeakOrfanidisOctaves() {
	c, err := PeakOrfanidisOctaves(1000, 2, 1, 88200)
	if err != nil {
		return
	}

	fmt.Printf("%.3f\n", cmplx.Abs(c.Response(1000, 88200)))
	// Output: 2.000
}
