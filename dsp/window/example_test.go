package window

import "fmt"

func ExampleGenerate() {
	w := Generate(TypeHann, 4)
	fmt.Printf("%.2f %.2f %.2f %.2f\n", w[0], w[1], w[2], w[3])
	// Output:
	// 0.00 0.75 0.75 0.00
}

func ExampleApply() {
	buf := []float64{1, 1, 1, 1}
	Apply(TypeHann, buf)
	fmt.Printf("%.2f %.2f %.2f %.2f\n", buf[0], buf[1], buf[2], buf[3])
	// Output:
	// 0.00 0.75 0.75 0.00
}

func ExampleInfo() {
	m := Info(TypeHann)
	fmt.Printf("%s %.1f\n", m.Name, m.ENBW)
	// Output:
	// Hann 1.5
}
