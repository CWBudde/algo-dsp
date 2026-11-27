package reverb

import "fmt"

func ExampleDAVFreeverb32() {
	// Reserve the largest delay lengths each stage may ever use, one
	// left/right pair per comb and allpass stage.
	capacity := [][2]int{{8, 8}}

	r, err := NewDAVFreeverb32(capacity, capacity)
	if err != nil {
		panic(err)
	}

	// Choose the active geometry, then configure the active stages.
	if err := r.SetGeometry([][2]int{{3, 5}}, [][2]int{{2, 4}}); err != nil {
		panic(err)
	}

	if err := r.SetCombFeedback(0.5, 0.25); err != nil {
		panic(err)
	}

	if err := r.SetAllpassFeedback([][2]float32{{0.5, 0.75}}); err != nil {
		panic(err)
	}

	for frame := range 8 {
		var input float32
		if frame == 0 {
			input = 1
		}

		left, right := r.ProcessSample(input)
		fmt.Printf("%d: %.5f %.5f\n", frame, left, right)
	}
	// Output:
	// 0: 0.00000 0.00000
	// 1: 0.00000 0.00000
	// 2: 0.00000 0.00000
	// 3: -1.00000 0.00000
	// 4: 0.00000 0.00000
	// 5: 1.50000 -1.00000
	// 6: -0.37500 0.00000
	// 7: -0.09375 0.00000
}
