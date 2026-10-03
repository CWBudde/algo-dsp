package align_test

import (
	"errors"
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/internal/testutil"
	"github.com/cwbudde/algo-dsp/measure/music/align"
)

// stems returns four deterministic stereo parts of n samples.
func stems(n int) [][][]float64 {
	parts := make([][][]float64, 4)
	for p := range parts {
		parts[p] = [][]float64{
			testutil.DeterministicNoise(int64(2*p+1), 0.2, n),
			testutil.DeterministicNoise(int64(2*p+2), 0.2, n),
		}
	}

	sine := testutil.DeterministicSine(220, SampleRate, 0.3, n)
	for i, v := range sine {
		parts[1][0][i] += v
		parts[1][1][i] -= v
	}

	return parts
}

// mixOf sums the parts channel by channel, delayed by lag samples, plus a
// small residual.
func mixOf(parts [][][]float64, n, lag int) [][]float64 {
	mix := [][]float64{make([]float64, n), make([]float64, n)}
	residual := testutil.DeterministicNoise(99, 1e-3, n)

	for c := range mix {
		for i := range mix[c] {
			j := i - lag
			if j >= 0 && j < n {
				for _, part := range parts {
					if ch := part[min(c, len(part)-1)]; j < len(ch) {
						mix[c][i] += ch[j]
					}
				}
			}

			mix[c][i] += residual[i]
		}
	}

	return mix
}

func TestCheckParity(t *testing.T) {
	t.Parallel()

	const n = 2 * SampleRate

	parts := stems(n)
	short := stems(n)
	short[2][0] = short[2][0][:n-30]
	short[2][1] = short[2][1][:n-30]
	monoPart := append(stems(n), [][]float64{testutil.DeterministicNoise(42, 0.1, n)})

	tests := []struct {
		name  string
		mix   [][]float64
		parts [][][]float64
	}{
		{"aligned", mixOf(parts, n, 0), parts},
		{"mix late", mixOf(parts, n, 17), parts},
		{"mix early", mixOf(parts, n, -30), parts},
		{"short part", mixOf(short, n, 5), short},
		{"mono mix", mixOf(parts, n, 0)[:1], parts},
		{"five parts", mixOf(parts, n, 0), monoPart},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			want := refCheckAlignment(tc.mix, tc.parts)

			got, err := align.Check(tc.mix, tc.parts, SampleRate)
			if err != nil {
				t.Fatal(err)
			}

			if got.Correlation != want.Correlation || got.BestLag*1000 != want.BestLagMS || got.ResidualRMSDB != want.ResidualRMSDB {
				t.Fatalf("got %+v, want %+v", got, *want)
			}
		})
	}
}

func TestCheckFindsLag(t *testing.T) {
	t.Parallel()

	const n = SampleRate

	parts := stems(n)

	for _, lag := range []int{-40, -7, 0, 12, 48} {
		res, err := align.Check(mixOf(parts, n, -lag), parts, SampleRate, align.WithStride(1))
		if err != nil {
			t.Fatal(err)
		}

		if math.Round(res.BestLag*SampleRate) != float64(lag) {
			t.Errorf("lag %d: found %v samples", lag, res.BestLag*SampleRate)
		}

		if res.BestCorrelation < 0.99 {
			t.Errorf("lag %d: correlation %v", lag, res.BestCorrelation)
		}

		if lag == 0 && (res.ResidualRMSDB > -55 || res.ResidualRMSDB < -65) {
			t.Errorf("residual %v dB, want ≈ -60 dB", res.ResidualRMSDB)
		}
	}

	// A lag outside the search range is not found.
	res, err := align.Check(mixOf(parts, n, -100), parts, SampleRate, align.WithMaxLag(0.001))
	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(res.BestLag) > 0.001 || res.BestCorrelation > 0.5 {
		t.Errorf("out-of-range lag: %+v", res)
	}
}

func TestCheckLengthMismatch(t *testing.T) {
	t.Parallel()

	const n = SampleRate

	parts := stems(n)
	parts[3][1] = parts[3][1][:n-96] // 4 ms short

	mix := mixOf(stems(n), n, 0)

	res, err := align.Check(mix, parts, SampleRate)
	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(res.MaxLengthMismatch-0.004) > 1e-12 {
		t.Fatalf("mismatch %v", res.MaxLengthMismatch)
	}

	_, err = align.Check(mix, parts, SampleRate, align.WithMaxLengthMismatch(0.002))
	if !errors.Is(err, align.ErrLengthMismatch) {
		t.Fatalf("err = %v", err)
	}

	_, err = align.Check(mix, parts, SampleRate, align.WithMaxLengthMismatch(0.005))
	if err != nil {
		t.Fatal(err)
	}
}

func TestCheckSilence(t *testing.T) {
	t.Parallel()

	z := [][]float64{make([]float64, 1000)}

	res, err := align.Check(z, [][][]float64{z}, SampleRate)
	if err != nil {
		t.Fatal(err)
	}

	if res.Correlation != 0 || res.BestLag != 0 || res.BestCorrelation != 0 || res.ResidualRMSDB != -120 {
		t.Fatalf("%+v", res)
	}
}

// TestCheckLagCandidates checks that lags without a defined correlation (no
// overlap, or no energy in the overlap) never win against a genuinely
// overlapping, even negative, correlation, and that a huge WithMaxLag is
// limited to the overlapping lags.
func TestCheckLagCandidates(t *testing.T) {
	t.Parallel()

	ramp := make([]float64, 200)
	shifted := make([]float64, len(ramp))

	for i := range ramp {
		ramp[i] = math.Sin(0.3*float64(i)) + 0.01*float64(i)
		if i >= 3 {
			shifted[i] = ramp[i-3]
		}
	}

	tests := []struct {
		name     string
		ref      []float64
		part     []float64
		rate     float64
		maxLag   float64
		wantLag  float64
		wantCorr float64
	}{
		// Lags ±1..±10 have no overlap with a one-sample clip.
		{"no overlap", []float64{1}, []float64{-1}, 1, 10, 0, -1},
		// Lags ±1, ±2 overlap only silence in one of the signals.
		{"silent overlap", []float64{1, 0, 0, 0, 0}, []float64{-1, 0, 0, 0, 0}, 1, 2, 0, -1},
		{"huge max lag", ramp, shifted, SampleRate, math.MaxFloat64, 3.0 / SampleRate, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res, err := align.Check([][]float64{tc.ref}, [][][]float64{{tc.part}}, tc.rate,
				align.WithMaxLag(tc.maxLag), align.WithStride(1))
			if err != nil {
				t.Fatal(err)
			}

			if res.BestLag != tc.wantLag || math.Abs(res.BestCorrelation-tc.wantCorr) > 1e-12 {
				t.Fatalf("best lag %v (corr %v), want %v (corr %v)", res.BestLag, res.BestCorrelation, tc.wantLag, tc.wantCorr)
			}
		})
	}
}

func TestCheckErrors(t *testing.T) {
	t.Parallel()

	ref := [][]float64{make([]float64, 10)}
	parts := [][][]float64{ref}

	tests := []struct {
		name  string
		ref   [][]float64
		parts [][][]float64
		rate  float64
		opts  []align.Option
		want  error
	}{
		{"nil option", ref, parts, 1, []align.Option{nil}, align.ErrNilOption},
		{"lag", ref, parts, 1, []align.Option{align.WithMaxLag(-1)}, align.ErrInvalidArgument},
		{"stride", ref, parts, 1, []align.Option{align.WithStride(0)}, align.ErrInvalidArgument},
		{"mismatch", ref, parts, 1, []align.Option{align.WithMaxLengthMismatch(math.NaN())}, align.ErrInvalidArgument},
		{"rate", ref, parts, 0, nil, align.ErrInvalidArgument},
		{"empty ref", nil, parts, 1, nil, align.ErrInvalidArgument},
		{"ragged ref", [][]float64{make([]float64, 10), make([]float64, 9)}, parts, 1, nil, align.ErrInvalidArgument},
		{"no parts", ref, nil, 1, nil, align.ErrInvalidArgument},
		{"empty part", ref, [][][]float64{{}}, 1, nil, align.ErrInvalidArgument},
	}

	for _, tc := range tests {
		_, err := align.Check(tc.ref, tc.parts, tc.rate, tc.opts...)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
}

func BenchmarkCheck(b *testing.B) {
	parts := stems(SampleRate)
	mix := mixOf(parts, SampleRate, 0)

	b.ReportAllocs()

	for b.Loop() {
		_, err := align.Check(mix, parts, SampleRate)
		if err != nil {
			b.Fatal(err)
		}
	}
}
