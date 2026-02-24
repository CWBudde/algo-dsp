package hilbert

import "fmt"

func ExampleProcessor64_ProcessSample() {
	p, err := New64Default()
	if err != nil {
		panic(err)
	}

	a, b := p.ProcessSample(1)
	fmt.Printf("A=%.6f B=%.6f\n", a, b)
	// Output: A=0.001466 B=0.000000
}

func ExampleProcessor64_ProcessEnvelopeSample() {
	p, err := New64Default()
	if err != nil {
		panic(err)
	}

	env := p.ProcessEnvelopeSample(1)
	fmt.Printf("env=%.6f\n", env)
	// Output: env=0.001466
}
