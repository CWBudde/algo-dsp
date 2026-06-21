package effects

import "testing"

func BenchmarkDistortionTanhExact(b *testing.B) {
	d, _ := NewDistortion(48000,
		WithDistortionMode(DistortionModeTanh),
		WithDistortionApproxMode(DistortionApproxExact),
		WithDistortionDrive(3),
	)

	x := 0.1

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		x = d.ProcessSample(x)
	}

	_ = x
}

func BenchmarkDistortionTanhPolynomial(b *testing.B) {
	d, _ := NewDistortion(48000,
		WithDistortionMode(DistortionModeTanh),
		WithDistortionApproxMode(DistortionApproxPolynomial),
		WithDistortionDrive(3),
	)

	x := 0.1

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		x = d.ProcessSample(x)
	}

	_ = x
}
