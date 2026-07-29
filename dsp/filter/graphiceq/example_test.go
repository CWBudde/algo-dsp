package graphiceq_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/filter/graphiceq"
)

func ExampleDesign() {
	bands := []graphiceq.Band{
		{Frequency: 62.5, GainDB: 6},
		{Frequency: 125, GainDB: 3},
		{Frequency: 250, GainDB: 0},
		{Frequency: 500, GainDB: -3},
		{Frequency: 1000, GainDB: 0},
	}

	cfg := graphiceq.Config{
		SampleRate: 48000,
		Bands:      bands,
		IIRBands:   2,
	}

	result, err := graphiceq.Design(cfg)
	if err != nil {
		panic(err)
	}

	fmt.Println(len(result.Sections))
	fmt.Println(result.Latency)
	fmt.Println(result.Metrics.MaxErrorDB < 1)

	// Output:
	// 2
	// 192
	// true
}

// ExampleDefaultLength shows where the latency saving comes from: each band
// moved into the IIR part doubles the lowest frequency the FIR still has to
// resolve, and therefore halves its length.
func ExampleDefaultLength() {
	cfg := graphiceq.Config{
		SampleRate: 48000,
		Bands: []graphiceq.Band{
			{Frequency: 62.5},
			{Frequency: 125},
			{Frequency: 250},
			{Frequency: 500},
		},
	}

	for split := range 3 {
		cfg.IIRBands = split
		fmt.Println(split, graphiceq.DefaultLength(cfg))
	}

	// Output:
	// 0 1537
	// 1 769
	// 2 385
}
