package pitch_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
)

// exampleSine builds one frame of a sine at the given frequency.
func exampleSine(freqHz, sampleRate, amplitude float64, n int) []float64 {
	out := make([]float64, n)

	step := 2 * math.Pi * freqHz / sampleRate
	for i := range out {
		out[i] = amplitude * math.Sin(step*float64(i))
	}

	return out
}

func ExampleYINDetector() {
	d, err := pitch.NewYINDetector(48000)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	frame := exampleSine(440, 48000, 0.5, d.FrameSize())

	est, err := d.Detect(frame)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Printf("%.1f Hz voiced=%v\n", est.FrequencyHz, est.Voiced)
	// Output: 440.0 Hz voiced=true
}

// ExampleYINDetector_missingFundamental shows YIN's defining property: it
// estimates the period rather than picking a spectral peak, so it recovers a
// fundamental that carries no energy of its own.
func ExampleYINDetector_missingFundamental() {
	d, err := pitch.NewYINDetector(48000)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// Harmonics 2 through 5 of 200 Hz, with nothing at 200 Hz itself.
	frame := make([]float64, d.FrameSize())
	for _, h := range []int{2, 3, 4, 5} {
		partial := exampleSine(200*float64(h), 48000, 0.25, len(frame))
		for i := range frame {
			frame[i] += partial[i]
		}
	}

	est, err := d.Detect(frame)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Printf("%.1f Hz\n", est.FrequencyHz)
	// Output: 200.0 Hz
}

func ExamplePitchTracker() {
	tracker, err := pitch.NewPitchTracker(48000)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// Feed several frames of a steady tone in small blocks, as a streaming
	// caller would.
	signal := exampleSine(220, 48000, 0.5, 4*tracker.Detector().FrameSize())
	for start := 0; start < len(signal); start += 256 {
		end := min(start+256, len(signal))
		tracker.Write(signal[start:end])
	}

	est := tracker.Estimate()

	fmt.Printf("%.1f Hz voiced=%v latency=%d samples\n",
		est.FrequencyHz, est.Voiced, tracker.Latency())
	// Output: 220.0 Hz voiced=true latency=1600 samples
}
