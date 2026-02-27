package dynamics_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects/dynamics"
)

// ExampleNewMultibandCompressor demonstrates creating a basic 3-band
// multiband compressor with Linkwitz-Riley crossovers.
func ExampleNewMultibandCompressor() {
	// Create a 3-band compressor: [0–500 Hz], [500–5000 Hz], [5000+ Hz]
	// with LR4 (4th-order Linkwitz-Riley) crossovers at 48 kHz
	mc, err := dynamics.NewMultibandCompressor(
		[]float64{500, 5000}, // Crossover frequencies
		4,                    // LR4 order
		48000,                // Sample rate
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Bands: %d\n", mc.NumBands())
	fmt.Printf("Crossover order: LR%d\n", mc.CrossoverOrder())
	fmt.Printf("Crossover frequencies: %v Hz\n", mc.CrossoverFreqs())
	// Output:
	// Bands: 3
	// Crossover order: LR4
	// Crossover frequencies: [500 5000] Hz
}

// ExampleNewMultibandCompressorWithConfig demonstrates creating a multiband
// compressor with per-band configuration.
func ExampleNewMultibandCompressorWithConfig() {
	autoTrue := true
	configs := []dynamics.BandConfig{
		{ThresholdDB: dynamics.Float64Ptr(-24), Ratio: 3.0, KneeDB: dynamics.Float64Ptr(8.0), AttackMs: 20, ReleaseMs: 200, AutoMakeup: &autoTrue},
		{ThresholdDB: dynamics.Float64Ptr(-18), Ratio: 4.0, KneeDB: dynamics.Float64Ptr(6.0), AttackMs: 10, ReleaseMs: 100, AutoMakeup: &autoTrue},
		{ThresholdDB: dynamics.Float64Ptr(-12), Ratio: 2.0, KneeDB: dynamics.Float64Ptr(4.0), AttackMs: 5, ReleaseMs: 80, AutoMakeup: &autoTrue},
	}

	mc, err := dynamics.NewMultibandCompressorWithConfig(
		[]float64{500, 5000}, 4, 48000, configs,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Band 0 (low):  threshold=%.0f dB, ratio=%.0f:1\n",
		mc.Band(0).Threshold(), mc.Band(0).Ratio())
	fmt.Printf("Band 1 (mid):  threshold=%.0f dB, ratio=%.0f:1\n",
		mc.Band(1).Threshold(), mc.Band(1).Ratio())
	fmt.Printf("Band 2 (high): threshold=%.0f dB, ratio=%.0f:1\n",
		mc.Band(2).Threshold(), mc.Band(2).Ratio())
	// Output:
	// Band 0 (low):  threshold=-24 dB, ratio=3:1
	// Band 1 (mid):  threshold=-18 dB, ratio=4:1
	// Band 2 (high): threshold=-12 dB, ratio=2:1
}

// ExampleMultibandCompressor_ProcessSample demonstrates sample-by-sample
// processing with a multiband compressor.
func ExampleMultibandCompressor_ProcessSample() {
	mc, _ := dynamics.NewMultibandCompressor([]float64{1000}, 4, 48000)

	// Process a single sample
	_ = mc.ProcessSample(0.5)

	fmt.Println("Multiband compressor processed one sample")
	// Output:
	// Multiband compressor processed one sample
}

// ExampleMultibandCompressor_ProcessInPlace demonstrates block processing
// with a multiband compressor.
func ExampleMultibandCompressor_ProcessInPlace() {
	mc, _ := dynamics.NewMultibandCompressor([]float64{500, 5000}, 4, 48000)

	// Generate a test signal (sine at 1 kHz)
	buf := make([]float64, 512)
	for i := range buf {
		buf[i] = 0.4 * math.Sin(2*math.Pi*1000*float64(i)/48000)
	}

	mc.ProcessInPlace(buf)
	fmt.Println("Block processed successfully")
	// Output:
	// Block processed successfully
}

// ExampleMultibandCompressor_metering demonstrates using per-band metering.
func ExampleMultibandCompressor_metering() {
	mc, _ := dynamics.NewMultibandCompressor([]float64{1000}, 4, 48000)
	mc.ResetMetrics()

	// Process some signal
	for i := range 1000 {
		mc.ProcessSample(0.5 * math.Sin(2*math.Pi*440*float64(i)/48000))
	}

	metrics := mc.GetMetrics()
	if len(metrics.Bands) > 0 {
		fmt.Println("Multiband metrics collected")
	}
	// Output:
	// Multiband metrics collected
}

// ExampleMultibandCompressor_feedbackRMS demonstrates enabling feedback/RMS
// mode per band using the shared dynamics core controls.
func ExampleMultibandCompressor_feedbackRMS() {
	mc, _ := dynamics.NewMultibandCompressor([]float64{250, 2500}, 4, 48000)

	_ = mc.SetAllBandsTopology(dynamics.DynamicsTopologyFeedback)
	_ = mc.SetAllBandsDetectorMode(dynamics.DetectorModeRMS)
	_ = mc.SetAllBandsRMSWindow(20)
	_ = mc.SetAllBandsFeedbackRatioScale(true)

	fmt.Printf("Band0 topology: %v\n", mc.Band(0).Topology())
	fmt.Printf("Band0 detector: %v\n", mc.Band(0).DetectorMode())
	// Output:
	// Band0 topology: 1
	// Band0 detector: 1
}
