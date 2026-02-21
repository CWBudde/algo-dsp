package spatial_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects/spatial"
)

func ExampleStereoWidener_ProcessStereo() {
	w, err := spatial.NewStereoWidener(48000, spatial.WithWidth(2.0))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	left, right := 0.8, 0.2
	outL, outR := w.ProcessStereo(left, right)

	fmt.Printf("L=%.4f R=%.4f\n", outL, outR)
	// Output:
	// L=1.1000 R=-0.1000
}

func ExampleStereoWidener_ProcessStereoInPlace() {
	w, err := spatial.NewStereoWidener(48000, spatial.WithWidth(0.0))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	left := []float64{1, 0, -1, 0}
	right := []float64{0, 1, 0, -1}
	if err := w.ProcessStereoInPlace(left, right); err != nil {
		fmt.Println("error:", err)
		return
	}

	for i := range left {
		fmt.Printf("[%d] L=%.4f R=%.4f\n", i, left[i], right[i])
	}
	// Output:
	// [0] L=0.5000 R=0.5000
	// [1] L=0.5000 R=0.5000
	// [2] L=-0.5000 R=-0.5000
	// [3] L=-0.5000 R=-0.5000
}

func ExampleStereoWidener_ProcessInterleavedInPlace() {
	w, err := spatial.NewStereoWidener(48000, spatial.WithWidth(1.5))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// Interleaved: L, R, L, R, ...
	buf := []float64{0.8, 0.2, 0.6, 0.4}
	if err := w.ProcessInterleavedInPlace(buf); err != nil {
		fmt.Println("error:", err)
		return
	}

	for i := 0; i < len(buf); i += 2 {
		fmt.Printf("L=%.4f R=%.4f\n", buf[i], buf[i+1])
	}
	// Output:
	// L=0.9500 R=0.0500
	// L=0.6500 R=0.3500
}

func ExampleStereoWidener_bassMono() {
	w, err := spatial.NewStereoWidener(48000,
		spatial.WithWidth(2.0),
		spatial.WithBassMonoFreq(120),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// Process a stereo signal with bass mono crossover active.
	const n = 4800
	for i := 0; i < n; i++ {
		phase := 2 * math.Pi * 1000 * float64(i) / 48000
		l := math.Sin(phase)
		r := math.Sin(phase + 0.3)
		_, _ = w.ProcessStereo(l, r)
	}

	fmt.Printf("bass_mono_freq=%.0f width=%.1f\n", w.BassMonoFreq(), w.Width())
	// Output:
	// bass_mono_freq=120 width=2.0
}
