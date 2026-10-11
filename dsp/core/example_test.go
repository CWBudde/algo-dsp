package core_test

import (
	"errors"
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

func ExampleLinearToDBFloor() {
	fmt.Printf("%.1f\n", core.LinearToDBFloor(0.5, 1e-6))
	fmt.Printf("%.1f\n", core.LinearToDBFloor(0, 1e-6))
	fmt.Println(core.LinearToDB(0))

	// Output:
	// -6.0
	// -120.0
	// -Inf
}

func ExampleLinearPowerToDBFloor() {
	fmt.Printf("%.1f\n", core.LinearPowerToDBFloor(0, 1e-12))

	// Output:
	// -120.0
}

func ExamplePadReflect() {
	x := []float64{1, 2, 3, 4}
	dst := make([]float64, len(x)+4)

	if err := core.PadReflect(dst, x, 2, 2); err != nil {
		fmt.Println(err)
	}

	fmt.Println(dst)

	// A reflect pad must be shorter than the input.
	err := core.PadReflect(make([]float64, 9), x, 4, 1)
	fmt.Println(err)
	fmt.Println(errors.Is(err, core.ErrPadTooLong))

	// Output:
	// [3 2 1 2 3 4 3 2]
	// core: reflect pad must be shorter than the input: left 4, right 1 for 4 samples
	// true
}

func ExampleOverlaps() {
	buf := make([]float64, 4, 8)

	fmt.Println(core.Overlaps(buf[:3], buf[2:]))
	fmt.Println(core.Overlaps(buf[:2], buf[2:4]))
	fmt.Println(core.Overlaps(buf[:4], buf[4:8])) // shares only capacity
	fmt.Println(core.Overlaps(buf, nil))

	// Output:
	// true
	// false
	// false
	// false
}
