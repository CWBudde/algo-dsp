package effectchain

import (
	"errors"
	"math"
	"testing"
)

func loadTestChain(t *testing.T, graph string) *Chain {
	t.Helper()

	c := New(Context{SampleRate: 48000}, DefaultRegistry(WithIRProvider(catalogIR{48000})))
	if err := c.LoadGraph(graph); err != nil {
		t.Fatal(err)
	}

	return c
}

func TestResponseIndependentFilterGoldensAndNoStateAdvance(t *testing.T) {
	d := Descriptor{ID: "filter-lowpass"}
	preset := FactoryPreset{Num: map[string]float64{"freq": 1000, "q": math.Sqrt(0.5)}, Str: map[string]string{"family": "rbj"}}
	c := preparedCatalogChain(t, d, preset, 128)

	response, err := c.Response([]float64{0, 1000, 24000})
	if err != nil {
		t.Fatal(err)
	}

	for i, want := range []float64{1, math.Sqrt(0.5), 0} {
		if math.Abs(response[i]-want) > 1e-12 {
			t.Errorf("response%d got%.15g want%.15g", i, response[i], want)
		}
	}

	input := [][]float64{{0.2, -0.3, 0.1}, {0.5, 0.1, -0.7}}
	if err = c.ProcessPlanar(input); err != nil {
		t.Fatal(err)
	}

	if _, err = c.Response([]float64{50, 500, 1200}); err != nil {
		t.Fatal(err)
	}

	fresh := preparedCatalogChain(t, d, preset, 128)

	reference := [][]float64{{0.2, -0.3, 0.1}, {0.5, 0.1, -0.7}}
	if err = fresh.ProcessPlanar(reference); err != nil {
		t.Fatal(err)
	}

	for ch := range reference {
		for i, x := range reference[ch] {
			if input[ch][i] != x {
				t.Error("response advanced filter history")
			}
		}
	}

	if _, err = c.Response([]float64{math.NaN()}); err == nil {
		t.Error("unsafe response probe accepted")
	}

	for _, family := range []string{"butterworth", "bessel", "chebyshev1", "chebyshev2", "elliptic"} {
		preset.Str["family"] = family
		preset.Num["order"] = 4
		preset.Num["stopbandDB"] = 40
		filter := preparedCatalogChain(t, d, preset, 128)

		values, err := filter.Response([]float64{0, 100, 10000})
		if err != nil {
			t.Fatal(err)
		}

		if values[2] >= values[1]*0.1 {
			t.Errorf("%s not effective lowpass: %v", family, values)
		}
	}
}

func TestTransferIndependentDynamicsGoldens(t *testing.T) {
	d := Descriptor{ID: "dyn-compressor"}
	preset := FactoryPreset{Num: map[string]float64{"thresholdDB": -20, "ratio": 4, "kneeDB": 0, "makeupGainDB": 0}}
	c := preparedCatalogChain(t, d, preset, 128)

	levels, err := c.Transfer([]float64{-40, -20, -12, 0})
	if err != nil {
		t.Fatal(err)
	}

	for i, want := range []float64{-40, -20, -18, -15} {
		if math.Abs(levels[i]-want) > 1e-9 {
			t.Errorf("transfer%d got%g want%g", i, levels[i], want)
		}
	}

	if _, err = c.Transfer([]float64{math.Inf(1)}); err == nil {
		t.Error("unsafe transfer probe accepted")
	}

	if _, err = c.Response([]float64{1000}); err == nil {
		t.Error("nonlinear graph accepted as linear response")
	}
}

func TestLatencyDAGAlignsBranchesAndChannels(t *testing.T) {
	c := loadTestChain(t, `{"nodes":[{"id":"_input","type":"_input"},{"id":"freeze","type":"spectral-freeze","params":{"frozen":0}},{"id":"_output","type":"_output"}],"connections":[{"from":"_input","to":"freeze"},{"from":"freeze","to":"_output"},{"from":"_input","to":"_output"}]}`)
	if c.Latency() != 1024 {
		t.Fatalf("DAG latency%d want1024", c.Latency())
	}

	if err := c.PreparePlanar(2, 128); err != nil {
		t.Fatal(err)
	}

	const frames = 7000

	input := [][]float64{make([]float64, frames), make([]float64, frames)}
	original := [][]float64{make([]float64, frames), make([]float64, frames)}

	for ch := range input {
		for i := range input[ch] {
			input[ch][i] = 0.3 * math.Sin(float64(i)*0.023+float64(ch))
			original[ch][i] = input[ch][i]
		}
	}

	views := make([][]float64, 2)

	for pos := 0; pos < frames; pos += 128 {
		end := min(pos+128, frames)
		views[0] = input[0][pos:end]

		views[1] = input[1][pos:end]
		if err := c.ProcessPlanar(views); err != nil {
			t.Fatal(err)
		}
	}

	for ch := range input {
		for i := 1; i < frames-c.Latency(); i++ {
			if math.Abs(input[ch][i+c.Latency()]-original[ch][i]) > 1e-9 {
				t.Fatalf("unaligned channel%d sample%d got%.12g want%.12g", ch, i, input[ch][i+c.Latency()], original[ch][i])
			}
		}
	}

	if allocations := testing.AllocsPerRun(3, func() {
		_ = c.ResetProcessing()
		views[0] = input[0][:128]
		views[1] = input[1][:128]
		_ = c.ProcessPlanar(views)
	}); allocations != 0 {
		t.Errorf("DAG reset/render allocations%g", allocations)
	}

	c.Reset()

	if c.Latency() != 0 {
		t.Error("reset retained latency")
	}
}

func TestConvolutionStereoIRRateAndAlignedDry(t *testing.T) {
	d := Descriptor{ID: "reverb-conv"}

	c := preparedCatalogChain(t, d, FactoryPreset{Num: map[string]float64{"wet": 0.35}}, 512)
	if c.Latency() != 128 {
		t.Fatalf("convolution latency%d", c.Latency())
	}

	block := [][]float64{make([]float64, 512), make([]float64, 512)}
	block[0][0] = 1

	block[1][0] = 1
	if err := c.ProcessPlanar(block); err != nil {
		t.Fatal(err)
	}

	if math.Abs(block[0][128]-1.35) > 1e-12 || math.Abs(block[1][128]-1.175) > 1e-12 {
		t.Fatalf("stereo impulse L%g R%g", block[0][128], block[1][128])
	}

	for _, sr := range []float64{0, 44100} {
		chain := New(Context{SampleRate: 48000}, DefaultRegistry(WithIRProvider(catalogIR{sr})))
		if err := chain.LoadGraph(catalogGraph(t, d, FactoryPreset{})); err == nil {
			t.Errorf("IR rate%g accepted", sr)
		}
	}

	missing := New(Context{SampleRate: 48000}, DefaultRegistry())
	if err := missing.LoadGraph(catalogGraph(t, d, FactoryPreset{})); err == nil {
		t.Error("missing IR silently bypassed")
	}
}

type checkedFailure struct{}

var errChecked = errors.New("checked render failed")

func (checkedFailure) Configure(Context, Params) error  { return nil }
func (checkedFailure) Process([]float64)                {}
func (checkedFailure) ProcessWithError([]float64) error { return errChecked }
func (checkedFailure) Reset()                           {}
func TestCheckedRuntimeErrorsReachMonoAndPlanar(t *testing.T) {
	registry := NewRegistry()
	registry.MustRegister("failure", func(Context) (Runtime, error) { return checkedFailure{}, nil })

	c := New(Context{SampleRate: 48000}, registry)
	if err := c.LoadGraph(catalogGraph(t, Descriptor{ID: "failure"}, FactoryPreset{})); err != nil {
		t.Fatal(err)
	}

	if c.Process([]float64{0.1}) || !errors.Is(c.Err(), errChecked) {
		t.Error("mono error lost")
	}

	if err := c.PreparePlanar(1, 128); err != nil {
		t.Fatal(err)
	}

	if err := c.ProcessPlanar([][]float64{{0.1}}); !errors.Is(err, errChecked) {
		t.Errorf("planar error lost:%v", err)
	}

	if _, retained := c.outBuf[InputNodeID]; retained {
		t.Error("failed render retained caller input")
	}
}

func TestEveryAdvertisedCurveWorksForPresets(t *testing.T) {
	for _, descriptor := range DefaultDescriptors(48000) {
		if descriptor.View != "eq" && descriptor.View != "dynamics" {
			continue
		}

		for _, preset := range descriptor.Presets {
			t.Run(descriptor.ID+"/"+preset.ID, func(t *testing.T) {
				c := preparedCatalogChain(t, descriptor, preset, 128)

				var (
					values []float64
					err    error
				)
				if descriptor.View == "eq" {
					values, err = c.Response([]float64{0, 50, 1000, 10000, 24000})
				} else {
					values, err = c.Transfer([]float64{-100, -40, -20, -6, 0})
				}

				if err != nil {
					t.Fatal(err)
				}

				for _, v := range values {
					if math.IsNaN(v) || math.IsInf(v, 0) {
						t.Fatalf("nonfinite curve: %v", values)
					}
				}
			})
		}
	}
}

func TestPreparePlanarGrowthRetainsProcessingState(t *testing.T) {
	d := Descriptor{ID: "filter-lowpass"}
	preset := FactoryPreset{Num: map[string]float64{"freq": 1000}}
	a, b := preparedCatalogChain(t, d, preset, 128), preparedCatalogChain(t, d, preset, 128)
	first := [][]float64{{1, 0, 0}, {0, 1, 0}}
	second := [][]float64{{1, 0, 0}, {0, 1, 0}}

	if err := a.ProcessPlanar(first); err != nil {
		t.Fatal(err)
	}

	if err := b.ProcessPlanar(second); err != nil {
		t.Fatal(err)
	}

	if err := a.PreparePlanar(2, 256); err != nil {
		t.Fatal(err)
	}

	first = [][]float64{{0, 0, 0}, {0, 0, 0}}
	second = [][]float64{{0, 0, 0}, {0, 0, 0}}

	if err := a.ProcessPlanar(first); err != nil {
		t.Fatal(err)
	}

	if err := b.ProcessPlanar(second); err != nil {
		t.Fatal(err)
	}

	for ch := range first {
		for i, v := range first[ch] {
			if v != second[ch][i] {
				t.Fatalf("preparation reset channel%d", ch)
			}
		}
	}
}
