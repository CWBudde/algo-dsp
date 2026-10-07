package effectchain

import (
	"fmt"
	"math"
	"math/cmplx"

	"github.com/cwbudde/algo-dsp/dsp/effects/dynamics"
	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

func runtimeBiquads(runtime Runtime) *biquad.Chain {
	switch r := runtime.(type) {
	case *filterRuntime:
		if r.moogLP == nil {
			return r.fx
		}
	case *equalizerRuntime:
		return r.fx
	}

	return nil
}

// Response evaluates the complex transfer of a graph composed of linear
// biquad EQ/filter nodes, sums and bypasses, returning linear magnitudes.
// Dynamic EQ uses its current biquad coefficients, providing a snapshot of
// the response rather than predicting future envelope-dependent gains.
// Unsupported nonlinear, time-varying or frequency-split nodes return errors.
// It does not process audio, advance filter history, or alter prepared storage.
func (c *Chain) Response(frequencies []float64) ([]float64, error) {
	if !c.HasGraph() {
		return nil, fmt.Errorf("effectchain: response requires a graph")
	}

	for _, hz := range frequencies {
		if math.IsNaN(hz) || math.IsInf(hz, 0) || hz < 0 || hz > c.ctx.SampleRate*0.5 {
			return nil, fmt.Errorf("effectchain: invalid response frequency %g", hz)
		}
	}

	for id, node := range c.graph.Nodes {
		if id == InputNodeID || id == OutputNodeID || node.Bypassed || node.Type == "split" || node.Type == "sum" {
			continue
		}

		if _, dynamic := c.NodeRuntime(id).(*dynamicEQRuntime); !dynamic && runtimeBiquads(c.NodeRuntime(id)) == nil {
			return nil, fmt.Errorf("effectchain: response unsupported for node %q (%s)", id, node.Type)
		}
	}

	values := make(map[string]complex128, len(c.graph.Nodes))

	output := make([]float64, len(frequencies))
	for i, hz := range frequencies {
		values[InputNodeID] = 1
		for _, id := range c.graph.Order {
			if id == InputNodeID {
				continue
			}

			node := c.graph.Nodes[id]
			parents := c.graph.MainIncoming[id]

			value := complex(0, 0)
			for _, edge := range parents {
				value += values[edge.From]
			}

			if len(parents) > 0 {
				value /= complex(float64(len(parents)), 0)
			}

			if !node.Bypassed {
				if fx := runtimeBiquads(c.NodeRuntime(id)); fx != nil {
					value *= complex(fx.Gain(), 0)
					for section := range fx.NumSections() {
						value *= fx.Section(section).Response(hz, c.ctx.SampleRate)
					}
				}

				if eq, ok := c.NodeRuntime(id).(*dynamicEQRuntime); ok {
					for band := 0; band < eq.count; band++ {
						coeff, err := eq.fx.BandCoefficients(band)
						if err != nil {
							return nil, fmt.Errorf("effectchain: dynamic EQ response: %w", err)
						}

						value *= coeff.Response(hz, c.ctx.SampleRate)
					}
				}
			}

			values[id] = value
		}

		output[i] = cmplx.Abs(values[OutputNodeID])
	}

	return output, nil
}

func runtimeCurve(runtime Runtime) dynamics.StaticCurveProcessor {
	switch r := runtime.(type) {
	case *compressorRuntime:
		return r.fx
	case *limiterRuntime:
		return r.fx
	case *lookaheadLimiterRuntime:
		return r.fx
	case *gateRuntime:
		return r.fx
	case *expanderRuntime:
		return r.fx
	case *multibandRuntime:
		if r.fx != nil {
			return r.fx.Band(r.curveBand)
		}
	}

	return nil
}

// Transfer evaluates static input/output dBFS levels for a graph of dynamics
// gain computers, sums and bypasses. It uses each processor's actual gain
// computer without envelope smoothing and does not advance processing state.
// Multiband nodes inspect the gain computer selected by responseBand (default
// zero); this is a band curve, not the combined frequency-dependent response.
func (c *Chain) Transfer(levelsDB []float64) ([]float64, error) {
	if !c.HasGraph() {
		return nil, fmt.Errorf("effectchain: transfer requires a graph")
	}

	for _, db := range levelsDB {
		if math.IsNaN(db) || math.IsInf(db, 0) || db < -1000 || db > 1000 {
			return nil, fmt.Errorf("effectchain: invalid transfer level %g", db)
		}
	}

	for id, node := range c.graph.Nodes {
		if id == InputNodeID || id == OutputNodeID || node.Bypassed || node.Type == "split" || node.Type == "sum" {
			continue
		}

		if runtimeCurve(c.NodeRuntime(id)) == nil {
			return nil, fmt.Errorf("effectchain: transfer unsupported for node %q (%s)", id, node.Type)
		}
	}

	values := make(map[string]float64, len(c.graph.Nodes))

	output := make([]float64, len(levelsDB))
	for i, db := range levelsDB {
		values[InputNodeID] = math.Pow(10, db/20)
		for _, id := range c.graph.Order {
			if id == InputNodeID {
				continue
			}

			node := c.graph.Nodes[id]
			parents := c.graph.MainIncoming[id]

			value := 0.0
			for _, edge := range parents {
				value += values[edge.From]
			}

			if len(parents) > 0 {
				value /= float64(len(parents))
			}

			if !node.Bypassed {
				if fx := runtimeCurve(c.NodeRuntime(id)); fx != nil {
					value = fx.CalculateOutputLevel(value)
				}
			}

			values[id] = value
		}

		output[i] = 20 * math.Log10(math.Max(values[OutputNodeID], math.SmallestNonzeroFloat64))
	}

	return output, nil
}
