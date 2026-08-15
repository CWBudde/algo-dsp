# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`algo-dsp` is a reusable, production-quality DSP (Digital Signal Processing) algorithm library written in Go. This library is intentionally separated from application concerns, file I/O, and UI components to remain focused on algorithmic implementations.

**Module**: `github.com/cwbudde/algo-dsp`

**Related Repositories**:

- `github.com/cwbudde/algo-fft`: FFT backend (consumed as dependency, not duplicated, see ../algo-fft)
- `github.com/cwbudde/wav`: WAV container support (separate concern) (see ../wav)
- `github.com/cwbudde/mfw`: Application that will consume this library (see ../mfw)

**Web Demo**: [https://cwbudde.github.io/algo-dsp/](https://cwbudde.github.io/algo-dsp/)

## Development Commands

### Setup and Build

```bash
# Initialize module (if not already done)
go mod init github.com/cwbudde/algo-dsp

# Build the project
go build ./...

# Install justfile commands (once justfile is created)
just test      # Run all tests
just lint      # Run linters
just lint-fix  # Run linters with auto-fix
just fmt       # Format code
just bench     # Run benchmarks
just ci        # Run all CI checks locally
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests with race detector
go test -race ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./dsp/window/
go test ./dsp/filter/biquad/

# Run benchmarks
go test -bench=. ./...
go test -bench=BenchmarkSpecificFunc -benchmem ./path/to/package/
```

### Linting and Formatting

```bash
# Format code
just fmt

# Run golangci-lint (once configured)
just lint

# Run with auto-fix where possible
just lint-fix
```

## Architecture and Package Structure

The library is organized into focused packages with clear boundaries:

### Core DSP Packages (`dsp/`)

- **`window/`**: Window functions (Hann, Hamming, Blackman, Kaiser, etc.) with coefficient generators
- **`filter/`**: Filter implementations
  - `biquad/`: Biquad runtime and cascaded chains
  - `fir/`: FIR filter runtime (direct form + partitioned convolution)
  - `design/`: Filter design calculators (Butterworth, Chebyshev, parametric EQ)
  - `bank/`: Octave and fractional-octave filter banks
  - `weighting/`: A/B/C/Z frequency weighting filters
- **`spectrum/`**: Spectral analysis utilities (magnitude, phase, group delay, smoothing)
- **`conv/`**: Convolution, deconvolution, and correlation (direct, overlap-add, overlap-save)
- **`resample/`**: Sample rate conversion with anti-aliasing
- **`signal/`**: Signal generators (sine, noise, sweep) and utilities (normalize, envelope)
- **`effects/`**: Optional algorithmic audio effects

### Measurement Packages (`measure/`)

- **`thd/`**: THD (Total Harmonic Distortion) and THD+N analysis kernels
- **`sweep/`**: Log sweep generation and deconvolution for impulse response measurement
- **`ir/`**: Impulse response metrics (RT60, EDT, C50, C80, D50, center time)

### Statistics Packages (`stats/`)

- **`time/`**: Time-domain statistics (RMS, crest factor, zero crossings, moments)
- **`frequency/`**: Frequency-domain statistics (spectral centroid, flatness, bandwidth)

### Internal Packages (`internal/`)

- **`testutil/`**: Test fixtures, reference vectors, tolerance helpers, deterministic test signals
- **`simd/`**: Optional SIMD/architecture-specific optimizations (with scalar fallbacks)
- **`unsafeopt/`**: Isolated low-level unsafe optimizations when justified

## API Design Principles

When implementing or modifying APIs in this library:

1. **Small interfaces, concrete constructors**: Prefer explicit types with option patterns

   ```go
   func NewProcessor(opts ...Option) (*Processor, error)
   ```

2. **Streaming-friendly processing**: Support both one-shot and reusable processing patterns

   ```go
   func Process(input []float64) ([]float64, error)  // One-shot
   func (p *Processor) ProcessInPlace(buf []float64) error  // Reusable, zero-alloc
   ```

3. **Deterministic behavior**: Same input + options = same output (critical for testing)

4. **Zero-allocation fast paths**: Provide in-place variants for hot paths in real-time scenarios

5. **Clear error semantics**: Use `fmt.Errorf("context: %w", err)` for error wrapping

6. **Documentation requirements**: All public types and functions require doc comments and runnable examples

7. **Pragmatic generics**: Use generics only when they provide clear value without API complexity

## Numerical Correctness and Validation

### Testing Strategy

- Use **table-driven tests** for comprehensive coverage of edge cases
- Implement **golden vector tests** comparing outputs against trusted references (MATLAB, NumPy, known datasets)
- Apply **property-based testing** for algorithmic invariants
- Define **tolerance policies** per algorithm category (floating-point comparisons)
- Track expected numerical drift across different architectures

### Coverage Targets

- **Core algorithm packages**: ≥ 90% coverage
- **Project-wide**: ≥ 85% coverage

### Benchmarking

- Maintain microbenchmarks for all hot paths
- Track allocations/op and bytes/op as first-class metrics
- Document algorithm selection thresholds (e.g., when overlap-add beats direct convolution)
- Benchmark quality/performance trade-offs for configurable algorithms

## Boundary Rules (Critical)

This library must remain **algorithm-centric and transport-agnostic**:

### ❌ Prohibited Dependencies

- No Wails, React, or any UI framework
- No desktop runtime packages (ASIO, CoreAudio, JACK, PortAudio)
- No file format codecs (WAV, AIFF, FLAC) - use `github.com/cwbudde/wav` separately
- No application-specific logging or configuration frameworks
- No app orchestration or state management

### ✅ Allowed Dependencies

- `github.com/cwbudde/algo-fft` for FFT operations (consumed via narrow interfaces)
- Standard library packages
- Minimal, justified external dependencies (document rationale)

### Integration Philosophy

- Keep FFT integration as **interface contracts** only - no FFT implementation duplication
- The library should work equally well in CLI tools, desktop apps, servers, or embedded contexts

## Performance and Optimization

### Optimization Strategy (Phase 24)

- Begin with **correct, readable implementations**
- Use **profiling data** to identify actual bottlenecks before optimizing
- Maintain **scalar reference implementations** as source of truth
- Add architecture-specific optimizations behind **build tags**
- Require **numerical parity tests** between optimized and reference paths
- Measure and document performance gains with benchmarks

### Real-Time Considerations

- Minimize allocations in processing hot paths
- Provide pre-allocated buffer reuse patterns
- Support block-based processing with predictable latency
- Document worst-case allocation behavior

## Migration from `mfw`

When extracting code from the existing `mfw` application:

1. **Extraction order** (as defined in PLAN.md Phase 11):
   - Window functions first
   - Filter primitives and design
   - Spectrum/convolution/resampling helpers
   - Measurement kernels last

2. **Migration mechanics**:
   - Move code **with tests** in atomic commits
   - Add **compatibility tests** in `mfw` to validate behavior parity
   - Switch imports in `mfw` only after parity verification
   - Remove duplicated code only when both repos' CI passes

3. **Adapter pattern**: Keep `mfw` APIs adapter-friendly during transition

## Versioning and Releases

- **Semantic versioning**: Major.Minor.Patch
- **Pre-release phase**: `v0.x` until API stabilization (Phase 25)
- **Stability guarantee**: `v1.0.0` freezes public API with backward compatibility commitment
- **Go version support**: Latest stable + previous stable Go versions
- **Conventional commits**: Required for automated changelog generation

## Development Phases

Refer to [PLAN.md](PLAN.md) for the complete phase roadmap. Current phase completion should be tracked in issue/PR descriptions.

**Initial 90-day focus**:

- Month 1: Bootstrap (Phase 0-1) + Window functions (Phase 2)
- Month 2: Filter runtimes (Phase 3) + start filter design (Phase 4)
- Month 3: Complete filter design + weighting/banks (Phase 5) + spectrum utilities (Phase 6)

## Code Quality Standards

### Required for all packages

- Passing `golangci-lint` (configuration in `.golangci.yml`)
- Test coverage meeting package-specific targets
- Runnable examples in package documentation
- Benchmark presence for performance-critical paths

### Before merging to main

- All tests pass with race detector
- Benchmark sanity checks (no major regressions)
- Documentation updated for API changes
- Examples build and run successfully

## Releasing, and Not Drifting

This module is part of the `github.com/cwbudde/algo-*` family, which is co-developed
across separate repositories. That arrangement failed once already, and the rules below
exist to stop it failing the same way twice.

**What went wrong (August 2026).** The family had drifted onto three different `algo-fft`
versions simultaneously — `algo-pde` on v0.6.15, `algo-dsp` on v0.7.3, `algo-acoustics` on
v0.6.11 — while `algo-fft`'s own `main` sat 97 commits past its latest tag and its
CHANGELOG documented a `v0.7.5` that had never been tagged. Because `algo-fft`'s generic
`PlanReal2D`/`PlanReal3D` had changed signature between the v0.6 and v0.7 lines, _no single
upgrade anywhere would compile_. Untangling it took a day and four coordinated releases.

Three separate mistakes combined to produce that. Each now has a check.

### 1. Do not let work pile up untagged

Work that only exists on `main` cannot be consumed. If you finish something a sibling repo
needs, tag it — do not wait for a milestone.

```bash
just check-unreleased     # how much is sitting past the latest tag?
```

A scheduled CI job (`.github/workflows/dep-drift.yml`) reports this weekly.

### 2. Do not sit on stale siblings

```bash
just check-deps           # are all github.com/cwbudde/* deps at their latest tags?
```

This is wired into the repo's aggregate check recipe, and the same scheduled job files a
GitHub issue when it starts failing. If a bump is _deliberately_ deferred, write down why in
`PLAN.md` — an undocumented old pin is indistinguishable from an forgotten one.

Renovate (`.github/renovate.json`) opens the bump PRs automatically and groups the whole
`cwbudde` family into a single PR on purpose: an incompatible `algo-fft` can reach a
consumer through two different dependency paths at once, so bumping them one PR at a time
produces intermediate combinations that never build.

### 3. Never remove or change exported API without the version saying so

Always release through the guard rather than by hand:

```bash
just tag-release v0.8.0       # runs every precondition, then tags and pushes
```

It refuses to tag when the tree is dirty, when `HEAD` is not a pushed default branch, when the tag
already exists or does not sort after the current one, when siblings are stale, when
`CHANGELOG.md` has no section for the version, or when the exported API changed
incompatibly without the version reflecting it.

**That last rule is stricter than semver, deliberately.** Semver exempts `v0.x` — "anything
MAY change at any time" — so `gorelease` will happily approve a _patch_ bump across a
removed symbol. Every module in this family is `v0.x`, so that exemption is exactly the hole
we fell through: `KernelEightStep` was removed and `PlanReal2D` became generic, and nothing
in the version numbers said so. The guard therefore requires a **minor** bump for any
incompatible change while on `v0.x`.

When you do break API, say so in the CHANGELOG in the form a consumer needs: the old
signature, the new signature, and the call-site rewrite. "Refactored plans" does not help
anyone; `NewPlanReal2D(rows, cols)` → `NewPlanReal2D64(rows, cols)` does.

### Order of operations for a cross-repo change

Releases must flow up the dependency graph, never sideways:

```
algo-vecmath ─┐
algo-approx  ─┼─→ algo-dsp ─┐
algo-fft ─────┴─→ algo-pde ─┴─→ algo-acoustics
```

Tag the dependency first, then bump and tag its consumers, then the consumers' consumers.
Bumping a consumer before its dependency is tagged is what forces pseudo-versions into
`go.mod`, and those are how a repo quietly ends up pinned to a commit nobody can find later.
