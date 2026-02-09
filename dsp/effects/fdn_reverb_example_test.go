package effects

import "fmt"

func ExampleFDNReverb() {
	r, err := NewFDNReverb(48000)
	if err != nil {
		panic(err)
	}
	_ = r.SetPreDelay(0.015)
	_ = r.SetRT60(2.5)
	_ = r.SetDamp(0.4)
	_ = r.SetModDepth(0.002)
	_ = r.SetModRate(0.2)

	buf := []float64{1, 0, 0, 0, 0}
	r.ProcessInPlace(buf)
	fmt.Printf("%.3f %.3f %.3f %.3f %.3f", buf[0], buf[1], buf[2], buf[3], buf[4])
	// Output: 1.000 0.000 0.000 0.000 0.000
}
