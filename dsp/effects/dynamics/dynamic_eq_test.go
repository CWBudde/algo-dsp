package dynamics

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/design"
)

const eqTestSampleRate = 48000.0

// sineBuffer renders n samples of a unit-amplitude sine at freqHz scaled by amp.
func sineBuffer(freqHz, amp float64, n int, sampleRate float64) []float64 {
	buf := make([]float64, n)
	w := 2 * math.Pi * freqHz / sampleRate

	for i := range buf {
		buf[i] = amp * math.Sin(w*float64(i))
	}

	return buf
}

// rms returns the RMS of the last fraction of buf, skipping the transient.
func rmsTail(buf []float64, skip int) float64 {
	if skip >= len(buf) {
		return 0
	}

	sum := 0.0
	for _, v := range buf[skip:] {
		sum += v * v
	}

	return math.Sqrt(sum / float64(len(buf)-skip))
}

func newTestEQ(t *testing.T, configs ...EQBandConfig) *DynamicEQ {
	t.Helper()

	eq, err := NewDynamicEQWithConfig(eqTestSampleRate, configs)
	if err != nil {
		t.Fatalf("NewDynamicEQWithConfig: %v", err)
	}

	return eq
}

func TestNewDynamicEQValidation(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate float64
		cfg        EQBandConfig
		wantErr    bool
	}{
		{
			name:       "valid peak band",
			sampleRate: eqTestSampleRate,
			cfg:        EQBandConfig{FrequencyHz: 1000},
		},
		{
			name:       "zero sample rate",
			sampleRate: 0,
			cfg:        EQBandConfig{FrequencyHz: 1000},
			wantErr:    true,
		},
		{
			name:       "missing frequency",
			sampleRate: eqTestSampleRate,
			cfg:        EQBandConfig{},
			wantErr:    true,
		},
		{
			name:       "frequency at nyquist",
			sampleRate: eqTestSampleRate,
			cfg:        EQBandConfig{FrequencyHz: eqTestSampleRate / 2},
			wantErr:    true,
		},
		{
			name:       "q too low",
			sampleRate: eqTestSampleRate,
			cfg:        EQBandConfig{FrequencyHz: 1000, Q: 0.001},
			wantErr:    true,
		},
		{
			name:       "q too high",
			sampleRate: eqTestSampleRate,
			cfg:        EQBandConfig{FrequencyHz: 1000, Q: 1000},
			wantErr:    true,
		},
		{
			name:       "ratio below one",
			sampleRate: eqTestSampleRate,
			cfg:        EQBandConfig{FrequencyHz: 1000, Ratio: 0.5},
			wantErr:    true,
		},
		{
			name:       "knee too wide",
			sampleRate: eqTestSampleRate,
			cfg:        EQBandConfig{FrequencyHz: 1000, KneeDB: Float64Ptr(100)},
			wantErr:    true,
		},
		{
			name:       "attack too short",
			sampleRate: eqTestSampleRate,
			cfg:        EQBandConfig{FrequencyHz: 1000, AttackMs: 0.001},
			wantErr:    true,
		},
		{
			name:       "release too long",
			sampleRate: eqTestSampleRate,
			cfg:        EQBandConfig{FrequencyHz: 1000, ReleaseMs: 100000},
			wantErr:    true,
		},
		{
			name:       "range too wide",
			sampleRate: eqTestSampleRate,
			cfg:        EQBandConfig{FrequencyHz: 1000, RangeDB: 200},
			wantErr:    true,
		},
		{
			name:       "static gain too large",
			sampleRate: eqTestSampleRate,
			cfg:        EQBandConfig{FrequencyHz: 1000, StaticGainDB: 100},
			wantErr:    true,
		},
		{
			name:       "invalid band type",
			sampleRate: eqTestSampleRate,
			cfg:        EQBandConfig{FrequencyHz: 1000, Type: EQBandType(99)},
			wantErr:    true,
		},
		{
			name:       "invalid band mode",
			sampleRate: eqTestSampleRate,
			cfg:        EQBandConfig{FrequencyHz: 1000, Mode: EQBandMode(99)},
			wantErr:    true,
		},
		{
			name:       "invalid detector source",
			sampleRate: eqTestSampleRate,
			cfg:        EQBandConfig{FrequencyHz: 1000, DetectorSource: EQDetectorSource(99)},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewDynamicEQWithConfig(tt.sampleRate, []EQBandConfig{tt.cfg})
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewDynamicEQWithConfig error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestDynamicEQBandLimits(t *testing.T) {
	eq, err := NewDynamicEQ(eqTestSampleRate)
	if err != nil {
		t.Fatalf("NewDynamicEQ: %v", err)
	}

	for i := range maxDynamicEQBands {
		if _, err := eq.AddBand(EQBandConfig{FrequencyHz: 100 * float64(i+1)}); err != nil {
			t.Fatalf("AddBand(%d): %v", i, err)
		}
	}

	if _, err := eq.AddBand(EQBandConfig{FrequencyHz: 1000}); err == nil {
		t.Fatal("AddBand beyond maxDynamicEQBands should fail")
	}

	if got := eq.NumBands(); got != maxDynamicEQBands {
		t.Fatalf("NumBands = %d, want %d", got, maxDynamicEQBands)
	}
}

func TestDynamicEQBandIndexErrors(t *testing.T) {
	eq := newTestEQ(t, EQBandConfig{FrequencyHz: 1000})

	if err := eq.SetBandThreshold(1, -10); err == nil {
		t.Error("SetBandThreshold with out-of-range index should fail")
	}

	if err := eq.SetBandFrequency(-1, 500); err == nil {
		t.Error("SetBandFrequency with negative index should fail")
	}

	if _, err := eq.BandConfig(3); err == nil {
		t.Error("BandConfig with out-of-range index should fail")
	}

	if _, err := eq.BandCoefficients(3); err == nil {
		t.Error("BandCoefficients with out-of-range index should fail")
	}

	if _, err := eq.BandGainDB(3); err == nil {
		t.Error("BandGainDB with out-of-range index should fail")
	}

	if _, err := eq.BandStaticCurve(3, -60, 0, 6); err == nil {
		t.Error("BandStaticCurve with out-of-range index should fail")
	}
}

func TestDynamicEQUpdateIntervalValidation(t *testing.T) {
	eq := newTestEQ(t, EQBandConfig{FrequencyHz: 1000})

	if got := eq.UpdateInterval(); got != defaultEQUpdateInterval {
		t.Fatalf("UpdateInterval = %d, want %d", got, defaultEQUpdateInterval)
	}

	if err := eq.SetUpdateInterval(0); err == nil {
		t.Error("SetUpdateInterval(0) should fail")
	}

	if err := eq.SetUpdateInterval(maxEQUpdateInterval + 1); err == nil {
		t.Error("SetUpdateInterval above maximum should fail")
	}

	if err := eq.SetUpdateInterval(1); err != nil {
		t.Errorf("SetUpdateInterval(1): %v", err)
	}
}

// TestDynamicEQStaticBandMatchesDesigner verifies that a static band is exactly
// the canonical parametric section from dsp/filter/design.
func TestDynamicEQStaticBandMatchesDesigner(t *testing.T) {
	tests := []struct {
		name string
		typ  EQBandType
		want func(freq, gainDB, q, sr float64) any
	}{
		{"peak", EQBandPeak, nil},
		{"low shelf", EQBandLowShelf, nil},
		{"high shelf", EQBandHighShelf, nil},
	}

	const (
		freq   = 1000.0
		q      = 1.5
		gainDB = 6.0
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eq := newTestEQ(t, EQBandConfig{
				Type:         tt.typ,
				FrequencyHz:  freq,
				Q:            q,
				StaticGainDB: gainDB,
				Mode:         EQBandModeStatic,
			})

			want := design.Peak(freq, gainDB, q, eqTestSampleRate)

			switch tt.typ {
			case EQBandLowShelf:
				want = design.LowShelf(freq, gainDB, q, eqTestSampleRate)
			case EQBandHighShelf:
				want = design.HighShelf(freq, gainDB, q, eqTestSampleRate)
			case EQBandPeak:
			}

			buf := sineBuffer(freq, 0.5, 512, eqTestSampleRate)
			eq.ProcessInPlace(buf)

			got, err := eq.BandCoefficients(0)
			if err != nil {
				t.Fatalf("BandCoefficients: %v", err)
			}

			if got != want {
				t.Errorf("coefficients = %+v, want %+v", got, want)
			}
		})
	}
}

// TestDynamicEQDownwardGainVsLevel is the core static-behaviour test: the band
// gain must track the shared gain computer's prediction for the detector level.
func TestDynamicEQDownwardGainVsLevel(t *testing.T) {
	const (
		freq        = 1000.0
		thresholdDB = -20.0
		ratio       = 4.0
	)

	tests := []struct {
		name    string
		amp     float64
		wantCut bool
	}{
		{"below threshold", 0.01, false}, // -40 dBFS
		{"above threshold", 0.5, true},   // -6 dBFS
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eq := newTestEQ(t, EQBandConfig{
				FrequencyHz: freq,
				Q:           2,
				Mode:        EQBandModeDownward,
				ThresholdDB: thresholdDB,
				Ratio:       ratio,
				KneeDB:      Float64Ptr(0),
				AttackMs:    1,
				ReleaseMs:   50,
				RangeDB:     24,
			})

			if err := eq.SetUpdateInterval(1); err != nil {
				t.Fatalf("SetUpdateInterval: %v", err)
			}

			buf := sineBuffer(freq, tt.amp, 24000, eqTestSampleRate)
			eq.ProcessInPlace(buf)

			gainDB, err := eq.BandGainDB(0)
			if err != nil {
				t.Fatalf("BandGainDB: %v", err)
			}

			if !tt.wantCut {
				if gainDB != 0 {
					t.Fatalf("band gain = %.4f dB, want 0 below threshold", gainDB)
				}

				return
			}

			if gainDB >= 0 {
				t.Fatalf("band gain = %.4f dB, want negative above threshold", gainDB)
			}

			// The steady-state envelope of a peak detector on a sine settles at
			// the peak amplitude, so the gain computer's prediction for that
			// level is the expected band gain.
			want := ampToDB(eq.bands[0].core.GainForLevel(tt.amp))
			if math.Abs(gainDB-want) > 0.5 {
				t.Errorf("band gain = %.4f dB, want %.4f dB (from GainForLevel)", gainDB, want)
			}

			// The realized magnitude response at the band frequency must match.
			coeffs, err := eq.BandCoefficients(0)
			if err != nil {
				t.Fatalf("BandCoefficients: %v", err)
			}

			if got := coeffs.MagnitudeDB(freq, eqTestSampleRate); math.Abs(got-gainDB) > 1e-9 {
				t.Errorf("magnitude at %g Hz = %.6f dB, want %.6f dB", freq, got, gainDB)
			}
		})
	}
}

// TestDynamicEQModeDirections checks the sign of the dynamic gain for each mode
// above and below threshold.
func TestDynamicEQModeDirections(t *testing.T) {
	const (
		freq        = 1000.0
		thresholdDB = -20.0
	)

	tests := []struct {
		name     string
		mode     EQBandMode
		amp      float64
		wantSign int // -1 cut, 0 flat, +1 boost
	}{
		{"downward above", EQBandModeDownward, 0.5, -1},
		{"downward below", EQBandModeDownward, 0.005, 0},
		{"upward above", EQBandModeUpward, 0.5, +1},
		{"upward below", EQBandModeUpward, 0.005, 0},
		{"upward-below below", EQBandModeUpwardBelow, 0.005, +1},
		{"upward-below above", EQBandModeUpwardBelow, 0.5, 0},
		{"static above", EQBandModeStatic, 0.5, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eq := newTestEQ(t, EQBandConfig{
				FrequencyHz: freq,
				Q:           2,
				Mode:        tt.mode,
				ThresholdDB: thresholdDB,
				Ratio:       4,
				KneeDB:      Float64Ptr(0),
				AttackMs:    1,
				ReleaseMs:   50,
				RangeDB:     24,
			})

			buf := sineBuffer(freq, tt.amp, 24000, eqTestSampleRate)
			eq.ProcessInPlace(buf)

			gainDB, err := eq.BandGainDB(0)
			if err != nil {
				t.Fatalf("BandGainDB: %v", err)
			}

			switch tt.wantSign {
			case -1:
				if gainDB >= -0.1 {
					t.Errorf("band gain = %.4f dB, want a cut", gainDB)
				}
			case 0:
				if math.Abs(gainDB) > 0.1 {
					t.Errorf("band gain = %.4f dB, want flat", gainDB)
				}
			case +1:
				if gainDB <= 0.1 {
					t.Errorf("band gain = %.4f dB, want a boost", gainDB)
				}
			}
		})
	}
}

// TestDynamicEQUpwardMirrorsDownward verifies that upward mode is the exact
// mirror image of downward mode, and that upward-below mirrors the compressive
// curve about the threshold.
func TestDynamicEQUpwardMirrorsDownward(t *testing.T) {
	const (
		freq        = 1000.0
		thresholdDB = -20.0
	)

	mk := func(mode EQBandMode) *eqBand {
		eq := newTestEQ(t, EQBandConfig{
			FrequencyHz: freq,
			Mode:        mode,
			ThresholdDB: thresholdDB,
			Ratio:       4,
			KneeDB:      Float64Ptr(6),
			RangeDB:     48,
		})

		return eq.bands[0]
	}

	down, up, below := mk(EQBandModeDownward), mk(EQBandModeUpward), mk(EQBandModeUpwardBelow)

	for levelDB := -80.0; levelDB <= 0.0; levelDB += 2.5 {
		level := math.Pow(10, levelDB/20)

		d := down.gainDBForLevel(level)
		if got := up.gainDBForLevel(level); math.Abs(got+d) > 1e-9 {
			t.Fatalf("at %.1f dB: upward = %.9f, want %.9f (negated downward)", levelDB, got, -d)
		}

		// Upward-below at N dB under the threshold must equal the negated
		// downward gain at N dB over it.
		mirroredDB := 2*thresholdDB - levelDB

		want := -down.gainDBForLevel(math.Pow(10, mirroredDB/20))
		if got := below.gainDBForLevel(level); math.Abs(got-want) > 1e-9 {
			t.Fatalf("at %.1f dB: upward-below = %.9f, want %.9f", levelDB, got, want)
		}
	}
}

func TestDynamicEQRangeClamp(t *testing.T) {
	const rangeDB = 3.0

	eq := newTestEQ(t, EQBandConfig{
		FrequencyHz: 1000,
		Mode:        EQBandModeDownward,
		ThresholdDB: -60,
		Ratio:       20,
		KneeDB:      Float64Ptr(0),
		RangeDB:     rangeDB,
	})

	band := eq.bands[0]
	for levelDB := -80.0; levelDB <= 0.0; levelDB += 1 {
		got := band.gainDBForLevel(math.Pow(10, levelDB/20))
		if got < -rangeDB-1e-12 || got > rangeDB+1e-12 {
			t.Fatalf("at %.1f dB: gain %.4f exceeds range %g", levelDB, got, rangeDB)
		}
	}

	// A very loud signal must saturate at exactly -rangeDB.
	if got := band.gainDBForLevel(1.0); math.Abs(got+rangeDB) > 1e-12 {
		t.Errorf("saturated gain = %.6f dB, want %.6f dB", got, -rangeDB)
	}

	// Upward-below with a silent detector must saturate at +rangeDB.
	if err := eq.SetBandMode(0, EQBandModeUpwardBelow); err != nil {
		t.Fatalf("SetBandMode: %v", err)
	}

	if got := band.gainDBForLevel(0); math.Abs(got-rangeDB) > 1e-12 {
		t.Errorf("silent-detector gain = %.6f dB, want %.6f dB", got, rangeDB)
	}
}

// TestDynamicEQBandSelectivity checks that a band-filtered detector only reacts
// to energy inside its own region, and that wideband detection does not.
func TestDynamicEQBandSelectivity(t *testing.T) {
	newEQ := func(src EQDetectorSource) *DynamicEQ {
		return newTestEQ(
			t,
			EQBandConfig{
				FrequencyHz:    100,
				Q:              2,
				Mode:           EQBandModeDownward,
				ThresholdDB:    -30,
				Ratio:          4,
				KneeDB:         Float64Ptr(0),
				AttackMs:       1,
				ReleaseMs:      50,
				DetectorSource: src,
			},
			EQBandConfig{
				FrequencyHz:    5000,
				Q:              2,
				Mode:           EQBandModeDownward,
				ThresholdDB:    -30,
				Ratio:          4,
				KneeDB:         Float64Ptr(0),
				AttackMs:       1,
				ReleaseMs:      50,
				DetectorSource: src,
			},
		)
	}

	gains := func(eq *DynamicEQ) (float64, float64) {
		buf := sineBuffer(100, 0.5, 24000, eqTestSampleRate)
		eq.ProcessInPlace(buf)

		low, err := eq.BandGainDB(0)
		if err != nil {
			t.Fatalf("BandGainDB(0): %v", err)
		}

		high, err := eq.BandGainDB(1)
		if err != nil {
			t.Fatalf("BandGainDB(1): %v", err)
		}

		return low, high
	}

	lowGain, highGain := gains(newEQ(EQDetectorBandpass))
	if lowGain >= -1.0 {
		t.Errorf("bandpass detector: 100 Hz band gain = %.4f dB, want a clear cut", lowGain)
	}

	if math.Abs(highGain) > 0.5 {
		t.Errorf("bandpass detector: 5 kHz band gain = %.4f dB, want near zero for a 100 Hz tone", highGain)
	}

	lowGain, highGain = gains(newEQ(EQDetectorWideband))
	if lowGain >= -1.0 || highGain >= -1.0 {
		t.Errorf("wideband detector: gains = (%.4f, %.4f) dB, want both bands to react", lowGain, highGain)
	}
}

// TestDynamicEQMultiBandInteraction verifies that the series chain applies both
// bands and that a tone in one region is attenuated while the other passes.
func TestDynamicEQMultiBandInteraction(t *testing.T) {
	eq := newTestEQ(
		t,
		EQBandConfig{
			FrequencyHz: 200,
			Q:           1.5,
			Mode:        EQBandModeDownward,
			ThresholdDB: -30,
			Ratio:       8,
			KneeDB:      Float64Ptr(0),
			AttackMs:    1,
			ReleaseMs:   30,
			RangeDB:     24,
		},
		EQBandConfig{
			FrequencyHz: 6000,
			Q:           1.5,
			Mode:        EQBandModeDownward,
			ThresholdDB: -30,
			Ratio:       8,
			KneeDB:      Float64Ptr(0),
			AttackMs:    1,
			ReleaseMs:   30,
			RangeDB:     24,
		},
	)

	const n = 24000

	loud := sineBuffer(6000, 0.6, n, eqTestSampleRate)
	quiet := sineBuffer(200, 0.005, n, eqTestSampleRate)

	mix := make([]float64, n)
	for i := range mix {
		mix[i] = loud[i] + quiet[i]
	}

	out := make([]float64, n)
	copy(out, mix)
	eq.ProcessInPlace(out)

	// Compare RMS of the tail, where the detectors have settled.
	const skip = n / 2

	loudIn, quietIn := rmsTail(loud, skip), rmsTail(quiet, skip)

	// Isolate the output components by re-running each tone through the settled
	// coefficients of the corresponding band.
	highCoeffs, err := eq.BandCoefficients(1)
	if err != nil {
		t.Fatalf("BandCoefficients(1): %v", err)
	}

	lowCoeffs, err := eq.BandCoefficients(0)
	if err != nil {
		t.Fatalf("BandCoefficients(0): %v", err)
	}

	highGainDB := highCoeffs.MagnitudeDB(6000, eqTestSampleRate)
	lowGainDB := lowCoeffs.MagnitudeDB(200, eqTestSampleRate)

	if highGainDB >= -3.0 {
		t.Errorf("6 kHz band gain = %.4f dB, want a clear cut for the loud tone", highGainDB)
	}

	if math.Abs(lowGainDB) > 0.5 {
		t.Errorf("200 Hz band gain = %.4f dB, want near zero for the quiet tone", lowGainDB)
	}

	if loudIn <= 0 || quietIn <= 0 {
		t.Fatal("test signal has zero energy")
	}

	if rmsTail(out, skip) >= rmsTail(mix, skip) {
		t.Error("output RMS should be reduced by the compressed high band")
	}
}

func TestDynamicEQDeterminismAndReset(t *testing.T) {
	cfg := EQBandConfig{
		FrequencyHz: 2000,
		Q:           1.2,
		Mode:        EQBandModeDownward,
		ThresholdDB: -25,
		Ratio:       4,
		AttackMs:    5,
		ReleaseMs:   80,
	}

	eq := newTestEQ(t, cfg)

	first := sineBuffer(2000, 0.4, 4096, eqTestSampleRate)
	eq.ProcessInPlace(first)

	eq.Reset()

	second := sineBuffer(2000, 0.4, 4096, eqTestSampleRate)
	eq.ProcessInPlace(second)

	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("sample %d differs after Reset: %v vs %v", i, first[i], second[i])
		}
	}

	// A reset processor must equal a freshly constructed one.
	fresh := newTestEQ(t, cfg)

	third := sineBuffer(2000, 0.4, 4096, eqTestSampleRate)
	fresh.ProcessInPlace(third)

	for i := range first {
		if first[i] != third[i] {
			t.Fatalf("sample %d differs from a fresh processor: %v vs %v", i, first[i], third[i])
		}
	}

	if gain, err := eq.BandGainDB(0); err != nil || gain == 0 {
		t.Fatalf("expected non-zero gain before reset check, got %v (err %v)", gain, err)
	}

	eq.Reset()

	if gain, _ := eq.BandGainDB(0); gain != 0 {
		t.Errorf("band gain after Reset = %v, want 0", gain)
	}
}

// TestDynamicEQUpdateInterval verifies that control-rate updating stays close
// to per-sample updating and that the interval does not disturb filter state.
func TestDynamicEQUpdateInterval(t *testing.T) {
	cfg := EQBandConfig{
		FrequencyHz: 3000,
		Q:           1.5,
		Mode:        EQBandModeDownward,
		ThresholdDB: -25,
		Ratio:       4,
		AttackMs:    10,
		ReleaseMs:   100,
	}

	run := func(interval int) []float64 {
		eq := newTestEQ(t, cfg)
		if err := eq.SetUpdateInterval(interval); err != nil {
			t.Fatalf("SetUpdateInterval(%d): %v", interval, err)
		}

		buf := sineBuffer(3000, 0.5, 8192, eqTestSampleRate)
		eq.ProcessInPlace(buf)

		return buf
	}

	exact, coarse := run(1), run(defaultEQUpdateInterval)

	maxDiff := 0.0
	for i := range exact {
		if d := math.Abs(exact[i] - coarse[i]); d > maxDiff {
			maxDiff = d
		}
	}

	// The two runs differ only by the gain lag of one update interval during the
	// attack transient (0.67 ms of a 10 ms attack at 48 kHz), which is a small
	// fraction of the 0.5 peak amplitude.
	if maxDiff > 0.05 {
		t.Errorf("max deviation between interval 1 and %d = %.6f, want <= 0.05", defaultEQUpdateInterval, maxDiff)
	}

	// The coarse run must still settle to the same steady-state gain.
	eqExact, eqCoarse := newTestEQ(t, cfg), newTestEQ(t, cfg)
	if err := eqCoarse.SetUpdateInterval(defaultEQUpdateInterval); err != nil {
		t.Fatalf("SetUpdateInterval: %v", err)
	}

	if err := eqExact.SetUpdateInterval(1); err != nil {
		t.Fatalf("SetUpdateInterval: %v", err)
	}

	bufA := sineBuffer(3000, 0.5, 24000, eqTestSampleRate)
	bufB := sineBuffer(3000, 0.5, 24000, eqTestSampleRate)

	eqExact.ProcessInPlace(bufA)
	eqCoarse.ProcessInPlace(bufB)

	gainA, _ := eqExact.BandGainDB(0)
	gainB, _ := eqCoarse.BandGainDB(0)

	if math.Abs(gainA-gainB) > 0.05 {
		t.Errorf("steady-state gains differ: %.4f vs %.4f dB", gainA, gainB)
	}
}

func TestDynamicEQProcessInPlaceZeroAlloc(t *testing.T) {
	eq := newTestEQ(
		t,
		EQBandConfig{FrequencyHz: 120, Mode: EQBandModeDownward, ThresholdDB: -24},
		EQBandConfig{FrequencyHz: 2500, Mode: EQBandModeUpward, ThresholdDB: -24},
		EQBandConfig{FrequencyHz: 8000, Type: EQBandHighShelf, Mode: EQBandModeUpwardBelow, ThresholdDB: -24},
	)

	buf := sineBuffer(1000, 0.5, 512, eqTestSampleRate)
	sidechain := sineBuffer(1000, 0.5, 512, eqTestSampleRate)

	if allocs := testing.AllocsPerRun(20, func() { eq.ProcessInPlace(buf) }); allocs != 0 {
		t.Errorf("ProcessInPlace allocs = %v, want 0", allocs)
	}

	if allocs := testing.AllocsPerRun(20, func() {
		if err := eq.ProcessInPlaceSidechain(buf, sidechain); err != nil {
			t.Fatalf("ProcessInPlaceSidechain: %v", err)
		}
	}); allocs != 0 {
		t.Errorf("ProcessInPlaceSidechain allocs = %v, want 0", allocs)
	}
}

func TestDynamicEQSidechain(t *testing.T) {
	eq := newTestEQ(t, EQBandConfig{
		FrequencyHz: 1000,
		Q:           2,
		Mode:        EQBandModeDownward,
		ThresholdDB: -30,
		Ratio:       6,
		KneeDB:      Float64Ptr(0),
		AttackMs:    1,
		ReleaseMs:   50,
	})

	const n = 12000

	program := sineBuffer(1000, 0.02, n, eqTestSampleRate) // quiet program
	sidechain := sineBuffer(1000, 0.8, n, eqTestSampleRate)

	if err := eq.ProcessInPlaceSidechain(program, sidechain); err != nil {
		t.Fatalf("ProcessInPlaceSidechain: %v", err)
	}

	gain, err := eq.BandGainDB(0)
	if err != nil {
		t.Fatalf("BandGainDB: %v", err)
	}

	if gain >= -3.0 {
		t.Errorf("band gain = %.4f dB, want the loud sidechain to drive a clear cut", gain)
	}

	if err := eq.ProcessInPlaceSidechain(program, sidechain[:10]); err == nil {
		t.Error("mismatched sidechain length should fail")
	}
}

func TestDynamicEQMetrics(t *testing.T) {
	eq := newTestEQ(
		t,
		EQBandConfig{FrequencyHz: 1000, Mode: EQBandModeDownward, ThresholdDB: -30, Ratio: 6, AttackMs: 1},
		EQBandConfig{FrequencyHz: 4000, Mode: EQBandModeUpward, ThresholdDB: -30, Ratio: 6, AttackMs: 1},
	)

	buf := sineBuffer(1000, 0.6, 12000, eqTestSampleRate)
	eq.ProcessInPlace(buf)

	metrics := eq.GetMetrics()
	if len(metrics.Bands) != 2 {
		t.Fatalf("metrics bands = %d, want 2", len(metrics.Bands))
	}

	if metrics.Bands[0].InputPeak <= 0 || metrics.Bands[0].OutputPeak <= 0 {
		t.Errorf("band 0 peaks not recorded: %+v", metrics.Bands[0])
	}

	if metrics.Bands[0].MinGainDB >= 0 {
		t.Errorf("band 0 MinGainDB = %.4f, want negative", metrics.Bands[0].MinGainDB)
	}

	eq.ResetMetrics()

	if got := eq.GetMetrics().Bands[0]; got != (EQBandMetrics{}) {
		t.Errorf("metrics after ResetMetrics = %+v, want zero", got)
	}
}

func TestDynamicEQStaticCurve(t *testing.T) {
	eq := newTestEQ(t, EQBandConfig{
		FrequencyHz:  1000,
		StaticGainDB: 0,
		Mode:         EQBandModeDownward,
		ThresholdDB:  -20,
		Ratio:        4,
		KneeDB:       Float64Ptr(0),
		RangeDB:      24,
	})

	points, err := eq.BandStaticCurve(0, -60, 0, 5)
	if err != nil {
		t.Fatalf("BandStaticCurve: %v", err)
	}

	if len(points) != 13 {
		t.Fatalf("curve points = %d, want 13", len(points))
	}

	if points[0].InputDB != -60 || points[len(points)-1].InputDB != 0 {
		t.Errorf("curve endpoints = %.1f..%.1f, want -60..0", points[0].InputDB, points[len(points)-1].InputDB)
	}

	for _, p := range points {
		if p.InputDB <= -20 {
			if math.Abs(p.GainReductionDB) > 1e-9 {
				t.Errorf("at %.1f dB: gain %.6f dB, want unity below threshold", p.InputDB, p.GainReductionDB)
			}

			continue
		}

		want := -(p.InputDB - (-20.0)) * (1 - 1.0/4.0)
		if math.Abs(p.GainReductionDB-want) > 1e-9 {
			t.Errorf("at %.1f dB: gain %.6f dB, want %.6f dB", p.InputDB, p.GainReductionDB, want)
		}
	}

	// The curve must not disturb detector state.
	before, _ := eq.BandGainDB(0)

	if _, err := eq.BandStaticCurve(0, -60, 0, 5); err != nil {
		t.Fatalf("BandStaticCurve: %v", err)
	}

	if after, _ := eq.BandGainDB(0); after != before {
		t.Errorf("static curve mutated band state: %v -> %v", before, after)
	}
}

func TestDynamicEQSetters(t *testing.T) {
	eq := newTestEQ(t, EQBandConfig{FrequencyHz: 1000})

	setters := []struct {
		name string
		ok   func() error
		bad  func() error
	}{
		{"frequency", func() error { return eq.SetBandFrequency(0, 2000) }, func() error { return eq.SetBandFrequency(0, 0) }},
		{"q", func() error { return eq.SetBandQ(0, 3) }, func() error { return eq.SetBandQ(0, 0.001) }},
		{"static gain", func() error { return eq.SetBandStaticGain(0, -3) }, func() error { return eq.SetBandStaticGain(0, 200) }},
		{"type", func() error { return eq.SetBandType(0, EQBandLowShelf) }, func() error { return eq.SetBandType(0, EQBandType(9)) }},
		{"mode", func() error { return eq.SetBandMode(0, EQBandModeUpward) }, func() error { return eq.SetBandMode(0, EQBandMode(9)) }},
		{"range", func() error { return eq.SetBandRange(0, 6) }, func() error { return eq.SetBandRange(0, 500) }},
		{
			"detector source",
			func() error { return eq.SetBandDetectorSource(0, EQDetectorWideband) },
			func() error { return eq.SetBandDetectorSource(0, EQDetectorSource(9)) },
		},
		{"detector q", func() error { return eq.SetBandDetectorQ(0, 4) }, func() error { return eq.SetBandDetectorQ(0, 0) }},
		{"threshold", func() error { return eq.SetBandThreshold(0, -18) }, func() error { return eq.SetBandThreshold(0, math.NaN()) }},
		{"ratio", func() error { return eq.SetBandRatio(0, 8) }, func() error { return eq.SetBandRatio(0, 0.1) }},
		{"knee", func() error { return eq.SetBandKnee(0, 12) }, func() error { return eq.SetBandKnee(0, -1) }},
		{"attack", func() error { return eq.SetBandAttack(0, 20) }, func() error { return eq.SetBandAttack(0, 0) }},
		{"release", func() error { return eq.SetBandRelease(0, 250) }, func() error { return eq.SetBandRelease(0, 0) }},
		{
			"detector mode",
			func() error { return eq.SetBandDetectorMode(0, DetectorModeRMS) },
			func() error { return eq.SetBandDetectorMode(0, DetectorMode(9)) },
		},
		{"rms window", func() error { return eq.SetBandRMSWindow(0, 50) }, func() error { return eq.SetBandRMSWindow(0, 0) }},
		{"sidechain low cut", func() error { return eq.SetBandSidechainLowCut(0, 80) }, func() error { return eq.SetBandSidechainLowCut(0, -1) }},
		{
			"sidechain high cut",
			func() error { return eq.SetBandSidechainHighCut(0, 12000) },
			func() error { return eq.SetBandSidechainHighCut(0, -1) },
		},
	}

	for _, s := range setters {
		if err := s.ok(); err != nil {
			t.Errorf("%s: valid value rejected: %v", s.name, err)
		}

		if err := s.bad(); err == nil {
			t.Errorf("%s: invalid value accepted", s.name)
		}
	}

	cfg, err := eq.BandConfig(0)
	if err != nil {
		t.Fatalf("BandConfig: %v", err)
	}

	if cfg.FrequencyHz != 2000 || cfg.Q != 3 || cfg.StaticGainDB != -3 || cfg.Type != EQBandLowShelf {
		t.Errorf("resolved config = %+v, want the applied setter values", cfg)
	}

	if cfg.KneeDB == nil || *cfg.KneeDB != 12 {
		t.Errorf("resolved knee = %v, want 12", cfg.KneeDB)
	}

	if cfg.DetectorMode == nil || *cfg.DetectorMode != DetectorModeRMS {
		t.Errorf("resolved detector mode = %v, want RMS", cfg.DetectorMode)
	}
}

func TestDynamicEQSetBandConfigAndSampleRate(t *testing.T) {
	eq := newTestEQ(t, EQBandConfig{FrequencyHz: 1000})

	if err := eq.SetBandConfig(0, EQBandConfig{FrequencyHz: 4000, Type: EQBandHighShelf, StaticGainDB: 4}); err != nil {
		t.Fatalf("SetBandConfig: %v", err)
	}

	got, err := eq.BandCoefficients(0)
	if err != nil {
		t.Fatalf("BandCoefficients: %v", err)
	}

	want := design.HighShelf(4000, 4, defaultEQBandQ, eqTestSampleRate)
	if got != want {
		t.Errorf("coefficients = %+v, want %+v", got, want)
	}

	if err := eq.SetBandConfig(0, EQBandConfig{FrequencyHz: 0}); err == nil {
		t.Error("SetBandConfig with invalid frequency should fail")
	}

	if err := eq.SetBandConfig(5, EQBandConfig{FrequencyHz: 1000}); err == nil {
		t.Error("SetBandConfig with invalid index should fail")
	}

	if err := eq.SetSampleRate(96000); err != nil {
		t.Fatalf("SetSampleRate: %v", err)
	}

	if eq.SampleRate() != 96000 {
		t.Errorf("SampleRate = %v, want 96000", eq.SampleRate())
	}

	got, _ = eq.BandCoefficients(0)
	if want := design.HighShelf(4000, 4, defaultEQBandQ, 96000); got != want {
		t.Errorf("coefficients after resample = %+v, want %+v", got, want)
	}

	if err := eq.SetSampleRate(0); err == nil {
		t.Error("SetSampleRate(0) should fail")
	}

	// Dropping the sample rate below 2x the band frequency must be rejected.
	if err := eq.SetSampleRate(4000); err == nil {
		t.Error("SetSampleRate below 2x the band frequency should fail")
	}
}

func TestDynamicEQEmptyChainIsTransparent(t *testing.T) {
	eq, err := NewDynamicEQ(eqTestSampleRate)
	if err != nil {
		t.Fatalf("NewDynamicEQ: %v", err)
	}

	buf := sineBuffer(1000, 0.5, 128, eqTestSampleRate)

	want := make([]float64, len(buf))
	copy(want, buf)

	eq.ProcessInPlace(buf)

	for i := range buf {
		if buf[i] != want[i] {
			t.Fatalf("sample %d = %v, want %v (empty chain must be transparent)", i, buf[i], want[i])
		}
	}
}
