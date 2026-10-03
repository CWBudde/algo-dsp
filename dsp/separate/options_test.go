package separate_test

import (
	"errors"
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/separate"
	"github.com/cwbudde/algo-dsp/dsp/stft"
)

func TestHPSSOptionErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opt  separate.Option
		want error
	}{
		{"nil", nil, separate.ErrNilOption},
		{"even harmonic kernel", separate.WithKernels(16, 17), separate.ErrInvalidKernel},
		{"zero percussive kernel", separate.WithKernels(17, 0), separate.ErrInvalidKernel},
		{"negative kernel", separate.WithKernels(-3, 3), separate.ErrInvalidKernel},
		{"zero power", separate.WithPower(0), separate.ErrInvalidPower},
		{"negative power", separate.WithPower(-1), separate.ErrInvalidPower},
		{"nan power", separate.WithPower(math.NaN()), separate.ErrInvalidPower},
		{"margin below one", separate.WithMargin(0.5, 2), separate.ErrInvalidMargin},
		{"infinite margin", separate.WithMargin(2, math.Inf(1)), separate.ErrInvalidMargin},
		{"nan margin", separate.WithMargin(math.NaN(), 2), separate.ErrInvalidMargin},
		{"bad nfft", separate.WithSTFT(1, 1), separate.ErrInvalidSTFT},
		{"bad stft option", separate.WithSTFT(256, 64, nil), separate.ErrInvalidSTFT},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h, err := separate.NewHPSS(tc.opt)
			if !errors.Is(err, tc.want) || h != nil {
				t.Fatalf("got %v, %v; want %v", h, err, tc.want)
			}
		})
	}

	_, err := separate.NewHPSS(separate.WithSTFT(1, 1))
	if !errors.Is(err, stft.ErrInvalidSize) {
		t.Errorf("STFT error not wrapped: %v", err)
	}
}

func TestCentreOptionErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opt  separate.CentreOption
		want error
	}{
		{"nil", nil, separate.ErrNilOption},
		{"zero exponent", separate.WithCentreExponent(0), separate.ErrInvalidExponent},
		{"infinite exponent", separate.WithCentreExponent(math.Inf(1)), separate.ErrInvalidExponent},
		{"nan exponent", separate.WithCentreExponent(math.NaN()), separate.ErrInvalidExponent},
		{"bad hop", separate.WithCentreSTFT(256, 0), separate.ErrInvalidSTFT},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c, err := separate.NewCentreExtractor(tc.opt)
			if !errors.Is(err, tc.want) || c != nil {
				t.Fatalf("got %v, %v; want %v", c, err, tc.want)
			}
		})
	}
}

func TestHPSSOptionsApplied(t *testing.T) {
	t.Parallel()

	h, err := separate.NewHPSS(separate.WithSTFT(1024, 256, stft.WithNormalized()), separate.WithKernels(3, 5))
	if err != nil {
		t.Fatal(err)
	}

	s := h.STFT()
	if s.NFFT() != 1024 || s.Hop() != 256 || !s.Normalized() || h.HasMargin() {
		t.Fatalf("unexpected settings: nfft %d hop %d normalized %v margin %v",
			s.NFFT(), s.Hop(), s.Normalized(), h.HasMargin())
	}

	d, err := separate.NewHPSS()
	if err != nil {
		t.Fatal(err)
	}

	if d.STFT().NFFT() != separate.DefaultNFFT || d.STFT().Hop() != separate.DefaultHop {
		t.Fatal("unexpected default STFT")
	}
}
