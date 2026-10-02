package resample

import (
	"math"
	"testing"
)

func TestGroupDelayUnitsAndImpulseTail(t *testing.T) {
	for _, ratio := range [][2]int{{1, 1}, {2, 1}, {1, 2}, {160, 147}, {147, 160}} {
		r := mustResampler(t, ratio[0], ratio[1])
		up, down := r.Ratio()

		center := float64(len(r.Prototype())-1) / 2
		if r.GroupDelayInput() != center/float64(up) || r.GroupDelayOutput() != center/float64(down) {
			t.Fatalf("ratio%v delay in=%g out=%g", ratio, r.GroupDelayInput(), r.GroupDelayOutput())
		}
	}

	r := mustResampler(t, 2, 1)
	impulse := r.Process([]float64{1})
	tail := r.Process(make([]float64, r.TapsPerPhase()))
	output := append(impulse, tail...)
	prototype := r.Prototype()
	checkOutputBits(t, output[:len(prototype)], prototype)

	var weight, moment float64
	for index, value := range output {
		weight += value
		moment += float64(index) * value
	}

	if math.Abs(moment/weight-r.GroupDelayOutput()) > 1e-12 {
		t.Fatalf("impulse centroid=%g reported delay=%g", moment/weight, r.GroupDelayOutput())
	}
}

func extremeRateAttenuation(t *testing.T, quality Quality, taps int) (float64, float64) {
	t.Helper()

	const (
		inputRate   = 384000
		inputLength = 98304
	)

	options := []Option{WithQuality(quality), WithTapsPerPhase(taps)}
	pass := mustResampler(t, 1, 48, options...)
	stop := pass.Clone()
	inputPass := sine(1000, inputRate, inputLength)
	inputStop := sine(6000, inputRate, inputLength)
	outputPass := pass.Process(inputPass)
	outputStop := stop.Process(inputStop)

	return math.Abs(dbRatio(rms(outputPass[256:]), rms(inputPass[12288:]))),
		-dbRatio(rms(outputStop[256:]), rms(inputStop[12288:]))
}

func TestExtremeDownsampleConfiguredQuality(t *testing.T) {
	baselinePass, baselineStop := extremeRateAttenuation(t, QualityBalanced, 32)
	t.Logf("384k->8k default balanced: pass droop%.3fdB stop attenuation%.3fdB", baselinePass, baselineStop)

	tests := []struct {
		quality                      Quality
		maxPassbandDB, minStopbandDB float64
	}{
		{QualityFast, 0.7, 55}, {QualityBalanced, 0.35, 75}, {QualityBest, 0.2, 90},
	}
	for _, test := range tests {
		taps := QualityProfile(test.quality).TapsPerPhase * 48
		pass, stop := extremeRateAttenuation(t, test.quality, taps)
		t.Logf("384k->8k quality%d taps%d: pass droop%.3fdB stop attenuation%.3fdB", test.quality, taps, pass, stop)

		if pass > test.maxPassbandDB || stop < test.minStopbandDB {
			t.Fatalf("quality%d violates configured pass/stop thresholds: %.3f/%.3fdB", test.quality, pass, stop)
		}
	}
}
