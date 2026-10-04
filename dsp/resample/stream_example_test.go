package resample_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/resample"
)

func ExampleStream() {
	plan, _ := resample.NewStreamPlan(44100, 48000, 64, resample.QualityBalanced)
	workspace, _ := plan.WorkspaceBytes(2)
	fmt.Println(workspace > 0)

	stream, _ := plan.NewStream(3)
	output := make([]float64, plan.OutputBlockFrames())

	_, _ = stream.ProcessInto(output, []float64{.2, .4, .1})
	for !stream.Done() {
		_, _, _ = stream.FlushInto(output)
	}

	fmt.Println(stream.InputFrames(), stream.OutputFrames())
	stream.Reset()
	fmt.Println(stream.InputFrames())

	count, _ := resample.FrameCount(3, 44100, 48000)
	position, _ := resample.FramePosition(3, 44100, 48000)
	fmt.Println(count, position)
	// Output:
	// true
	// 3 4
	// 0
	// 4 3
}
