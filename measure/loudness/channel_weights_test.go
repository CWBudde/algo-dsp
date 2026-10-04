package loudness_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/cwbudde/algo-dsp/measure/loudness"
)

func TestBS1770ChannelWeights(t *testing.T) {
	for _, tc := range []struct {
		channels int
		selected []int
		want     []float64
	}{
		{1, nil, []float64{1}},
		{2, nil, []float64{1, 1}},
		{5, nil, []float64{1, 1, 1, 1.41, 1.41}},
		{6, nil, []float64{1, 1, 1, 0, 1.41, 1.41}},
		{6, []int{5, 3, 0}, []float64{1.41, 0, 1}},
		{6, []int{3}, []float64{0}},
		{8, nil, []float64{1, 1, 1, 1, 1, 1, 1, 1}},
	} {
		got, err := loudness.BS1770ChannelWeights(tc.channels, tc.selected)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%+v: %v, %v", tc, got, err)
		}

		got[0] = 99

		again, _ := loudness.BS1770ChannelWeights(tc.channels, tc.selected)
		if !reflect.DeepEqual(again, tc.want) {
			t.Fatal("aliased weights")
		}
	}

	for _, tc := range []struct {
		channels int
		selected []int
	}{{0, nil}, {9, nil}, {2, []int{}}, {2, []int{-1}}, {2, []int{2}}, {2, []int{0, 0}}} {
		if _, err := loudness.BS1770ChannelWeights(tc.channels, tc.selected); !errors.Is(err, loudness.ErrInvalid) {
			t.Fatalf("%+v: %v", tc, err)
		}
	}
}

func ExampleBS1770ChannelWeights() {
	weights, _ := loudness.BS1770ChannelWeights(6, []int{0, 3, 4})
	fmt.Println(weights)
	// Output: [1 0 1.41]
}
