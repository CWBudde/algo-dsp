package effectchain

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
)

func catalogGraph(t *testing.T, d Descriptor, preset FactoryPreset) string {
	t.Helper()

	params := map[string]any{}
	for id, v := range preset.Num {
		params[id] = v
	}

	for id, v := range preset.Str {
		params[id] = v
	}

	graph := graphState{Nodes: []graphNode{{ID: InputNodeID, Type: InputNodeID}, {ID: "fx", Type: d.ID, Params: params}, {ID: OutputNodeID, Type: OutputNodeID}}, Connections: []graphConnection{{From: InputNodeID, To: "fx"}, {From: "fx", To: OutputNodeID}}}

	b, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}

	return string(b)
}

func preparedCatalogChain(t *testing.T, d Descriptor, preset FactoryPreset, maxFrames int) *Chain {
	t.Helper()

	c := New(Context{SampleRate: 48000}, DefaultRegistry(WithIRProvider(catalogIR{48000})))
	if err := c.LoadGraph(catalogGraph(t, d, preset)); err != nil {
		t.Fatal(err)
	}

	if err := c.PreparePlanar(2, maxFrames); err != nil {
		t.Fatal(err)
	}

	return c
}

func TestCatalogueStreamingPartitionResetAndAllocations(t *testing.T) {
	const frames = 16385

	for _, d := range DefaultDescriptors(48000) {
		for _, preset := range []FactoryPreset{d.Presets[0], d.Presets[len(d.Presets)-1]} {
			t.Run(d.ID+"/"+preset.ID, func(t *testing.T) {
				c := preparedCatalogChain(t, d, preset, frames)

				source := [][]float64{make([]float64, frames), make([]float64, frames)}
				for i := range source[0] {
					source[0][i] = 0.2*math.Sin(float64(i)*0.043) + 0.01*math.Cos(float64(i)*0.173)
					source[1][i] = 0.14 * math.Sin(float64(i)*0.017+0.7)
				}

				whole := [][]float64{append([]float64(nil), source[0]...), append([]float64(nil), source[1]...)}
				if err := c.ProcessPlanar(whole); err != nil {
					t.Fatal(err)
				}

				for _, sizes := range [][]int{{1}, {128}, {512}, {17, 3, 509, 2, 382}} {
					if err := c.ResetProcessing(); err != nil {
						t.Fatal(err)
					}

					parts := [][]float64{append([]float64(nil), source[0]...), append([]float64(nil), source[1]...)}
					views := make([][]float64, 2)

					for position, iteration := 0, 0; position < frames; iteration++ {
						end := min(position+sizes[iteration%len(sizes)], frames)
						views[0] = parts[0][position:end]

						views[1] = parts[1][position:end]
						if err := c.ProcessPlanar(views); err != nil {
							t.Fatal(err)
						}

						position = end
					}

					for channel := range whole {
						for i, want := range whole[channel] {
							if parts[channel][i] != want {
								t.Fatalf("partition%v channel%d frame%d: got%.17g want%.17g", sizes, channel, i, parts[channel][i], want)
							}
						}
					}
				}

				fresh := preparedCatalogChain(t, d, preset, 128)
				small := [][]float64{make([]float64, 128), make([]float64, 128)}

				allocationCount := testing.AllocsPerRun(3, func() {
					if err := fresh.ResetProcessing(); err != nil {
						panic(err)
					}

					for range 256 {
						for i := range small[0] {
							small[0][i] = 0.1
							small[1][i] = -0.05
						}

						if err := fresh.ProcessPlanar(small); err != nil {
							panic(err)
						}
					}
				})
				if allocationCount != 0 {
					t.Errorf("prepared reset/render allocations=%g", allocationCount)
				}
			})
		}
	}
}

func TestPlanarStereoRoutingAndAtomicShape(t *testing.T) {
	d := Descriptor{ID: "panner"}
	preset := FactoryPreset{Num: map[string]float64{"position": -1}, Str: map[string]string{"law": "equal-power"}}
	c := preparedCatalogChain(t, d, preset, 128)

	left, right := []float64{0.3, -0.2}, []float64{0.7, 0.4}
	if err := c.ProcessPlanar([][]float64{left, right}); err != nil {
		t.Fatal(err)
	}

	if left[0] == 0 || math.Abs(right[0]) > 1e-15 || math.Abs(right[1]) > 1e-15 {
		t.Fatalf("hard-left panner output L%v R%v", left, right)
	}

	if err := c.ResetProcessing(); err != nil {
		t.Fatal(err)
	}

	if err := c.ProcessPlanar([][]float64{{0.2}, {0.3, 0.4}}); err == nil {
		t.Error("unequal lengths accepted")
	}

	if err := c.PreparePlanar(1, 128); err == nil {
		t.Error("incomplete stereo pair accepted")
	}
}

func BenchmarkCataloguePreparedPlanar(b *testing.B) {
	for _, d := range DefaultDescriptors(48000) {
		b.Run(d.ID, func(b *testing.B) {
			c := New(Context{SampleRate: 48000}, DefaultRegistry(WithIRProvider(catalogIR{48000})))

			params := map[string]any{}
			for k, v := range d.Presets[len(d.Presets)-1].Num {
				params[k] = v
			}

			for k, v := range d.Presets[len(d.Presets)-1].Str {
				params[k] = v
			}

			node, _ := json.Marshal(params)

			graph := fmt.Sprintf(`{"nodes":[{"id":"_input","type":"_input"},{"id":"fx","type":%q,"params":%s},{"id":"_output","type":"_output"}],"connections":[{"from":"_input","to":"fx"},{"from":"fx","to":"_output"}]}`, d.ID, node)
			if err := c.LoadGraph(graph); err != nil {
				b.Fatal(err)
			}

			if err := c.PreparePlanar(2, 128); err != nil {
				b.Fatal(err)
			}

			block := [][]float64{make([]float64, 128), make([]float64, 128)}

			for range 256 {
				if err := c.ProcessPlanar(block); err != nil {
					b.Fatal(err)
				}
			}

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				if err := c.ProcessPlanar(block); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
