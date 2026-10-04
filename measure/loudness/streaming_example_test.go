package loudness_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/measure/loudness"
)

func ExampleNewStreamingMeter() {
	meter, _ := loudness.NewStreamingMeter(loudness.IntegratedConfig{SampleRate: 48000, Channels: 2, MaxFrames: 48000 * 4})
	block := make([]float32, 960)

	for frame := 0; frame < 480; frame++ {
		value := float32(math.Pow(10, -23.0/20) * math.Sin(2*math.Pi*1000*float64(frame)/48000))
		block[2*frame], block[2*frame+1] = value, value
	}

	for range 400 {
		if err := meter.ProcessInterleaved32(block); err != nil {
			panic(err)
		}
	}

	r := meter.Snapshot()
	fmt.Printf("M %.1f, S %.1f, I %.1f LUFS\n", r.Momentary, r.ShortTerm, r.Integrated)
	// Output: M -23.0, S -23.0, I -23.0 LUFS
}
