package features_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/measure/music/features"
)

// activityLevels returns two tracks of 4 s at 100 frames/s: a loud drum
// track playing throughout and a quiet pad that drops out in the third
// second.
func activityLevels() map[string][]float64 {
	drums := make([]float64, 400)
	pad := make([]float64, 400)

	for i := range drums {
		drums[i] = 0.4

		if i < 200 || i >= 300 {
			pad[i] = 0.01
		}
	}

	return map[string][]float64{"drums": drums, "pad": pad}
}

// oneSecondSpans returns four one-second spans.
func oneSecondSpans() []features.Interval {
	return []features.Interval{{Start: 0, End: 1}, {Start: 1, End: 2}, {Start: 2, End: 3}, {Start: 3, End: 4}}
}

func ExampleActivity() {
	act, err := features.Activity(activityLevels(), 100, oneSecondSpans())
	if err != nil {
		panic(err)
	}

	// The pad is 32 dB quieter than the drums but active wherever it plays.
	fmt.Println("drums", act["drums"].Active)
	fmt.Println("pad  ", act["pad"].Active)
	// Output:
	// drums [true true true true]
	// pad   [true true false true]
}

func ExampleTrackActivity() {
	act, err := features.Activity(activityLevels(), 100, oneSecondSpans())
	if err != nil {
		panic(err)
	}

	pad := act["pad"]
	fmt.Printf("reference %.1f dB, spans %.1f dB\n", pad.ReferenceDB, pad.SpanDB)
	// Output:
	// reference -40.0 dB, spans [-40.0 -40.0 -120.0 -40.0] dB
}

func ExampleActivityOption() {
	// The defaults, spelled out: -12 dB below the 95th percentile of the
	// frames above -80 dBFS.
	opts := []features.ActivityOption{
		features.WithActivityThreshold(features.DefaultActivityThresholdDB),
		features.WithActivityGate(features.DefaultGate),
		features.WithActivityPercentile(features.DefaultActivityPercentile),
	}

	act, err := features.Activity(activityLevels(), 100, oneSecondSpans(), opts...)
	if err != nil {
		panic(err)
	}

	fmt.Println(act["pad"].Active)
	// Output:
	// [true true false true]
}

func ExampleWithActivityThreshold() {
	// A span half a second into a one-second pause has half the power,
	// -3 dB: active at the default -12 dB, inactive at -2 dB.
	spans := []features.Interval{{Start: 1.5, End: 2.5}}

	for _, db := range []float64{-12, -2} {
		act, err := features.Activity(activityLevels(), 100, spans, features.WithActivityThreshold(db))
		if err != nil {
			panic(err)
		}

		fmt.Printf("%v dB: %v\n", db, act["pad"].Active[0])
	}
	// Output:
	// -12 dB: true
	// -2 dB: false
}

func ExampleWithActivityGate() {
	// A track at -90 dBFS is below the default -80 dBFS gate and therefore
	// never active; a -100 dBFS gate lets it count.
	levels := map[string][]float64{"hiss": {3e-5, 3e-5, 3e-5, 3e-5}}
	spans := []features.Interval{{Start: 0, End: 0.04}}

	for _, gate := range []float64{features.DefaultGate, 1e-5} {
		act, err := features.Activity(levels, 100, spans, features.WithActivityGate(gate))
		if err != nil {
			panic(err)
		}

		fmt.Printf("gate %g: %v\n", gate, act["hiss"].Active[0])
	}
	// Output:
	// gate 0.0001: false
	// gate 1e-05: true
}

func ExampleWithActivityPercentile() {
	// With p = 1 the reference is the loudest frame.
	levels := map[string][]float64{"x": {0.1, 0.1, 0.1, 1}}

	act, err := features.Activity(levels, 100, []features.Interval{{Start: 0, End: 0.04}},
		features.WithActivityPercentile(1))
	if err != nil {
		panic(err)
	}

	fmt.Printf("reference %.0f dB\n", act["x"].ReferenceDB)
	// Output:
	// reference 0 dB
}
