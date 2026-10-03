package effectchain_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effectchain"
)

func ExampleDefaultDescriptors() {
	catalog := effectchain.DefaultDescriptors(48000)
	fmt.Println("Built-in effects:", len(catalog))
	fmt.Println("Registered factories:", len(effectchain.DefaultRegistry().Types()))
	// Output:
	// Built-in effects: 51
	// Registered factories: 51
}

func ExampleChain_PreparePlanar() {
	chain := effectchain.New(effectchain.Context{SampleRate: 48000}, effectchain.DefaultRegistry())

	err := chain.LoadGraph(`{"nodes":[{"id":"_input","type":"_input"},{"id":"pan","type":"panner","params":{"position":-1}},{"id":"_output","type":"_output"}],"connections":[{"from":"_input","to":"pan"},{"from":"pan","to":"_output"}]}`)
	if err != nil {
		panic(err)
	}

	if err = chain.PreparePlanar(2, 128); err != nil {
		panic(err)
	}

	channels := [][]float64{{0.25, -0.5}, {0.75, 0.5}}
	if err = chain.ProcessPlanar(channels); err != nil {
		panic(err)
	}

	fmt.Printf("Left: %.2f %.2f\n", channels[0][0], channels[0][1])

	if err = chain.ResetProcessing(); err != nil {
		panic(err)
	}

	fmt.Println("Latency:", chain.Latency())
	// Output:
	// Left: 0.25 -0.50
	// Latency: 0
}

func ExampleChain_Response() {
	chain := effectchain.New(effectchain.Context{SampleRate: 48000}, effectchain.DefaultRegistry())

	err := chain.LoadGraph(`{"nodes":[{"id":"_input","type":"_input"},{"id":"eq","type":"eq-parametric","params":{"bands":1,"band1FreqHz":1000,"band1GainDB":6}},{"id":"_output","type":"_output"}],"connections":[{"from":"_input","to":"eq"},{"from":"eq","to":"_output"}]}`)
	if err != nil {
		panic(err)
	}

	response, err := chain.Response([]float64{1000})
	if err != nil {
		panic(err)
	}

	fmt.Printf("1 kHz magnitude: %.6f\n", response[0])
	// Output: 1 kHz magnitude: 1.995262
}

type mappedExampleIR struct{}

func (mappedExampleIR) GetIR(int) ([][]float64, float64, bool) {
	return [][]float64{{1}, {.5}}, 48000, true
}

func ExampleWithSourceChannelMap() {
	registry := effectchain.DefaultRegistry(effectchain.WithIRProvider(mappedExampleIR{}), effectchain.WithSourceChannelMap([]int{1}))

	chain := effectchain.New(effectchain.Context{SampleRate: 48000}, registry)
	if err := chain.LoadGraph(`{"nodes":[{"id":"_input","type":"_input"},{"id":"reverb","type":"reverb-conv","params":{"wet":1}},{"id":"_output","type":"_output"}],"connections":[{"from":"_input","to":"reverb"},{"from":"reverb","to":"_output"}]}`); err != nil {
		panic(err)
	}

	if err := chain.PreparePlanar(1, 129); err != nil {
		panic(err)
	}

	block := [][]float64{make([]float64, 129)}

	block[0][0] = 1
	if err := chain.ProcessPlanar(block); err != nil {
		panic(err)
	}

	fmt.Printf("Physical right channel, dry + right IR: %.2f\n", block[0][128])
	// Output: Physical right channel, dry + right IR: 1.50
}
