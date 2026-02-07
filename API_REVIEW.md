# API Review Checklist (Phase 14)

Date: 2026-02-07
Scope: `github.com/cwbudde/algo-dsp` public packages

## 14.1 API Review

- [x] Review exported types/functions/methods for consistency across `dsp/*`, `measure/*`, `stats/*`.
- [x] Verify naming follows Go conventions (MixedCaps, no package stutter).
- [x] Verify public symbols have doc comments.
- [x] Identify unnecessary exports and remove obvious dead/test-only helpers.
- [x] Validate error semantics and wrapping strategy in public APIs.
- [x] Review option patterns (`core.ProcessorOption`, package-local options) for forward compatibility.

## 14.2 Decisions and Notes

- `dsp/core` is kept as the shared configuration contract for package-level generators/processors.
- Odd-order Chebyshev first-order sections remain deterministic compatibility behavior (fallback to Butterworth first-order sections) and are explicitly documented in code comments.
- Benchmarks now check returned errors so lint rules match release quality gates.
- Public-code TODO/FIXME markers were removed from implementation comments.

## Remaining v1.0 gates

- [ ] Run and archive full benchmark baseline for the release commit.
- [ ] Final release validation in CI: `go test -race ./...`, lint, vet.
- [ ] Tag `v1.0.0` after changelog freeze.
