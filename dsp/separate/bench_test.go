package separate_test

import (
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/separate"
	"github.com/cwbudde/algo-dsp/internal/testutil"
)

func BenchmarkHPSSSeparate(b *testing.B) {
	h, err := separate.NewHPSS()
	if err != nil {
		b.Fatal(err)
	}

	x := testutil.DeterministicNoise(1, 0.5, 24000) // 1 s at 24 kHz

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, _, _, err := h.SeparateSignal(x)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHPSSSeparateInto(b *testing.B) {
	h, err := separate.NewHPSS()
	if err != nil {
		b.Fatal(err)
	}

	spec, err := h.STFT().Forward(testutil.DeterministicNoise(1, 0.5, 24000))
	if err != nil {
		b.Fatal(err)
	}

	hs, ps, rs, err := h.Separate(spec)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		err := h.SeparateInto(hs, ps, rs, spec)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMedianFilter(b *testing.B) {
	for _, size := range []int{17, 31} {
		b.Run(map[int]string{17: "k17", 31: "k31"}[size], func(b *testing.B) {
			m, err := separate.NewMedianFilter(size)
			if err != nil {
				b.Fatal(err)
			}

			src := testutil.DeterministicNoise(2, 1, 1025)
			dst := make([]float64, len(src))

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				err := m.Filter(dst, src)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkSoftMasks(b *testing.B) {
	const frames, bins = 47, 1025

	est := [][][]float64{make([][]float64, frames), make([][]float64, frames), make([][]float64, frames), make([][]float64, frames)}
	dst := [][][]float64{make([][]float64, frames), make([][]float64, frames), make([][]float64, frames), make([][]float64, frames)}

	for s := range est {
		noise := testutil.DeterministicNoise(int64(s+1), 1, frames*bins)

		for i := range frames {
			est[s][i] = noise[i*bins : (i+1)*bins]
			for k := range est[s][i] {
				if est[s][i][k] < 0 {
					est[s][i][k] = -est[s][i][k]
				}
			}

			dst[s][i] = make([]float64, bins)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		err := separate.SoftMasks(dst, est, 2)
		if err != nil {
			b.Fatal(err)
		}
	}
}
