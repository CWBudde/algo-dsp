package design_test

import (
	"context"
	"fmt"
	"time"

	"github.com/cwbudde/algo-dsp/dsp/dither/design"
)

func ExampleDesigner() {
	designer, err := design.NewDesigner(44100,
		design.WithOrder(5),
		design.WithIterations(1000),
		design.WithSeed(42),
	)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	coeffs, err := designer.Run(ctx)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Found %d coefficients\n", len(coeffs))
	// Output: Found 5 coefficients
}
