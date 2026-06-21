package effects_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects"
)

func ExampleSpectralFreeze_ProcessInPlace() {
	freeze, err := effects.NewSpectralFreeze(48000)
	if err != nil {
		fmt.Println("error")
		return
	}

	_ = freeze.SetFrameSize(256)
	_ = freeze.SetHopSize(64)
	_ = freeze.SetMix(0.8)
	_ = freeze.SetPhaseMode(effects.SpectralFreezePhaseAdvance)
	freeze.Freeze()

	buf := make([]float64, 1024)
	for i := range 128 {
		buf[i] = math.Sin(2 * math.Pi * 440 * float64(i) / 48000)
	}

	freeze.ProcessInPlace(buf)
	fmt.Printf("len=%d frozen=%t\n", len(buf), freeze.Frozen())
	// Output:
	// len=1024 frozen=true
}
