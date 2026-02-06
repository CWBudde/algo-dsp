# algo-dsp

Reusable DSP algorithms for Go.

## Status

Early scaffold. See `PLAN.md` for the implementation roadmap.

## Scope

- DSP algorithms and measurement kernels
- No UI/runtime/application orchestration
- No audio container/file codecs

## Planned Package Areas

- `dsp/window`
- `dsp/filter/*`
- `dsp/spectrum`
- `dsp/conv`
- `dsp/resample`
- `dsp/signal`
- `measure/*`
- `stats/*`

## Development

Requirements:

- Go 1.25+
- `just` (optional)

Common commands:

- `just test`
- `just lint`
- `just format`
- `just ci`
