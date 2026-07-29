// Command mixedphase compares the fixed-length mixed-phase FIR designs.
package main

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/mixedphase"
)

func main() {
	const length = 129

	prototype := lowpassPrototype(length, 0.08)
	maxDelay := (length - 1) / 2

	fmt.Println(
		"method,delay,relative_magnitude_error,rms_error_db," +
			"peak_index,energy_centroid,pre_peak_energy," +
			"complex_rms_error,complex_peak_error",
	)

	for _, delay := range []int{0, 8, 16, 32, 64} {
		mix := float64(delay) / float64(maxDelay)

		iterative, err := mixedphase.DesignIterative(
			prototype,
			mixedphase.IterativeConfig{
				Length:     length,
				Delay:      delay,
				Iterations: 12,
			},
		)
		if err != nil {
			panic(err)
		}

		printResult("budde-iterative", delay, iterative)

		direct, err := mixedphase.DesignPhaseInterpolation(
			prototype,
			mixedphase.PhaseInterpolationConfig{
				Length: length,
				Mix:    mix,
			},
		)
		if err != nil {
			panic(err)
		}

		printResult("phase-interpolation", delay, direct)

		// The unweighted least-squares design coincides with phase
		// interpolation, so only the minimax path is worth a separate row.
		minimax, err := mixedphase.DesignComplexLeastSquares(
			prototype,
			mixedphase.ComplexLeastSquaresConfig{
				Length:            length,
				Mix:               mix,
				MinimaxIterations: 16,
			},
		)
		if err != nil {
			panic(err)
		}

		printResult("complex-minimax", delay, minimax)
	}
}

func printResult(method string, delay int, result mixedphase.Result) {
	metrics := result.Metrics

	// The complex-error columns stay empty for the designs that do not
	// prescribe a phase over the whole grid, so a zero is never mistaken for a
	// measurement.
	complexRMS, complexPeak := "", ""
	if result.ComplexError.Peak > 0 {
		complexRMS = fmt.Sprintf("%.9g", result.ComplexError.RMS)
		complexPeak = fmt.Sprintf("%.9g", result.ComplexError.Peak)
	}

	fmt.Printf(
		"%s,%d,%.9g,%.6f,%d,%.6f,%.9g,%s,%s\n",
		method,
		delay,
		metrics.RelativeMagnitudeError,
		metrics.RMSMagnitudeErrorDB,
		metrics.PeakIndex,
		metrics.EnergyCentroid,
		metrics.PrePeakEnergyRatio,
		complexRMS,
		complexPeak,
	)
}

func lowpassPrototype(length int, cutoff float64) []float64 {
	taps := make([]float64, length)
	middle := float64(length-1) / 2
	sum := 0.0

	for i := range taps {
		x := float64(i) - middle

		sinc := 2 * cutoff
		if x != 0 {
			sinc = math.Sin(2*math.Pi*cutoff*x) / (math.Pi * x)
		}

		windowValue := 0.5 -
			0.5*math.Cos(2*math.Pi*float64(i)/float64(length-1))
		taps[i] = sinc * windowValue
		sum += taps[i]
	}

	for i := range taps {
		taps[i] /= sum
	}

	return taps
}
