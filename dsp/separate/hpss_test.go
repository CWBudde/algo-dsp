package separate_test

import (
	"errors"
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/separate"
	"github.com/cwbudde/algo-dsp/dsp/stft"
	"github.com/cwbudde/algo-dsp/internal/testutil"
)

// toneAndClicks returns a steady tone (440 Hz with two overtones, faded in
// and out over 20 ms) and a train of single-sample clicks every 0.5 s
// starting at 0.25 s, both n samples at sample rate fs. The clicks are
// scaled so that the tone has 10 dB more energy than the click train.
func toneAndClicks(fs float64, n int) (tone, clicks []float64) {
	tone = make([]float64, n)
	clicks = make([]float64, n)
	fade := int(0.02 * fs)

	for i := range tone {
		t := float64(i) / fs
		v := 0.3*math.Sin(2*math.Pi*440*t) + 0.15*math.Sin(2*math.Pi*880*t) + 0.075*math.Sin(2*math.Pi*1320*t)

		g := 1.0
		if d := min(i, n-1-i); d < fade {
			g = 0.5 - 0.5*math.Cos(math.Pi*float64(d)/float64(fade))
		}

		tone[i] = g * v
	}

	count := 0
	for pos := int(0.25 * fs); pos < n; pos += int(0.5 * fs) {
		count++
	}

	amp := math.Sqrt(energy(tone) / 10 / float64(count))
	for pos := int(0.25 * fs); pos < n; pos += int(0.5 * fs) {
		clicks[pos] = amp
	}

	return tone, clicks
}

func add(a, b []float64) []float64 {
	out := make([]float64, len(a))
	for i := range a {
		out[i] = a[i] + b[i]
	}

	return out
}

func energy(x []float64) float64 {
	e := 0.0
	for _, v := range x {
		e += v * v
	}

	return e
}

// relErrDB returns 10·log10(Σ(got−want)² / Σwant²).
func relErrDB(got, want []float64) float64 {
	d := 0.0

	for i := range want {
		e := got[i] - want[i]
		d += e * e
	}

	return 10 * math.Log10(d/energy(want))
}

func sum3(a, b, c []float64) []float64 {
	out := make([]float64, len(a))
	for i := range a {
		out[i] = a[i] + b[i] + c[i]
	}

	return out
}

// isolation applies the masks computed from the mixture to each component
// separately and returns, per output, 10·log10(right/wrong) in dB.
func isolation(t *testing.T, h *separate.HPSS, tone, clicks []float64) (harmDB, percDB float64) {
	t.Helper()

	s := h.STFT()

	mixSpec, err := s.Forward(add(tone, clicks))
	if err != nil {
		t.Fatal(err)
	}

	mh, mp, _, err := h.Masks(mixSpec)
	if err != nil {
		t.Fatal(err)
	}

	masked := func(x []float64, mask [][]float64) []float64 {
		spec, err := s.Forward(x)
		if err != nil {
			t.Fatal(err)
		}

		err = separate.ApplyMask(spec, spec, mask)
		if err != nil {
			t.Fatal(err)
		}

		y, err := s.Inverse(spec, len(x))
		if err != nil {
			t.Fatal(err)
		}

		return y
	}

	harmDB = 10 * math.Log10(energy(masked(tone, mh))/energy(masked(clicks, mh)))
	percDB = 10 * math.Log10(energy(masked(clicks, mp))/energy(masked(tone, mp)))

	return harmDB, percDB
}

func TestHPSSIsolation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fs   float64
		opts []separate.Option
	}{
		{"24k default", 24000, nil},
		{"44k1 default", 44100, nil},
		{"24k margin2", 24000, []separate.Option{separate.WithMargin(2, 2)}},
		{"24k binary margin", 24000, []separate.Option{separate.WithMargin(2, 2), separate.WithPower(math.Inf(1))}},
		{"24k kernels31", 24000, []separate.Option{separate.WithKernels(31, 31)}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h, err := separate.NewHPSS(tc.opts...)
			if err != nil {
				t.Fatal(err)
			}

			tone, clicks := toneAndClicks(tc.fs, int(2*tc.fs))
			harmDB, percDB := isolation(t, h, tone, clicks)

			harm, perc, resid, err := h.SeparateSignal(add(tone, clicks))
			if err != nil {
				t.Fatal(err)
			}

			sdrH := -relErrDB(harm, tone)
			sdrP := -relErrDB(perc, clicks)
			t.Logf("%s: isolation harm %.1f dB, perc %.1f dB; SDR harm %.1f dB, perc %.1f dB; resid %.1f dB",
				tc.name, harmDB, percDB, sdrH, sdrP, 10*math.Log10(energy(resid)/energy(add(tone, clicks))+1e-300))

			if harmDB < 20 || percDB < 20 {
				t.Errorf("isolation harm %.1f dB, perc %.1f dB, want > 20 dB", harmDB, percDB)
			}
		})
	}
}

func TestHPSSReconstruction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts []separate.Option
	}{
		{"default", nil},
		{"margin", []separate.Option{separate.WithMargin(3, 2)}},
		{"binary margin", []separate.Option{separate.WithMargin(1.5, 1.5), separate.WithPower(math.Inf(1))}},
		{"power 1", []separate.Option{separate.WithPower(1)}},
		{"power 3.5", []separate.Option{separate.WithPower(3.5), separate.WithKernels(5, 31)}},
		{"reflect stft", []separate.Option{separate.WithSTFT(1024, 256, stft.WithCenter(stft.PadReflect))}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h, err := separate.NewHPSS(tc.opts...)
			if err != nil {
				t.Fatal(err)
			}

			tone, clicks := toneAndClicks(24000, 24000)
			x := add(add(tone, clicks), testutil.DeterministicNoise(7, 0.05, len(tone)))

			harm, perc, resid, err := h.SeparateSignal(x)
			if err != nil {
				t.Fatal(err)
			}

			sigDB := relErrDB(sum3(harm, perc, resid), x)
			if sigDB > -100 {
				t.Errorf("signal reconstruction error %.1f dB, want < -100 dB", sigDB)
			}

			if !h.HasMargin() && energy(resid) != 0 {
				t.Errorf("residual energy %g without margin, want 0", energy(resid))
			}

			spec, err := h.STFT().Forward(x)
			if err != nil {
				t.Fatal(err)
			}

			hs, ps, rs, err := h.Separate(spec)
			if err != nil {
				t.Fatal(err)
			}

			num, den := 0.0, 0.0

			for i := range spec {
				for k := range spec[i] {
					d := hs[i][k] + ps[i][k] + rs[i][k] - spec[i][k]
					num += real(d)*real(d) + imag(d)*imag(d)
					den += real(spec[i][k])*real(spec[i][k]) + imag(spec[i][k])*imag(spec[i][k])
				}
			}

			specDB := 10 * math.Log10(num/den+1e-300)
			if specDB > -100 {
				t.Errorf("spectrogram reconstruction error %.1f dB, want < -100 dB", specDB)
			}

			t.Logf("reconstruction: signal %.1f dB, spectrogram %.1f dB", sigDB, specDB)
		})
	}
}

func TestHPSSMasksSumToOne(t *testing.T) {
	t.Parallel()

	for _, opts := range [][]separate.Option{
		nil,
		{separate.WithMargin(2, 3)},
		{separate.WithMargin(2, 3), separate.WithPower(math.Inf(1))},
		{separate.WithPower(math.Inf(1))},
	} {
		h, err := separate.NewHPSS(opts...)
		if err != nil {
			t.Fatal(err)
		}

		// Include an all-zero region so that 0/0 bins occur.
		x := testutil.DeterministicNoise(3, 1, 24000)
		clear(x[4000:20000])

		spec, err := h.STFT().Forward(x)
		if err != nil {
			t.Fatal(err)
		}

		mh, mp, mr, err := h.Masks(spec)
		if err != nil {
			t.Fatal(err)
		}

		for i := range mh {
			for k := range mh[i] {
				s := mh[i][k] + mp[i][k] + mr[i][k]
				if math.Abs(s-1) > 1e-15 {
					t.Fatalf("frame %d bin %d: masks sum to %v", i, k, s)
				}

				if !h.HasMargin() && mr[i][k] != 0 {
					t.Fatalf("residual mask %v without margin", mr[i][k])
				}

				if mh[i][k] < 0 || mp[i][k] < 0 || mr[i][k] < -1e-15 {
					t.Fatalf("negative mask %v %v %v", mh[i][k], mp[i][k], mr[i][k])
				}
			}
		}
	}
}

func TestHPSSZeroInput(t *testing.T) {
	t.Parallel()

	spec := [][]complex128{make([]complex128, 129), make([]complex128, 129)}

	for _, opts := range [][]separate.Option{nil, {separate.WithMargin(2, 2)}, {separate.WithPower(math.Inf(1))}} {
		h, err := separate.NewHPSS(opts...)
		if err != nil {
			t.Fatal(err)
		}

		mh, mp, mr, err := h.Masks(spec)
		if err != nil {
			t.Fatal(err)
		}

		for i := range mh {
			for k := range mh[i] {
				if mh[i][k] != 0.5 || mp[i][k] != 0.5 || mr[i][k] != 0 {
					t.Fatalf("zero bin masks %v %v %v, want 0.5 0.5 0", mh[i][k], mp[i][k], mr[i][k])
				}
			}
		}
	}

	h, err := separate.NewHPSS(separate.WithSTFT(256, 64))
	if err != nil {
		t.Fatal(err)
	}

	harm, perc, resid, err := h.SeparateSignal(nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(harm) != 0 || len(perc) != 0 || len(resid) != 0 {
		t.Fatalf("empty input gave lengths %d %d %d", len(harm), len(perc), len(resid))
	}

	// Empty spectrogram.
	hs, ps, rs, err := h.Separate(nil)
	if err != nil || len(hs) != 0 || len(ps) != 0 || len(rs) != 0 {
		t.Fatalf("empty spectrogram: %v %d %d %d", err, len(hs), len(ps), len(rs))
	}
}

func TestHPSSDeterminism(t *testing.T) {
	t.Parallel()

	x := testutil.DeterministicNoise(11, 0.5, 12000)

	run := func(h *separate.HPSS) ([]float64, []float64, []float64) {
		harm, perc, resid, err := h.SeparateSignal(x)
		if err != nil {
			t.Fatal(err)
		}

		return harm, perc, resid
	}

	h1, err := separate.NewHPSS(separate.WithMargin(2, 2))
	if err != nil {
		t.Fatal(err)
	}

	a1, b1, c1 := run(h1)
	a2, b2, c2 := run(h1)
	a3, b3, c3 := run(h1.Clone())

	for i := range x {
		if a1[i] != a2[i] || b1[i] != b2[i] || c1[i] != c2[i] ||
			a1[i] != a3[i] || b1[i] != b3[i] || c1[i] != c3[i] {
			t.Fatalf("sample %d differs between runs", i)
		}
	}
}

func TestHPSSErrors(t *testing.T) {
	t.Parallel()

	// The subtests run in parallel, and an HPSS must not be shared between
	// goroutines, so each call gets its own.
	h := hpssFactory{t}

	good := [][]complex128{{1, 2, 3}, {4, 5, 6}}
	ragged := [][]complex128{{1, 2, 3}, {4, 5}}
	nonFinite := [][]complex128{{1, complex(math.Inf(1), 0), 3}, {4, 5, 6}}
	nanSpec := [][]complex128{{1, complex(0, math.NaN()), 3}, {4, 5, 6}}

	m3 := func(frames, bins int) [][]float64 {
		m := make([][]float64, frames)
		for i := range m {
			m[i] = make([]float64, bins)
		}

		return m
	}

	c3 := func(frames, bins int) [][]complex128 {
		m := make([][]complex128, frames)
		for i := range m {
			m[i] = make([]complex128, bins)
		}

		return m
	}

	tests := []struct {
		name string
		call func() error
		want error
	}{
		{"masks ragged", func() error { _, _, _, err := h.Masks(ragged); return err }, separate.ErrShapeMismatch},
		{"masks inf", func() error { _, _, _, err := h.Masks(nonFinite); return err }, separate.ErrInvalidValue},
		{"masks nan", func() error { _, _, _, err := h.Masks(nanSpec); return err }, separate.ErrInvalidValue},
		{"separate ragged", func() error { _, _, _, err := h.Separate(ragged); return err }, separate.ErrShapeMismatch},
		{"separate inf", func() error { _, _, _, err := h.Separate(nonFinite); return err }, separate.ErrInvalidValue},
		{"masksinto shape", func() error { return h.MasksInto(m3(2, 3), m3(2, 3), m3(1, 3), good) }, separate.ErrShapeMismatch},
		{"masksinto ragged", func() error { return h.MasksInto(m3(2, 3), m3(2, 3), m3(2, 3), ragged) }, separate.ErrShapeMismatch},
		{"separateinto shape", func() error { return h.SeparateInto(c3(2, 3), c3(2, 2), c3(2, 3), good) }, separate.ErrShapeMismatch},
		{"signal inf", func() error {
			x := make([]float64, 4096)
			x[100] = math.Inf(1)
			_, _, _, err := h.SeparateSignal(x)

			return err
		}, separate.ErrInvalidValue},
		{"signal reflect too short", func() error {
			hr, err := separate.NewHPSS(separate.WithSTFT(256, 64, stft.WithCenter(stft.PadReflect)))
			if err != nil {
				return err
			}

			_, _, _, err = hr.SeparateSignal(make([]float64, 10))

			return err
		}, stft.ErrSignalTooShort},
		{"signal padnone window zero", func() error {
			hn, err := separate.NewHPSS(separate.WithSTFT(256, 64, stft.WithCenter(stft.PadNone)))
			if err != nil {
				return err
			}

			_, _, _, err = hn.SeparateSignal(make([]float64, 1000))

			return err
		}, stft.ErrWindowSumZero},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.call()
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

type hpssFactory struct{ t *testing.T }

func (f hpssFactory) new() *separate.HPSS {
	f.t.Helper()

	h, err := separate.NewHPSS()
	if err != nil {
		f.t.Fatal(err)
	}

	return h
}

func (f hpssFactory) Masks(spec [][]complex128) ([][]float64, [][]float64, [][]float64, error) {
	return f.new().Masks(spec)
}

func (f hpssFactory) Separate(spec [][]complex128) ([][]complex128, [][]complex128, [][]complex128, error) {
	return f.new().Separate(spec)
}

func (f hpssFactory) MasksInto(a, b, c [][]float64, spec [][]complex128) error {
	return f.new().MasksInto(a, b, c, spec)
}

func (f hpssFactory) SeparateInto(a, b, c, spec [][]complex128) error {
	return f.new().SeparateInto(a, b, c, spec)
}

func (f hpssFactory) SeparateSignal(x []float64) ([]float64, []float64, []float64, error) {
	return f.new().SeparateSignal(x)
}

func TestHPSSMasksIntoNoAllocs(t *testing.T) {
	h, err := separate.NewHPSS()
	if err != nil {
		t.Fatal(err)
	}

	spec, err := h.STFT().Forward(testutil.DeterministicNoise(5, 1, 12000))
	if err != nil {
		t.Fatal(err)
	}

	mh, mp, mr, err := h.Masks(spec)
	if err != nil {
		t.Fatal(err)
	}

	allocs := testing.AllocsPerRun(5, func() {
		err := h.MasksInto(mh, mp, mr, spec)
		if err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 0 {
		t.Fatalf("MasksInto allocates %v times per run", allocs)
	}

	hs, ps, rs, err := h.Separate(spec)
	if err != nil {
		t.Fatal(err)
	}

	allocs = testing.AllocsPerRun(5, func() {
		err := h.SeparateInto(hs, ps, rs, spec)
		if err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 0 {
		t.Fatalf("SeparateInto allocates %v times per run", allocs)
	}
}
