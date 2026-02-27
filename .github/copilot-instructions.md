# Copilot Instructions for `algo-dsp`

## What this repo is
- `algo-dsp` is a Go DSP algorithm library (`github.com/cwbudde/algo-dsp`), not an application.
- Keep code algorithm-centric and transport-agnostic: no UI frameworks, no audio-device APIs, no file codecs.
- Main package domains:
  - `dsp/` core algorithms (window/filter/conv/resample/signal/effects/core/buffer)
  - `measure/` analysis kernels (`thd`, `sweep`, `ir`)
  - `stats/` time/frequency descriptors
  - `internal/` support code (`testutil`, SIMD/unsafe internals)

## Architecture and boundaries
- Preserve strict separation between reusable DSP kernels and app/runtime concerns.
- FFT is consumed via dependency (`github.com/cwbudde/algo-fft`); do not duplicate FFT implementations.
- Web UI lives in `web/` as a demo only; algorithm logic belongs in Go packages.

## Code patterns to follow
- Prefer explicit constructors/options patterns (see `dsp/core/options.go`).
- Provide streaming/reusable and in-place paths where relevant (avoid unnecessary allocations).
- Maintain deterministic behavior for identical input/options.
- Follow existing error style with package-scoped sentinels and clear prefixes (see `dsp/conv/conv.go`, e.g. `conv: empty input`).
- Keep scalar/reference behavior correct first; optimizations (SIMD/build tags) must preserve numerical parity.

## Testing and examples
- Add/maintain table-driven tests in the target package.
- Keep runnable examples in `*_test.go` with `Example...` and exact `// Output:` blocks (see `dsp/window/example_test.go`).
- Keep benchmarks near hot paths (`*_bench_test.go`), especially for algorithm thresholds.

## Local workflows (use these commands)
- Format: `just fmt` (uses `treefmt`).
- Lint: `just lint` (or `just lint-fix`).
- Test: `just test`; race: `just test-race`; coverage: `just test-coverage`.
- Benchmarks: `just bench` (or focused `just bench-ci`).
- Full local CI parity: `just ci` (`check-formatted`, `test`, `lint`, `check-tidy`).
- Equivalent raw Go commands are documented in `README.md`.

## Integration points and build tags
- Go version target is `go 1.25` (`go.mod`).
- Core external deps: `algo-fft`, `algo-approx`, `algo-vecmath`.
- Respect existing build-tag modes (e.g. `purego` fallback, optional `fastmath` paths in effects docs).

## Web demo context (do not couple library code to it)
- Demo build/run: `just web-demo` or `./web/build-wasm.sh` + `python3 -m http.server ...`.
- Demo uses Go/WASM outputs but is not the architectural center of the repository.
