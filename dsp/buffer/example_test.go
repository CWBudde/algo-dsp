package buffer_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/buffer"
)

func ExampleBuffer() {
	b := buffer.New(4)
	copy(b.Samples(), []float64{1, 2, 3, 4})

	b.Resize(6)
	b.ZeroRange(1, 5)

	fmt.Println(b.Samples())
	fmt.Println(b.Len(), b.Cap())

	// Output:
	// [1 0 0 0 0 0]
	// 6 8
}
