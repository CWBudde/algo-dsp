package effectchain

import (
	"fmt"
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
)

func TestParametricEQDefaultLayout(t *testing.T) {
	for _, sr := range []float64{8000, 16000, 44100, 48000, 96000} {
		t.Run(fmt.Sprint(sr), func(t *testing.T) {
			for _, d := range DefaultDescriptors(sr) {
				if d.ID != "eq-parametric" && d.ID != "dyn-eq" {
					continue
				}

				preset := d.Presets[0]
				if d.ID == "dyn-eq" {
					if preset.Num["bands"] != 4 || preset.Str["band1Type"] != "peak" || preset.Num["band1FreqHz"] != 80 {
						t.Fatal("dynamic EQ defaults changed")
					}

					continue
				}

				if preset.Num["bands"] != 6 {
					t.Fatal("want six bands")
				}

				previous := 0.0

				for i, kind := range []string{"highpass", "lowshelf", "peak", "peak", "highshelf", "lowpass"} {
					prefix := fmt.Sprintf("band%d", i+1)

					hz := preset.Num[prefix+"FreqHz"]
					if preset.Str[prefix+"Type"] != kind || hz <= previous || hz >= sr/2 {
						t.Fatalf("invalid band %d: %g Hz, %s", i+1, hz, preset.Str[prefix+"Type"])
					}

					if preset.Num[prefix+"Order"] != 2 {
						t.Fatal("default band order must preserve second-order filters")
					}

					if i > 0 && (hz/previous < 2.4 || hz/previous > 3.6) {
						t.Fatalf("uneven logarithmic spacing: %g / %g", hz, previous)
					}

					previous = hz
				}

				vocal := d.Presets[1]
				if vocal.Str["band1Type"] != "lowshelf" || vocal.Str["band2Type"] != "peak" || vocal.Str["band3Type"] != "highshelf" {
					t.Fatal("Vocal presence types changed")
				}
			}
		})
	}
}

func TestParametricEQBandOrderProcessingAndResponse(t *testing.T) {
	for _, kind := range []string{"highpass", "lowpass", "peak", "lowshelf", "highshelf"} {
		for _, order := range []int{2, 4, 6, 8, 10, 12} {
			t.Run(fmt.Sprintf("%s/order%d", kind, order), func(t *testing.T) {
				preset := FactoryPreset{Num: map[string]float64{"bands": 1, "band1FreqHz": 1000, "band1GainDB": 12, "band1Q": 1, "band1Order": float64(order)}, Str: map[string]string{"band1Type": kind}}

				chain := New(Context{SampleRate: 48000}, DefaultRegistry())
				if err := chain.LoadGraph(catalogGraph(t, Descriptor{ID: "eq-parametric"}, preset)); err != nil {
					t.Fatal(err)
				}

				coeffs, err := design.ParametricBand(48000, 1000, 12, 1, kind, order)
				if err != nil {
					t.Fatal(err)
				}

				reference := biquad.NewChain(coeffs)
				frequencies := []float64{100, 1000, 3000, 20000}

				response, err := chain.Response(frequencies)
				if err != nil {
					t.Fatal(err)
				}

				for i, f := range frequencies {
					want := math.Pow(10, reference.MagnitudeDB(f, 48000)/20)
					if math.Abs(response[i]-want) > 1e-10 {
						t.Fatalf("%g Hz: %g, want %g", f, response[i], want)
					}
				}

				if err := chain.PreparePlanar(1, 128); err != nil {
					t.Fatal(err)
				}

				want := make([]float64, 2048)
				want[0] = 1
				reference.ProcessBlock(want)

				for offset := 0; offset < len(want); offset += 128 {
					block := [][]float64{make([]float64, 128)}
					if offset == 0 {
						block[0][0] = 1
					}

					if err := chain.ProcessPlanar(block); err != nil {
						t.Fatal(err)
					}

					for i, sample := range block[0] {
						if math.Abs(sample-want[offset+i]) > 1e-12 {
							t.Fatalf("sample%d: %g, want %g", offset+i, sample, want[offset+i])
						}
					}
				}
			})
		}
	}
}

func TestParametricEQPassBandsProcessAndInspect(t *testing.T) {
	for _, sr := range []float64{8000, 48000, 96000} {
		for _, kind := range []string{"highpass", "lowpass"} {
			t.Run(fmt.Sprintf("%g/%s", sr, kind), func(t *testing.T) {
				hz, q := sr/8, math.Sqrt(0.5)

				coeff := design.Highpass(hz, q, sr)
				if kind == "lowpass" {
					coeff = design.Lowpass(hz, q, sr)
				}

				for _, gain := range []float64{-24, 0, 24} {
					preset := FactoryPreset{Num: map[string]float64{"bands": 1, "band1FreqHz": hz, "band1Q": q, "band1GainDB": gain}, Str: map[string]string{"band1Type": kind}}

					chain := New(Context{SampleRate: sr}, DefaultRegistry())
					if err := chain.LoadGraph(catalogGraph(t, Descriptor{ID: "eq-parametric"}, preset)); err != nil {
						t.Fatal(err)
					}

					frequencies := []float64{hz / 10, hz, sr * .45}

					response, err := chain.Response(frequencies)
					if err != nil {
						t.Fatal(err)
					}

					for i, f := range frequencies {
						if want := math.Sqrt(coeff.MagnitudeSquared(f, sr)); math.Abs(response[i]-want) > 1e-12 {
							t.Fatalf("response %g Hz: %g, want %g", f, response[i], want)
						}
					}

					if math.Abs(20*math.Log10(response[1])+3.010299956639812) > 1e-9 {
						t.Fatal("pass cutoff must be -3 dB")
					}

					if err := chain.PreparePlanar(1, 128); err != nil {
						t.Fatal(err)
					}

					block := [][]float64{make([]float64, 128)}

					block[0][0] = 1
					if err := chain.ProcessPlanar(block); err != nil {
						t.Fatal(err)
					}

					section := biquad.NewSection(coeff)

					for i, got := range block[0] {
						input := 0.0
						if i == 0 {
							input = 1
						}

						if want := section.ProcessSample(input); math.Abs(got-want) > 1e-12 {
							t.Fatalf("sample %d: %g, want %g", i, got, want)
						}
					}
				}
			})
		}
	}
}
