# Migration Guide

## Upgrading to v1.0.0 from v0.x

This document tracks migration notes for prerelease users moving to the stabilized v1 API.

## Compatibility goals

- Public package paths remain unchanged.
- Existing core algorithm entry points remain source-compatible.
- Error-returning APIs keep explicit error values for invalid parameters and empty inputs.

## Notable behavioral clarifications

- `measure/ir` and `measure/sweep` benchmark and example paths now consistently treat algorithm APIs as error-returning calls.
- `dsp/filter/design` Chebyshev odd-order first-order section fallback behavior is explicitly documented as a deterministic compatibility choice.

## Recommended upgrade checklist

1. Update module dependency to `v1.0.0`.
2. Run `go test ./...` in downstream projects.
3. Re-run any local benchmark baselines that depend on project-specific workloads.
4. Verify custom wrappers around `measure/*` and `stats/*` still propagate errors.

## Deprecated or removed symbols

No public API removals are currently planned for `v1.0.0`.

If removals are introduced before the tag is cut, they must be listed here with one-line replacement guidance.
