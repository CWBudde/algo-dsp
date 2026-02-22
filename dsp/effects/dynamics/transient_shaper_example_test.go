package dynamics_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects/dynamics"
)

func ExampleTransientShaper_configuration() {
	ts, err := dynamics.NewTransientShaper(48000)
	if err != nil {
		panic(err)
	}

	_ = ts.SetAttackAmount(0.6)
	_ = ts.SetSustainAmount(-0.3)
	_ = ts.SetAttack(8)
	_ = ts.SetRelease(120)

	buf := []float64{0.0, 0.9, 0.4, 0.2, 0.1, 0.0}
	ts.ProcessInPlace(buf)

	fmt.Printf("attack=%.1f sustain=%.1f\n", ts.AttackAmount(), ts.SustainAmount())
	// Output:
	// attack=0.6 sustain=-0.3
}
