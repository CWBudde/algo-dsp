package resample

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"sync"
	"testing"
)

// referenceResampleAligned is a verbatim copy of the AudioVisualizer
// ResampleAligned implementation (internal/audioanalysis/load.go) that
// ProcessAligned replaces. Its output must stay bit-identical.
func referenceResampleAligned(x []float64, inputRate, outputRate int) ([]float64, error) {
	if inputRate <= 0 || outputRate <= 0 {
		return nil, errors.New("invalid sample rate")
	}

	if inputRate == outputRate {
		return append([]float64(nil), x...), nil
	}

	r, err := NewForRates(float64(inputRate), float64(outputRate), WithQuality(QualityBest))
	if err != nil {
		return nil, err
	}

	up, down := r.Ratio()
	delayInput := float64(len(r.Prototype())-1) / (2 * float64(up))
	delayOutput := float64(len(r.Prototype())-1) / (2 * float64(down))
	padded := make([]float64, len(x)+int(math.Ceil(delayInput))+4)
	copy(padded, x)
	raw := r.Process(padded)

	y := make([]float64, int(math.Round(float64(len(x))*float64(outputRate)/float64(inputRate))))
	for i := range y {
		pos := float64(i) + delayOutput
		j := int(pos)

		u := pos - float64(j)
		if j+1 < len(raw) {
			y[i] = raw[j]*(1-u) + raw[j+1]*u
		}
	}

	return y, nil
}

func deterministicNoise(seed uint64, n int) []float64 {
	rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))

	out := make([]float64, n)
	for i := range out {
		out[i] = 2*rng.Float64() - 1
	}

	return out
}

func TestProcessAlignedReferenceParity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		inRate, outRate int
		length          int
	}{
		{48000, 24000, 48000*2 + 123},
		{44100, 24000, 44100*2 + 77},
		{48000, 24000, 1},
		{44100, 24000, 3},
		{44100, 24000, 147},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d_%d_len%d", tt.inRate, tt.outRate, tt.length), func(t *testing.T) {
			t.Parallel()

			input := deterministicNoise(uint64(tt.inRate+tt.length), tt.length)

			want, err := referenceResampleAligned(input, tt.inRate, tt.outRate)
			if err != nil {
				t.Fatal(err)
			}

			got, err := ResampleAligned(input, float64(tt.inRate), float64(tt.outRate), WithQuality(QualityBest))
			if err != nil {
				t.Fatal(err)
			}

			checkOutputBits(t, got, want)
		})
	}
}

// alignedCentroid returns the amplitude-weighted centre of y.
func alignedCentroid(y []float64) float64 {
	var weight, moment float64
	for i, v := range y {
		weight += v
		moment += float64(i) * v
	}

	return moment / weight
}

func TestProcessAlignedImpulseTiming(t *testing.T) {
	t.Parallel()

	tests := []struct {
		inRate, outRate float64
		quality         Quality
	}{
		{48000, 24000, QualityBest},
		{44100, 24000, QualityBest},
		{24000, 48000, QualityBest},
		{44100, 48000, QualityBest},
		{48000, 44100, QualityBest},
		{48000, 24000, QualityBalanced},
		{44100, 24000, QualityFast},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%.0f_%.0f_q%d", tt.inRate, tt.outRate, tt.quality), func(t *testing.T) {
			t.Parallel()

			r, err := NewForRates(tt.inRate, tt.outRate, WithQuality(tt.quality))
			if err != nil {
				t.Fatal(err)
			}

			up, down := r.Ratio()

			for _, at := range []int{1000, 1001, 2047, 3333} {
				input := make([]float64, 4800)
				input[at] = 1

				output, err := r.ProcessAligned(input)
				if err != nil {
					t.Fatal(err)
				}

				want := float64(at) * float64(up) / float64(down)
				if got := alignedCentroid(output); math.Abs(got-want) > 0.01 {
					t.Fatalf("impulse at %d landed at %.5f, want %.5f", at, got, want)
				}

				peak := 0
				for i, v := range output {
					if math.Abs(v) > math.Abs(output[peak]) {
						peak = i
					}
				}

				if math.Abs(float64(peak)-want) > 1 {
					t.Fatalf("impulse at %d peaks at %d, want %.3f", at, peak, want)
				}
			}
		})
	}
}

func TestProcessAlignedAliasing(t *testing.T) {
	t.Parallel()

	for _, inRate := range []float64{48000, 44100} {
		t.Run(fmt.Sprintf("%.0f", inRate), func(t *testing.T) {
			t.Parallel()

			input := sine(16000, inRate, int(inRate))

			output, err := ResampleAligned(input, inRate, 24000, WithQuality(QualityBest))
			if err != nil {
				t.Fatal(err)
			}

			if residual := rms(output[1000 : len(output)-1000]); residual > 0.003 {
				t.Fatalf("16 kHz alias residual RMS %.5f > 0.003", residual)
			}
		})
	}
}

func TestProcessAlignedLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		up, down, length, want int
	}{
		{1, 2, 0, 0},
		{1, 2, 1, 1},
		{1, 2, 3, 2},
		{1, 2, 48000, 24000},
		{2, 1, 5, 10},
		{80, 147, 44100, 24000},
		{80, 147, 1, 1},
		{160, 147, 1000, 1088},
		{1, 48, 23, 0},
		{1, 48, 24, 1},
		{1, 48, 1000, 21},
		{48, 1, 3, 144},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d_%d_len%d", tt.up, tt.down, tt.length), func(t *testing.T) {
			t.Parallel()

			r := mustResampler(t, tt.up, tt.down)
			input := deterministicNoise(uint64(tt.length), tt.length)

			output, err := r.ProcessAligned(input)
			if err != nil {
				t.Fatal(err)
			}

			if output == nil || len(output) != tt.want {
				t.Fatalf("len=%d (nil=%t), want %d", len(output), output == nil, tt.want)
			}

			if tt.want > 0 && output[len(output)-1] == 0 && input[len(input)-1] != 0 {
				t.Fatal("last output sample lost its interpolation neighbour")
			}
		})
	}
}

func TestProcessAlignedPreservesStreamingState(t *testing.T) {
	t.Parallel()

	input := deterministicNoise(7, 1500)
	reference := mustResampler(t, 160, 147)
	want := append(reference.Process(input[:700]), reference.Process(input[700:])...)

	stream := mustResampler(t, 160, 147)
	first := stream.Process(input[:700])

	aligned, err := stream.ProcessAligned(input)
	if err != nil {
		t.Fatal(err)
	}

	checkOutputBits(t, append(first, stream.Process(input[700:])...), want)

	// Prior streaming state must not leak into the aligned result.
	fresh, err := mustResampler(t, 160, 147).ProcessAligned(input)
	if err != nil {
		t.Fatal(err)
	}

	checkOutputBits(t, aligned, fresh)
}

func TestProcessAlignedConcurrent(t *testing.T) {
	t.Parallel()

	r := mustResampler(t, 80, 147, WithQuality(QualityBest))
	input := deterministicNoise(11, 4410)

	want, err := r.ProcessAligned(input)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup

	results := make([][]float64, 4)
	for i := range results {
		wg.Go(func() {
			results[i], _ = r.ProcessAligned(input)
		})
	}

	wg.Wait()

	for _, got := range results {
		checkOutputBits(t, got, want)
	}
}

func TestResampleAlignedEdgeCases(t *testing.T) {
	t.Parallel()

	empty, err := ResampleAligned(nil, 48000, 24000)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty input: out=%v err=%v", empty, err)
	}

	input := []float64{1, 2, 3}

	same, err := ResampleAligned(input, 48000, 48000)
	if err != nil {
		t.Fatal(err)
	}

	same[0] = 9

	if input[0] != 1 || len(same) != len(input) {
		t.Fatal("equal rates must return an independent unfiltered copy")
	}

	for _, rates := range [][2]float64{{0, 48000}, {48000, -1}, {math.NaN(), 1}, {1, math.Inf(1)}} {
		if _, err := ResampleAligned(input, rates[0], rates[1]); !errors.Is(err, ErrInvalidRate) {
			t.Fatalf("rates %v: err=%v, want ErrInvalidRate", rates, err)
		}
	}
}
