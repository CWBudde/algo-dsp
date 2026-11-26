package dynamics

import (
	"fmt"
	"math"
	"testing"
)

func TestDAVSpectralGateTiming(t *testing.T) {
	// Expected words were measured with independent x87 extended arithmetic
	// and explicit binary32 argument and result stores.
	tests := []struct {
		name                string
		attackMS, releaseMS float64
		rate                float64
		attack, release     uint64
	}{
		{"stock", .5, 50, 44100. / 256, 0x3feffd6107300000, 0x3fed86a740000000},
		{"maximum FFT order", 1, 100, 44100. / 8192, 0x4452e6f880000000, 0x3fd1a926a0000000},
		{"fast automated attack", .025121377781033516, 2.4729864597320557, 44100. / 256, 0x417e52d030000000, 0x3fc927b620000000},
		{"multiple exponent wraps", .01, 50, 48000. / 256, 0x3fefffff34cc7a00, 0x3fedb837a0000000},
		{"nonfinite packed pole", 1, 1, 1000. / 128, 0x3ff0000000000000, 0x37f0000000000000},
		{"signed pole", 1, 1, 1000. / 255, 0x4008000000000000, 0xc000000000000000},
		{"wrapped unity pole", 1, 1, 1000. / 256, 0x4000000000000000, 0xbff0000000000000},
		{"fast automated release", .1, .025121377781033516, 44100. / 256, 0x3ff0000000000000, 0xc17e52d020000000},
		{"long times", 43, 276, 192000. / 128, 0x3f85e50800000000, 0x3feff24b00000000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			attack, release, err := DAVSpectralGateTiming(tc.attackMS, tc.releaseMS, tc.rate)
			if err != nil {
				t.Fatal(err)
			}
			if a, r := math.Float64bits(attack), math.Float64bits(release); a != tc.attack || r != tc.release {
				t.Fatalf("timing words (%016x, %016x), want (%016x, %016x)", a, r, tc.attack, tc.release)
			}
		})
	}
}

func TestDAVSpectralGateTimingInvalidDomain(t *testing.T) {
	for _, in := range [][3]float64{{0, 1, 10}, {1, -1, 10}, {1, 1, 0}, {1, math.NaN(), 10}, {1, 1, math.Inf(1)}} {
		if _, _, err := DAVSpectralGateTiming(in[0], in[1], in[2]); err == nil {
			t.Errorf("invalid timing %v accepted", in)
		}
	}
	a, r, err := DAVSpectralGateTiming(1, 1, math.SmallestNonzeroFloat64)
	if a != 1 || r != 0 || err != nil {
		t.Fatalf("small detector rate gives (%v, %v, %v), want (1, 0, nil)", a, r, err)
	}
}

func ExampleDAVSpectralGateTiming() {
	attack, release, _ := DAVSpectralGateTiming(.5, 50, 44100./256)
	fmt.Printf("attack %.3f, release %.3f\n", attack, release)
	// Output: attack 1.000, release 0.923
}
