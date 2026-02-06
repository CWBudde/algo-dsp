package bank

import (
	"math"
	"sort"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
)

var octaveRatio = math.Pow(10, 0.3)

const defaultOrder = 4
const defaultLowerFreq = 20.0
const defaultUpperFreq = 20000.0

type Band struct {
	CenterFreq float64
	LowCutoff  float64
	HighCutoff float64
	LP         *biquad.Chain
	HP         *biquad.Chain
}

func (b *Band) MagnitudeDB(freqHz, sampleRate float64) float64 {
	return b.LP.MagnitudeDB(freqHz, sampleRate) + b.HP.MagnitudeDB(freqHz, sampleRate)
}

type Bank struct {
	bands      []Band
	sampleRate float64
	order      int
}

type bankConfig struct {
	order   int
	lowerHz float64
	upperHz float64
}

type Option func(*bankConfig)
