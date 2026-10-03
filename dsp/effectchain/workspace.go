package effectchain

import (
	"fmt"
	"math"
)

type workspaceConfig struct {
	frames   int
	irFrames map[int]int
}

// WorkspaceOption supplies preparation and impulse-response geometry for a
// constructor-free storage estimate.
type WorkspaceOption func(*workspaceConfig)

// WithWorkspaceFrames sets the maximum prepared block length (default 256).
func WithWorkspaceFrames(maxFrames int) WorkspaceOption {
	return func(c *workspaceConfig) { c.frames = maxFrames }
}

// WithWorkspaceIRFrames supplies irIndex-to-frame-count bounds per IR channel.
// Mono and stereo IRs must have equal per-channel lengths. Convolution graphs
// require a positive bound for every referenced IR. The map is copied on use.
func WithWorkspaceIRFrames(frames map[int]int) WorkspaceOption {
	return func(c *workspaceConfig) {
		c.irFrames = make(map[int]int, len(frames))
		for id, count := range frames {
			c.irFrames[id] = count
		}
	}
}

// EstimateWorkspace returns a conservative byte budget for built-in graph
// construction and prepared planar rendering, without constructing processors,
// FFT plans, delay histories or IR spectra. It includes all registered node
// instances (even bypassed/disconnected ones), per-channel routing/scratch,
// power-of-two streaming capacities, convolution partitions, RMS windows,
// and compensation on parallel paths. Fixed per-node margins cover small
// objects/maps and coefficient design scratch; rebuild margins cover temporary
// constructor/configuration buffers. The caller's source, IR samples, output,
// presets and application storage are excluded. Custom factories are unsupported.
func EstimateWorkspace(ctx Context, jsonGraph string, channels int, opts ...WorkspaceOption) (int64, error) {
	if !isFiniteWorkspace(ctx.SampleRate) || ctx.SampleRate <= 0 || ctx.SampleRate > 1e9 || channels < 1 || channels > 8 {
		return 0, fmt.Errorf("effectchain: invalid workspace context/channel count")
	}

	cfg := workspaceConfig{frames: 256}

	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	if cfg.frames < 1 || cfg.frames > 1<<20 {
		return 0, fmt.Errorf("effectchain: invalid workspace block size")
	}

	graph, err := parseGraph(jsonGraph)
	if err != nil {
		return 0, fmt.Errorf("effectchain: estimate graph: %w", err)
	}

	if !hasRequiredIONodes(graph) {
		return 0, fmt.Errorf("effectchain: workspace requires valid graph I/O")
	}

	known := map[string]bool{}
	for _, id := range DefaultRegistry().Types() {
		known[id] = true
	}

	bytes := (3*int64(len(graph.Nodes))+2)*int64(cfg.frames)*8 + 65536

	latencies := make(map[string]int64, len(graph.Nodes))
	for _, id := range graph.Order {
		node := graph.Nodes[id]

		before := int64(0)
		for _, edge := range graph.Incoming[id] {
			before = max(before, latencies[edge.From])
		}

		for _, edge := range graph.Incoming[id] {
			if difference := before - latencies[edge.From]; difference > 0 {
				bytes += (difference+int64(cfg.frames))*8 + 256
			}
		}

		latencies[id] = before

		if isStructuralNodeType(node.Type) {
			continue
		}

		if !known[node.Type] {
			return 0, fmt.Errorf("effectchain: workspace unsupported effect %q", node.Type)
		}

		storage, latency, err := effectWorkspace(ctx.SampleRate, node, cfg)
		if err != nil {
			return 0, err
		}

		bytes += storage + 65536

		if !node.Bypassed {
			latencies[id] += latency
		}
	}

	if bytes > math.MaxInt64/int64(channels) {
		return 0, fmt.Errorf("effectchain: workspace estimate overflow")
	}

	return bytes * int64(channels), nil
}

func isFiniteWorkspace(v float64) bool             { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func workspaceSamples(seconds, rate float64) int64 { return int64(math.Ceil(seconds*rate)) + 8 }
func workspacePower2(n int64) int64 {
	result := int64(1)
	for result < n {
		result *= 2
	}

	return result
}

//nolint:funlen,cyclop
func effectWorkspace(rate float64, p Params, cfg workspaceConfig) (int64, int64, error) {
	frames := int64(cfg.frames)
	storage, latency := int64(0), int64(0)

	switch p.Type {
	case "delay":
		storage = 16 * workspaceSamples(2, rate)
	case "delay-simple":
		storage = 16 * workspaceSamples(clamp(p.GetNum("delayMs", 20), 0, 500)*0.001, rate)
	case "chorus":
		storage = 32 * workspaceSamples(0.018+max(clamp(p.GetNum("depth", 0.003), 0, 0.01), 0.003), max(rate, 44100))
	case "flanger":
		storage = 32 * workspaceSamples(0.01, rate)
	case "rotary":
		storage = 32*workspaceSamples(0.03, max(rate, 44100)) + 8*frames
	case "widener":
		storage = 16*workspaceSamples(0.001, rate) + 8*frames
	case "haas":
		storage = 32 * workspaceSamples(0.04, rate)
	case "crosstalk":
		storage = 64 * workspaceSamples(0.01, rate)
	case "vocoder":
		storage = 8 * frames
	case "granular":
		storage = 32 * workspaceSamples(2+max(clamp(p.GetNum("grainSeconds", 0.08), 0.005, 0.5), 0.08), rate)
	case "reverb-freeverb":
		storage = 256 << 10
	case "reverb-fdn", "reverb":
		// Eight lines total 18620 samples at 44100 Hz, plus modulation
		// headroom and the independently allocated pre-delay line.
		storage = 16 * workspaceSamples(18620.0/44100+8*max(clamp(p.GetNum("modDepth", 0.002), 0, 0.01), 0.002)+max(clamp(p.GetNum("preDelay", 0.01), 0, 0.1), 0.01), rate)
		if p.Type == "reverb" {
			storage += 256 << 10
		}
	case "pitch-time":
		sequenceMs := clamp(p.GetNum("sequence", 40), 20, 120)

		overlapMs := clamp(p.GetNum("overlap", 10), 4, 60)
		if overlapMs >= sequenceMs {
			overlapMs = sequenceMs - 1
		}

		sequence := max(int64(math.Round(sequenceMs*0.001*rate)), 32)
		overlap := max(int64(math.Round(overlapMs*0.001*rate)), 8)
		search := max(int64(math.Round(clamp(p.GetNum("search", 15), 2, 40)*0.001*rate)), 1)

		if overlap >= sequence || sequence-overlap < 4 {
			return 0, 0, fmt.Errorf("effectchain: invalid time-pitch workspace geometry")
		}

		ratio := math.Exp2(clamp(p.GetNum("semitones", 0), -24, 24) / 12)
		latency = sequence + search + int64(math.Ceil(float64(sequence-overlap)/ratio)) + 8
		input := workspacePower2(4*(sequence+search) + 64)
		output := workspacePower2(4*(latency+sequence+search) + 64)
		storage = 8*(input+output+2*overlap+2*search) + 80*max(overlap, int64(math.Ceil(0.01*rate)))

		if math.Abs(ratio-1) <= 1e-9 {
			latency = 0
		}
	case "pitch-spectral", "spectral-freeze":
		frame := int64(sanitizeSpectralPitchFrameSize(int(math.Round(p.GetNum("frameSize", 1024)))))
		// Both constructor and Configure allocate FFT state before the
		// streaming overlap-add history is prepared. Include these cold
		// allocations even when old plans are subsequently released.
		storage = 640 * max(frame, 1024)

		latency = frame
		if p.Type == "pitch-spectral" && math.Abs(p.GetNum("semitones", 0)) <= 1e-9 {
			latency = 0
		}
	case "reverb-conv":
		index := int(p.GetNum("irIndex", 0))

		irFrames := int64(cfg.irFrames[index])
		if irFrames < 1 || irFrames > 1<<40 {
			return 0, 0, fmt.Errorf("effectchain: convolution IR %d requires a bounded workspace frame count", index)
		}

		padded := ((irFrames + 127) / 128) * 128

		fftSum := int64(0)
		for block := int64(128); block <= 8192; block *= 2 {
			fftSum += 2 * block
			if block >= padded {
				break
			}
		}

		// IR spectra and retained input-frequency history both scale with
		// IR length. Include cold construction and the output accumulator.
		storage = 96*padded + 256*fftSum + 8*(frames+128)
		latency = 128
	case "dyn-expander":
		storage = 16 * workspaceSamples(max(clamp(p.GetNum("rmsWindowMs", 30), 1, 1000)*0.001, 0.03), rate)
	case "dyn-compressor", "dyn-limiter", "dyn-gate":
		storage = 16 * workspaceSamples(0.03, rate)
	case "dyn-lookahead":
		storage = 16 * workspaceSamples(0.03+clamp(p.GetNum("lookaheadMs", 3), 0, 200)*0.001, rate)
		latency = int64(math.Round(clamp(p.GetNum("lookaheadMs", 3), 0, 200) * 0.001 * rate))
	case "dyn-multiband":
		count := int64(clamp(math.Round(p.GetNum("bands", 3)), 2, 3))
		storage = 16*count*workspaceSamples(0.03, rate) + 8*count*frames
	case "dyn-eq":
		count := int64(clamp(math.Round(p.GetNum("bands", 4)), 1, 8))
		storage = 16 * count * workspaceSamples(0.03, rate)
	}

	return storage, latency, nil
}
