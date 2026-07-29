package shelving

import "testing"

func BenchmarkButterworthLowShelf(b *testing.B) {
	for b.Loop() {
		if _, err := ButterworthLowShelf(testSR, 1000, 12, 6); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkChebyshev1LowShelf(b *testing.B) {
	for b.Loop() {
		if _, err := Chebyshev1LowShelf(testSR, 1000, 12, 0.5, 6); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkChebyshev2LowShelf(b *testing.B) {
	for b.Loop() {
		if _, err := Chebyshev2LowShelf(testSR, 1000, 12, 0.5, 6); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEllipticLowShelf(b *testing.B) {
	for b.Loop() {
		if _, err := EllipticLowShelf(testSR, 1000, 12, 0.5, 6); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEllipticHighShelf(b *testing.B) {
	for b.Loop() {
		if _, err := EllipticHighShelf(testSR, 1000, 12, 0.5, 6); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEllipticLowShelfOrders(b *testing.B) {
	for _, order := range []int{2, 4, 8, 12} {
		b.Run(orderName(order), func(b *testing.B) {
			for b.Loop() {
				if _, err := EllipticLowShelf(testSR, 1000, 12, 0.5, order); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
