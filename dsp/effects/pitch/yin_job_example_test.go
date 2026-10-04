package pitch_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
)

func ExampleNewYINJob() {
	d, _ := pitch.NewYINDetector(48000)
	job, _ := pitch.NewYINJob(d)

	frame := make([]float64, d.FrameSize())
	for i := range frame {
		frame[i] = .3 * math.Sin(2*math.Pi*440*float64(i)/48000)
	}

	if err := job.Begin(frame); err != nil {
		panic(err)
	}

	for {
		done, err := job.Step(4096)
		if err != nil {
			panic(err)
		}

		if done {
			break
		}
	}

	estimate, _ := job.Result()
	fmt.Printf("%.0f Hz\n", estimate.FrequencyHz)
	// Output: 440 Hz
}
