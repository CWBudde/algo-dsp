package effectchain

import (
	"fmt"
	"math"
	"reflect"
	"testing"
)

type catalogIR struct{ rate float64 }

func (p catalogIR) GetIR(int) ([][]float64, float64, bool) {
	return [][]float64{{1, 0.2, 0, -0.1}, {0.5, 0, 0.1, 0}}, p.rate, true
}

func TestDescriptorCatalogue(t *testing.T) {
	for _, sr := range []float64{8000, 44100, 48000, 384000} {
		descriptors := DefaultDescriptors(sr)
		registry := DefaultRegistry(WithIRProvider(catalogIR{sr}))
		ids := make([]string, len(descriptors))

		for i, d := range descriptors {
			ids[i] = d.ID
			t.Run(fmt.Sprintf("%s/%g", d.ID, sr), func(t *testing.T) {
				if d.ID == "" || d.Name == "" || d.Category == "" || (d.ChannelMode != "mono" && d.ChannelMode != "stereo") || (d.View != "generic" && d.View != "eq" && d.View != "dynamics") {
					t.Errorf("incomplete descriptor %+v", d)
				}

				seen := map[string]bool{}

				for _, p := range d.Parameters {
					if p.ID == "" || p.Label == "" || (p.Type != "enum" && p.Type != "number" && p.Type != "boolean") || (p.Scale != "lin" && p.Scale != "log" && p.Scale != "dB") {
						t.Errorf("incomplete parameter %+v", p)
					}

					for _, value := range []float64{p.Min, p.Max, p.Default, p.Step} {
						if math.IsNaN(value) || math.IsInf(value, 0) {
							t.Errorf("nonfinite metadata %+v", p)
						}
					}

					if seen[p.ID] {
						t.Errorf("duplicate parameter %s", p.ID)
					}

					seen[p.ID] = true
					if p.Type == "enum" {
						found := false

						options := map[string]bool{}
						for _, o := range p.Options {
							if o.Value == "" || o.Label == "" || options[o.Value] {
								t.Errorf("invalid enum option %+v", o)
							}

							options[o.Value] = true
							if o.Value == p.DefaultString {
								found = true
							}
						}

						if !found {
							t.Errorf("default enum %s=%q absent", p.ID, p.DefaultString)
						}
					} else if p.Min > p.Default || p.Default > p.Max || p.Step <= 0 || (p.Type == "boolean" && (p.Min != 0 || p.Max != 1 || (p.Default != 0 && p.Default != 1))) {
						t.Errorf("invalid numeric metadata %+v", p)
					}
				}

				for _, preset := range d.Presets {
					r, err := registry.Lookup(d.ID)(Context{SampleRate: sr})
					if err != nil {
						t.Fatal(err)
					}

					if err = r.Configure(Context{SampleRate: sr}, Params{Type: d.ID, Num: preset.Num, Str: preset.Str}); err != nil {
						t.Fatal(err)
					}

					block := make([]float64, 4096)
					for i := range block {
						block[i] = 0.15 * math.Sin(float64(i)*0.031)
					}

					r.Process(block)

					for i, x := range block {
						if math.IsNaN(x) || math.IsInf(x, 0) {
							t.Fatalf("nonfinite frame%d", i)
						}
					}
				}
			})
		}

		if !reflect.DeepEqual(ids, registry.Types()) {
			t.Fatalf("catalogue mismatch descriptors=%v registry=%v", ids, registry.Types())
		}
	}
}

func TestDescriptorInvalidRatesUseOwnedFallback(t *testing.T) {
	want := DefaultDescriptors(44100)
	for _, rate := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if got := DefaultDescriptors(rate); !reflect.DeepEqual(got, want) {
			t.Errorf("invalid rate %v did not use canonical fallback", rate)
		}
	}
}

func TestDescriptorUnitsAndOwnership(t *testing.T) {
	first, second := DefaultDescriptors(44100), DefaultDescriptors(44100)
	for i, d := range first {
		for j, p := range d.Parameters {
			if d.ID == "pitch-time" && (p.ID == "sequence" || p.ID == "search" || p.ID == "overlap") && p.Unit != "ms" {
				t.Errorf("pitch unit %s", p.ID)
			}

			if d.ID == "granular" && p.ID == "overlap" && p.Unit != "" {
				t.Error("granular overlap must be ratio")
			}

			if (d.ID == "dist-cheb" || d.ID == "reverb-freeverb") && p.ID == "gain" && p.Unit != "" {
				t.Error("linear gain labeled dB")
			}

			first[i].Parameters[j].Label = "changed"
			if second[i].Parameters[j].Label == "changed" {
				t.Error("parameter alias")
			}

			if len(p.Options) > 0 {
				first[i].Parameters[j].Options[0].Label = "modified"
				if second[i].Parameters[j].Options[0].Label == "modified" {
					t.Error("enum options alias across calls")
				}
			}
		}
	}
}
