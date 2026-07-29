// Command graphiceq compares the all-FIR octave equaliser with the hybrid
// IIR/FIR design at equal latency.
//
// Each split is printed twice: once as the hybrid, and once as an all-FIR
// design forced to the same tap count. The pair is what shows whether moving
// bands into the recursive part buys anything the FIR could not have had for
// the same latency.
package main

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/filter/graphiceq"
)

const sampleRate = 48000

func main() {
	bands := octaveBands([]float64{6, -3, 0, 4, -6, 2, 0, -2, 5, 0})

	fmt.Println("method,iir_bands,taps,latency,rms_error_db,max_error_db")

	for split := range 5 {
		hybrid, err := graphiceq.Design(graphiceq.Config{
			SampleRate: sampleRate,
			Bands:      bands,
			IIRBands:   split,
		})
		if err != nil {
			panic(err)
		}

		printResult("hybrid", split, hybrid)

		if split == 0 {
			continue
		}

		reference, err := graphiceq.Design(graphiceq.Config{
			SampleRate: sampleRate,
			Bands:      bands,
			Length:     len(hybrid.Taps),
		})
		if err != nil {
			panic(err)
		}

		printResult("all-fir-equal-latency", 0, reference)
	}
}

func printResult(method string, split int, result graphiceq.Result) {
	fmt.Printf(
		"%s,%d,%d,%d,%.6f,%.6f\n",
		method,
		split,
		len(result.Taps),
		result.Latency,
		result.Metrics.RMSErrorDB,
		result.Metrics.MaxErrorDB,
	)
}

func octaveBands(gains []float64) []graphiceq.Band {
	bands := make([]graphiceq.Band, len(gains))
	frequency := 31.25

	for i, gain := range gains {
		bands[i] = graphiceq.Band{Frequency: frequency, GainDB: gain}
		frequency *= 2
	}

	return bands
}
