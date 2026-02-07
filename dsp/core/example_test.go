package core_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/core"
)

func ExampleApplyProcessorOptions() {
	cfg := core.ApplyProcessorOptions(
		core.WithSampleRate(44100),
		core.WithBlockSize(256),
	)

	fmt.Printf("sampleRate=%.0f blockSize=%d\n", cfg.SampleRate, cfg.BlockSize)

	// Output:
	// sampleRate=44100 blockSize=256
}

func ExampleEnsureLen() {
	buf := make([]float64, 2, 4)
	buf[0], buf[1] = 1, 2
	buf = core.EnsureLen(buf, 4)

	copied := core.CopyInto(buf[2:], []float64{3, 4})
	fmt.Println(copied, buf)

	core.Zero(buf[:2])
	fmt.Println(buf)

	// Output:
	// 2 [1 2 3 4]
	// [0 0 3 4]
}
