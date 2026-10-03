package reverb

import (
	"math"
	"testing"
)

func TestConvolutionReverbPartitionLimit(t *testing.T) {
	kernel := make([]float64, 3001)
	kernel[0], kernel[1024], kernel[3000] = 1, -.5, .25

	for _, order := range []int{10, 11, 13} {
		r, err := NewConvolutionReverbWithMaxBlockOrder(kernel, 7, order)
		if err != nil {
			t.Fatal(err)
		}

		if r.Latency() != 128 {
			t.Fatalf("order%d latency%d", order, r.Latency())
		}

		r.SetWetDry(1, 0)

		output := make([]float64, len(kernel)+r.Latency()+128)

		output[0] = 1
		if err := r.ProcessInPlace(output); err != nil {
			t.Fatal(err)
		}

		for i, got := range output {
			want := 0.0
			if at := i - r.Latency(); at >= 0 && at < len(kernel) {
				want = kernel[at]
			}

			if math.Abs(got-want) > 1e-9 {
				t.Fatalf("order%d sample%d got%g want%g", order, i, got, want)
			}
		}
	}

	if _, err := NewConvolutionReverbWithMaxBlockOrder(kernel, 7, 6); err == nil {
		t.Fatal("partition cap below latency order accepted")
	}

	if _, err := NewConvolutionReverbWithMaxBlockOrder(nil, 7, 10); err == nil {
		t.Fatal("empty IR accepted")
	}
}
