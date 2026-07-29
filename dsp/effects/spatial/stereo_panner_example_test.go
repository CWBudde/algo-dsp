package spatial_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects/spatial"
)

// ExampleStereoPanner_ProcessMono spreads a mono source across a stereo pair
// using the default equal-power law.
func ExampleStereoPanner_ProcessMono() {
	p, err := spatial.NewStereoPanner(48000, spatial.WithPanPosition(0.3))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	outL, outR := p.ProcessMono(1)

	fmt.Printf("L=%.4f R=%.4f power=%.4f\n", outL, outR, outL*outL+outR*outR)
	// Output:
	// L=0.5225 R=0.8526 power=1.0000
}

// ExampleStereoPanner_ProcessStereo rebalances an existing stereo pair. The
// balance mode never boosts: at the centre the pair passes through untouched,
// and panning right fades the left channel out.
func ExampleStereoPanner_ProcessStereo() {
	p, err := spatial.NewStereoPanner(48000)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	outL, outR := p.ProcessStereo(0.8, 0.2)
	fmt.Printf("centre:     L=%.4f R=%.4f\n", outL, outR)

	if err := p.SetPosition(0.5); err != nil {
		fmt.Println("error:", err)
		return
	}

	outL, outR = p.ProcessStereo(0.8, 0.2)
	fmt.Printf("half right: L=%.4f R=%.4f\n", outL, outR)

	if err := p.SetPosition(1); err != nil {
		fmt.Println("error:", err)
		return
	}

	outL, outR = p.ProcessStereo(0.8, 0.2)
	fmt.Printf("hard right: L=%.4f R=%.4f\n", outL, outR)
	// Output:
	// centre:     L=0.8000 R=0.2000
	// half right: L=0.5657 R=0.2000
	// hard right: L=0.0000 R=0.2000
}

// ExampleStereoPanner_panLaw compares the centre level of the three pan laws.
func ExampleStereoPanner_panLaw() {
	laws := []struct {
		name string
		law  spatial.PanLaw
	}{
		{"equal power", spatial.PanLawEqualPower},
		{"compromise ", spatial.PanLawCompromise},
		{"linear     ", spatial.PanLawLinear},
	}

	for _, l := range laws {
		p, err := spatial.NewStereoPanner(48000, spatial.WithPanLaw(l.law))
		if err != nil {
			fmt.Println("error:", err)
			return
		}

		gainL, gainR := p.Gains()
		fmt.Printf("%s centre gain L=%.4f R=%.4f\n", l.name, gainL, gainR)
	}
	// Output:
	// equal power centre gain L=0.7071 R=0.7071
	// compromise  centre gain L=0.5946 R=0.5946
	// linear      centre gain L=0.5000 R=0.5000
}

// ExampleStereoPanner_autoPan sweeps the pan position with the built-in LFO.
// A 1 Hz LFO at 48 kHz reaches hard right a quarter of a period (12000
// samples) after the centre.
func ExampleStereoPanner_autoPan() {
	p, err := spatial.NewStereoPanner(
		48000,
		spatial.WithAutoPanRate(1),
		spatial.WithAutoPanDepth(1),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	gainL, gainR := p.Gains()
	fmt.Printf("start:   L=%.4f R=%.4f\n", gainL, gainR)

	const quarterPeriod = 12000
	for range quarterPeriod {
		p.ProcessMono(1)
	}

	gainL, gainR = p.Gains()
	fmt.Printf("quarter: L=%.4f R=%.4f\n", gainL, gainR)
	// Output:
	// start:   L=0.7071 R=0.7071
	// quarter: L=0.0000 R=1.0000
}
