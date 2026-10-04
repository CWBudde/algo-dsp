package truepeak

import (
	"math"
	"testing"
)

func TestTruePeakAnalyticIntersampleTone(t *testing.T) {
	input := make([]float64, 4096)
	samplePeak := 0.0

	for i := range input {
		input[i] = math.Sin(math.Pi*float64(i)/2 + math.Pi/4)
		samplePeak = math.Max(samplePeak, math.Abs(input[i]))
	}

	m, _ := NewMeter(1)
	if err := m.ProcessPlanar([][]float64{input}); err != nil {
		t.Fatal(err)
	}

	m.Flush()

	peak := make([]float64, 1)
	if err := m.PeaksInto(peak); err != nil {
		t.Fatal(err)
	}
	// Analytic continuous tone reaches unity between the .7071 sample peaks.
	// Four phases have the BS.1770 quarter-grid under-read, plus FIR ripple.
	if samplePeak > .708 || peak[0] < .97 || peak[0] > 1.04 {
		t.Fatalf("sample%g interpolated%g", samplePeak, peak[0])
	}
}

func TestTruePeakPartitionFormatResetAndFlush(t *testing.T) {
	packed := make([]float32, 4099*2)
	planar32 := [][]float32{make([]float32, 4099), make([]float32, 4099)}
	planar64 := [][]float64{make([]float64, 4099), make([]float64, 4099)}

	for i := range planar32[0] {
		for ch := range planar32 {
			x := float32(.6 * math.Sin(float64(i)*(.119+float64(ch)*.7)))
			if i == 4098 {
				x = -.9
			}

			planar32[ch][i] = x
			planar64[ch][i] = float64(x)
			packed[2*i+ch] = x
		}
	}

	baseline, _ := NewMeter(2)
	if err := baseline.ProcessPlanar(planar64); err != nil {
		t.Fatal(err)
	}

	baseline.Flush()

	want := make([]float64, 2)
	_ = baseline.PeaksInto(want)

	for _, partition := range []int{1, 17, 128, 4099} {
		for mode := 0; mode < 3; mode++ {
			m, _ := NewMeter(2)
			for pass := 0; pass < 2; pass++ {
				m.Reset()

				for offset := 0; offset < 4099; offset += partition {
					end := min(offset+partition, 4099)

					var err error

					switch mode {
					case 0:
						err = m.ProcessPlanar([][]float64{planar64[0][offset:end], planar64[1][offset:end]})
					case 1:
						err = m.ProcessPlanar32([][]float32{planar32[0][offset:end], planar32[1][offset:end]})
					case 2:
						err = m.ProcessInterleaved32(packed[2*offset : 2*end])
					}

					if err != nil {
						t.Fatal(err)
					}
				}

				m.Flush()
				m.Flush()

				got := make([]float64, 2)

				_ = m.PeaksInto(got)
				for ch := range got {
					if got[ch] != want[ch] {
						t.Fatalf("partition%d mode%d channel%d got%g want%g", partition, mode, ch, got[ch], want[ch])
					}
				}

				if err := m.ProcessInterleaved32(nil); err == nil {
					t.Fatal("flushed input accepted")
				}
			}
		}
	}
}

func TestTruePeakValidationAndAllocation(t *testing.T) {
	for _, channels := range []int{0, 33} {
		if _, err := NewMeter(channels); err == nil {
			t.Fatal("invalid channels accepted")
		}
	}

	m, _ := NewMeter(2)
	if err := m.ProcessPlanar([][]float64{{.25}, {.5}}); err != nil {
		t.Fatal(err)
	}

	beforePosition, beforePeak := m.position, m.peaks[1]
	for _, block := range [][][]float64{{{1}}, {{1}, {1, 2}}, {{1}, {math.Inf(1)}}, {{1}, {math.NaN()}}, {{1}, {1e200}}} {
		if err := m.ProcessPlanar(block); err == nil {
			t.Fatal("invalid block accepted")
		}

		if m.position != beforePosition || m.peaks[1] != beforePeak {
			t.Fatal("invalid block changed history")
		}
	}

	if err := m.ProcessInterleaved32([]float32{1}); err == nil {
		t.Fatal("partial frame accepted")
	}

	if err := m.PeaksInto(make([]float64, 1)); err == nil {
		t.Fatal("short destination accepted")
	}

	block := make([]float32, 256)
	peaks := make([]float64, 2)

	if n := testing.AllocsPerRun(10, func() {
		m.Reset()

		for range 16 {
			if err := m.ProcessInterleaved32(block); err != nil {
				panic(err)
			}
		}

		m.Flush()

		if err := m.PeaksInto(peaks); err != nil {
			panic(err)
		}
	}); n != 0 {
		t.Fatalf("allocations%g", n)
	}
}

func BenchmarkTruePeakStereo128(b *testing.B) {
	m, _ := NewMeter(2)

	block := make([]float32, 256)
	for i := range block {
		block[i] = float32(.3 * math.Sin(float64(i)*.3))
	}

	b.ReportAllocs()
	b.SetBytes(256 * 4)
	b.ResetTimer()

	for range b.N {
		if err := m.ProcessInterleaved32(block); err != nil {
			b.Fatal(err)
		}
	}
}
