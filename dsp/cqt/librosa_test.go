package cqt

import (
	"math"
	"slices"
	"testing"
)

func median(v []float64) float64 {
	s := slices.Clone(v)
	slices.Sort(s)

	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}

	return (s[n/2-1] + s[n/2]) / 2
}

// TestLibrosaSanity compares the basic-pitch configuration with librosa.cqt.
//
// librosa's CQT is a different algorithm (recursive soxr resampling, other
// kernel placement), so it is not a bit-level reference: the relative
// Frobenius error between librosa and nnAudio is about 0.32 on this signal.
// What should agree is the scale that NormalizationLibrosa is meant to
// reproduce. On the interior frames the measured ratio librosa/ours has an
// overall median of 1.0013, and the per-bin medians above the lowest octave
// are within 1±0.02 for 96.7% of the bins. The outliers (0.90..1.21) sit
// next to the stationary 55 Hz and 440 Hz tones of the test signal, where the
// two algorithms leak differently into neighbouring bins. The lowest octave
// is excluded: its kernels are longer than the whole signal.
func TestLibrosaSanity(t *testing.T) {
	t.Parallel()

	fx := loadFixture(t, "basic_pitch")
	x := testSignal(t)[:fx.Length]

	tr, err := New(fx.Config.SR, fx.Config.options(t)...)
	if err != nil {
		t.Fatal(err)
	}

	got, err := tr.Process(x)
	if err != nil {
		t.Fatal(err)
	}

	frames, bins := tr.FrameCount(len(x)), tr.Bins()
	lo, hi := frames/10, frames-frames/10
	bpo := fx.Config.BinsPerOctave

	var all []float64

	close2, worst := 0, 0.0

	for b := range bins {
		var r []float64

		for fr := lo; fr < hi; fr++ {
			i := fr*bins + b
			if got[i] > 0 {
				r = append(r, fx.Librosa[i]/got[i])
			}
		}

		all = append(all, r...)

		if b < bpo {
			continue
		}

		m := median(r)
		worst = max(worst, math.Abs(m-1))

		if math.Abs(m-1) <= 0.02 {
			close2++
		}

		if m < 0.8 || m > 1.25 {
			t.Errorf("bin %d: median librosa/ours = %.4f outside [0.8, 1.25]", b, m)
		}
	}

	overall := median(all)
	frac := float64(close2) / float64(bins-bpo)
	t.Logf("overall median %.5f, %.1f%% of bins >= %d within 1±0.02, worst per-bin |median-1| %.4f",
		overall, 100*frac, bpo, worst)

	if math.Abs(overall-1) > 0.01 {
		t.Errorf("overall median librosa/ours = %.5f, want 1±0.01", overall)
	}

	if frac < 0.9 {
		t.Errorf("%.1f%% of the bins have a median within 1±0.02, want >= 90%%", 100*frac)
	}
}
