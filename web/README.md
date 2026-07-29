# Web Demo

The GitHub Pages demo for `algo-dsp`, hosted at
[https://cwbudde.github.io/algo-dsp/](https://cwbudde.github.io/algo-dsp/).

Its purpose is defined in [PLAN.md](../PLAN.md) (Phase 41b): it is the **showcase and integration
test for the public API**. Every package under `dsp/`, `measure/`, and `stats/` either appears here
or is explicitly listed there as out of scope.

All signal processing runs in Go compiled to WebAssembly. Web Audio only carries the finished PCM
to the output device — no browser DSP nodes are used, so what you hear is the library itself.

## What the demo contains

- **Transport and sequencer** — 16-step pure-tone sequencer with an exponential decay envelope;
  tempo, shuffle, decay, waveform (sine/triangle/saw/square), 11 scales and 12 root notes.
- **EQ** — interactive canvas with five draggable band nodes. Response curves are computed in Go
  via `dsp/filter/design`; families cover RBJ, Butterworth, Bessel, Chebyshev I/II and elliptic,
  across lowpass, highpass, bandpass, notch, allpass, peak and shelving types.
- **Spectrum analyser** — overlaid on the EQ canvas. Configurable FFT size, overlap, window
  (`dsp/window`) and smoothing.
- **Effects chain** — a node-graph editor driving `dsp/effectchain`, with ~40 effect types across
  modulation, saturation/colour, filters, routing, reverb (Freeverb, FDN, convolution), spatial,
  pitch and dynamics.
- **Impulse responses** — the convolution reverb draws on an embedded IR library
  (`internal/webdemo/data/irs.irlib`, compiled into the WASM binary).
- **Output strip** — fixed master compressor and limiter with static characteristic-curve plots.
- Light/dark/system theming, and settings persisted to `localStorage`.

## Layout

| Path                                      | Contents                                                      |
| ----------------------------------------- | ------------------------------------------------------------- |
| `index.html`, `styles.css`                | Markup and styling                                            |
| `app.js`                                  | Application shell: state, DOM wiring, Web Audio, WASM loading |
| `effect-chain.js`                         | Node-graph editor canvas                                      |
| `eq-canvas.js`                            | Interactive EQ + analyser canvas                              |
| `dynamics-graph.js`, `dist-cheb-graph.js` | Small static plots                                            |
| `wasm/main.go`                            | `syscall/js` entry point (`//go:build js && wasm`)            |
| `../internal/webdemo/`                    | Go engine behind the entry point                              |

`algo_dsp_demo.wasm` and `wasm_exec.js` are build artifacts and are not committed.

## Local run

```bash
just web-demo          # builds the WASM assets and serves on :8787
```

Or manually:

```bash
./web/build-wasm.sh
python3 -m http.server 8787 -d web
```

Then open <http://localhost:8787>. Opening `index.html` directly from the filesystem does **not**
work — the WASM module must be fetched over HTTP.

## Deployment

`.github/workflows/pages.yml` builds the WASM assets and publishes `web/` to GitHub Pages on every
push to `main`.

## Known limitations

These are tracked in PLAN.md Phase 41b rather than being accepted as final:

- The audio path uses `ScriptProcessorNode`, which runs on the main thread (Stage 3).
- The engine renders **mono**. Stereo effects (widener, rotary) apply an internal fold-down
  approximation rather than producing a true stereo image.
- The effect-chain editor is mouse-driven and not yet usable on touch devices.
- The interactive canvases are not yet keyboard-accessible.
- `measure/*` and `stats/*` are not yet exercised by the demo.
