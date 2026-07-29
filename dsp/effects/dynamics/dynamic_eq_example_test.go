package dynamics_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects/dynamics"
)

// sine renders n samples of a sine at freqHz with the given amplitude.
func sine(freqHz, amp float64, n int, sampleRate float64) []float64 {
	buf := make([]float64, n)
	for i := range buf {
		buf[i] = amp * math.Sin(2*math.Pi*freqHz*float64(i)/sampleRate)
	}

	return buf
}

func ExampleNewDynamicEQWithConfig() {
	// A de-essing band that only cuts when 6 kHz content gets loud, plus a
	// static low shelf that is always applied.
	eq, err := dynamics.NewDynamicEQWithConfig(48000, []dynamics.EQBandConfig{
		{
			FrequencyHz: 6000,
			Q:           2,
			Mode:        dynamics.EQBandModeDownward,
			ThresholdDB: -24,
			Ratio:       4,
			AttackMs:    2,
			ReleaseMs:   80,
			RangeDB:     12,
		},
		{
			Type:         dynamics.EQBandLowShelf,
			FrequencyHz:  120,
			StaticGainDB: -3,
			Mode:         dynamics.EQBandModeStatic,
		},
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Printf("bands: %d\n", eq.NumBands())
	fmt.Printf("update interval: %d samples\n", eq.UpdateInterval())

	// Output:
	// bands: 2
	// update interval: 32 samples
}

func ExampleDynamicEQ_ProcessInPlace() {
	eq, err := dynamics.NewDynamicEQWithConfig(48000, []dynamics.EQBandConfig{{
		FrequencyHz: 1000,
		Q:           2,
		Mode:        dynamics.EQBandModeDownward,
		ThresholdDB: -20,
		Ratio:       4,
		AttackMs:    1,
		ReleaseMs:   50,
		RangeDB:     24,
	}})
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// A loud 1 kHz tone drives the band into gain reduction.
	buf := sine(1000, 0.5, 24000, 48000)
	eq.ProcessInPlace(buf)

	gainDB, err := eq.BandGainDB(0)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Printf("band gain: %.2f dB\n", gainDB)

	// Output:
	// band gain: -10.17 dB
}

func ExampleDynamicEQ_ProcessSampleSidechain() {
	eq, err := dynamics.NewDynamicEQWithConfig(48000, []dynamics.EQBandConfig{{
		FrequencyHz: 1000,
		Q:           2,
		Mode:        dynamics.EQBandModeDownward,
		ThresholdDB: -30,
		Ratio:       6,
		AttackMs:    1,
		ReleaseMs:   50,
		RangeDB:     24,
	}})
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// The program is quiet, but a loud external sidechain ducks the band.
	program := sine(1000, 0.02, 12000, 48000)
	sidechain := sine(1000, 0.8, 12000, 48000)

	if err := eq.ProcessInPlaceSidechain(program, sidechain); err != nil {
		fmt.Println("error:", err)
		return
	}

	gainDB, err := eq.BandGainDB(0)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Printf("sidechain-driven gain: %.2f dB\n", gainDB)

	// Output:
	// sidechain-driven gain: -23.04 dB
}

func ExampleDynamicEQ_upwardBelow() {
	// Upward expansion: the band boosts as its region falls below the threshold.
	eq, err := dynamics.NewDynamicEQWithConfig(48000, []dynamics.EQBandConfig{{
		FrequencyHz: 200,
		Q:           1.5,
		Mode:        dynamics.EQBandModeUpwardBelow,
		ThresholdDB: -20,
		Ratio:       2,
		KneeDB:      dynamics.Float64Ptr(0),
		AttackMs:    1,
		ReleaseMs:   50,
		RangeDB:     12,
	}})
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	buf := sine(200, 0.01, 24000, 48000) // -40 dBFS, 20 dB under threshold
	eq.ProcessInPlace(buf)

	gainDB, err := eq.BandGainDB(0)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Printf("band gain: %+.2f dB\n", gainDB)

	// Output:
	// band gain: +10.19 dB
}

func ExampleDynamicEQ_BandStaticCurve() {
	eq, err := dynamics.NewDynamicEQWithConfig(48000, []dynamics.EQBandConfig{{
		FrequencyHz: 1000,
		Mode:        dynamics.EQBandModeDownward,
		ThresholdDB: -20,
		Ratio:       4,
		KneeDB:      dynamics.Float64Ptr(0),
		RangeDB:     24,
	}})
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	points, err := eq.BandStaticCurve(0, -40, 0, 10)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	for _, p := range points {
		fmt.Printf("%.1f dB in -> %.4f dB gain\n", p.InputDB, p.GainReductionDB)
	}

	// Output:
	// -40.0 dB in -> 0.0000 dB gain
	// -30.0 dB in -> 0.0000 dB gain
	// -20.0 dB in -> 0.0000 dB gain
	// -10.0 dB in -> -7.5000 dB gain
	// 0.0 dB in -> -15.0000 dB gain
}
