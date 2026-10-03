# Effect graphs

`DefaultRegistry` provides 51 reusable effect factories. `DefaultDescriptors`
returns their canonical parameter names, bounds at the requested sample rate,
units, enumeration values and independent factory preset maps. Mono effects
have independent state per planar channel. Widener, rotary, panner, Haas and
crosstalk process each adjacent stereo pair without folding either channel.

Load the existing JSON graph schema, then call `PreparePlanar(channels, maxFrames)`
before rendering. `ProcessPlanar` rejects unequal lengths and oversized blocks
before advancing state. Built-in prepared processing and `ResetProcessing` do
not allocate, including FFT scheduling, overlap searches and multiband buffers.
`Prepare` provides the corresponding mono preparation path.

The default filter designer supports RBJ, Butterworth, Bessel, Chebyshev I/II
and elliptic designs. `q` means Q for RBJ; legacy graphs may also use it for
high-order peak bandwidth or equiripple shape. New graphs should use
`bandwidthHz` for high-order peaks, `rippleDB` for Chebyshev I/elliptic passes,
and `stopbandDB` for Chebyshev II/elliptic passes. Unsupported family/kind
combinations normalize to RBJ. Parametric EQ exposes one to eight bell/shelf
bands; graphic EQ exposes ten Butterworth bands and even orders four to twelve.
`Response` returns actual complex-cascade magnitudes for linear filter/EQ
graphs and a coefficient snapshot for dynamic EQ. `Transfer` samples the actual static dynamics gain computers in dBFS;
neither API advances processing state.

Time-domain pitch uses streaming WSOLA plus fractional Hermite resampling.
Spectral pitch uses streaming phase-vocoder bin remapping; spectral freeze
captures a complete initial frame. Their sample clocks and overlap histories
survive arbitrary block boundaries. The existing standalone one-shot pitch and
freeze APIs remain available and retain their original whole-input semantics.

`Latency` reports algorithmic buffering, excluding musical delay, Haas delay,
FDN pre-delay and leading zeros contained in an impulse response. Technical
delays on parallel graph branches are aligned before summation. Hosts rendering
immutable source can discard the startup latency and feed zero padding after
the selected source to recover the selected-duration output. At 48 kHz,
default time-domain pitch at +12 semitones needs 3368 samples of lookahead;
identity pitch needs none. Default spectral pitch/freeze need 1024 samples.
All of these costs remain bounded by configured window/search sizes.

Convolution requires a supplied `IRProvider`, a nonempty finite mono/stereo IR,
equal channel lengths and exactly the context sample rate. Missing IRs and
rate mismatches are errors. Adjacent source channels use corresponding stereo
IR channels. Hosts packing a channel selection should supply
`WithSourceChannelMap` with the original physical channel indices so a right-only
selection or nonadjacent channels keep their corresponding IR sides. The dry path shares the convolution engine's technical delay.
IR normalization, conversion or file decoding belongs to the caller.

Before constructing a graph, `EstimateWorkspace` with `WithWorkspaceFrames`
and `WithWorkspaceIRFrames` provides an upper bound for cold runtime and
prepared storage, including constructor scratch and graph-delay alignment.
IR lengths are per channel; the channel count scales the complete bound.
Custom registries require their own memory contract.

`TryUpdateGraph` preserves unchanged runtimes and convolution tails while
staging changed zero-latency effects privately. Changed effects start fresh;
convolution wet-only changes retain all histories. Topology, bypass, IR and
buffered-effect changes return false so the host can replace the whole graph.
Control updates and rendering must be serialized by the host.
