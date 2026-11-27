package delay_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/delay"
)

func ExampleDAVVibrato32() {
	// Reserve the widest span up front: depth 1 at 48 kHz needs 12000 samples.
	line, err := delay.NewDAVVibrato32(12000)
	if err != nil {
		fmt.Println(err)
		return
	}

	// 48 kHz host rate, depth 0.05 (a 30-sample span), 0.2 Hz LFO.
	if err := line.Configure(48000, .05, .2); err != nil {
		fmt.Println(err)
		return
	}

	// Send an impulse once the line has run through its span.
	const impulse = 48

	peak, at := float32(0), 0

	for frame := range 2 * impulse {
		var input float32
		if frame == impulse {
			input = 1
		}

		if output := line.ProcessSample(input); output > peak {
			peak, at = output, frame-impulse
		}
	}

	fmt.Printf("impulse delayed by %d samples, peak %.2f\n", at, peak)

	// Retuning within the reserved span keeps the delay history and LFO phase.
	if err := line.Configure(48000, .2, 2); err != nil {
		fmt.Println(err)
	}
	// Output:
	// impulse delayed by 18 samples, peak 1.00
}
