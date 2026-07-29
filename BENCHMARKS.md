# Benchmarks

This document tracks baseline benchmark results for release readiness.

## Environment

- Date: 2026-02-07
- OS/Arch: `linux/amd64`
- CPU: `12th Gen Intel(R) Core(TM) i7-1255U`
- Command:

```bash
go test -run=^$ -bench=. -benchmem ./...
```

## Regression Guard

`benchmarks/baseline.json` holds machine-readable numbers for a 20-benchmark hot-path
subset, and `cmd/benchguard` compares a fresh run against it.

```bash
just bench-guard                        # compare against the baseline
just bench-guard 3 -fail                # exit non-zero on a regression
just bench-guard 5 "-enforce-timing"    # also gate on ns/op (quiet machines only)
just bench-baseline                     # re-record the baseline on this machine
```

The benchmark set itself is defined once, in the `bench-ci` justfile recipe, and both
CI and local runs drive it from there.

### What gates, and what does not

| Metric      | Tolerance | Gates by default |
| ----------- | --------- | ---------------- |
| `allocs/op` | exact     | yes              |
| `B/op`      | +10%      | yes              |
| `ns/op`     | +50%      | no — reported    |

**Allocation counts are the signal this guard exists to protect.** They are
deterministic for a given input size and identical across machines, core counts, and
thermal states, so a `0 -> 1` on a zero-alloc hot path is always a real defect. `B/op`
gets 10% of headroom for allocator size-class rounding.

**Timing is reported but does not gate**, and that default is a measured conclusion
rather than caution:

- Three repeat runs of the subset on an idle laptop at `-count=1`, with no code change
  at all, moved the sub-microsecond spectrum benchmarks by up to **43%**.
- Raising the repeat count did not rescue it. A `-count=3` run against a `-count=5`
  baseline on the _same machine_ still put five benchmarks past a 50% bound: a
  sustained benchmark sweep heats the CPU, so later entries measure a throttled core.
- Under real desktop load (a browser plus a parallel build) individual entries moved by
  **7x**. In that same run the guard reported _no regressions_, because every
  allocation column held steady — which is exactly the behaviour being aimed for.

No ns/op threshold is simultaneously loose enough to survive that and tight enough to
catch a genuine regression. So `benchguard` prints the delta and marks anything past
the bound `slower (advisory)`, leaving the judgement to a human. On quiet, thermally
stable hardware, `-enforce-timing` promotes it to a gating check; even then it is
suppressed when the baseline's `goos`/`goarch`/`cpu` do not match the current machine.

To reduce noise where it can, `benchguard` keeps the **minimum** across `-count=N`
observations, since interference only ever makes a benchmark look slower.

Two smaller caveats:

- `just bench-baseline` records at `-count=5` while `just bench-guard` compares at
  `-count=3`. A minimum over more samples is slightly lower, so comparisons carry a
  small bias toward "slower".
- CI runs the guard as a non-blocking job, and its baseline comes from developer
  hardware, so only the allocation columns carry a reliable signal there.

> **The checked-in baseline's `ns/op` figures are provisional.** They were captured on a
> workstation under heavy load (load average ~32), and at least
> `dsp/filter/fir.BenchmarkProcessSample/taps=128` is recorded roughly 6x slow as a
> result. Re-record with `just bench-baseline` on a quiet machine. The `B/op` and
> `allocs/op` columns are unaffected by load and are already correct.

### Updating the baseline

Re-record when a change legitimately moves the numbers, and say so in the PR:

1. Run `just bench-baseline` on an idle machine.
2. Commit `benchmarks/baseline.json` together with the change that moved it.
3. Update the prose baselines below if the change is large enough to matter.

Because the file records `goVersion`, `goos`, `goarch` and `cpu`, a baseline recorded
on different hardware is self-describing rather than silently misleading.

## Selected Baselines

These are representative checkpoints from the full benchmark run.

### `dsp/filter/biquad`

- `BenchmarkProcessSample`: `7.867 ns/op`, `0 B/op`, `0 allocs/op`
- `BenchmarkProcessBlock/N=1024`: `7981 ns/op`, `1026.39 MB/s`, `0 allocs/op`

### `dsp/filter/fir`

- `BenchmarkProcessSample/taps=128`: `117.0 ns/op`, `0 allocs/op`
- `BenchmarkProcessBlock/taps=128`: `124720 ns/op`, `65.68 MB/s`, `0 allocs/op`

### `dsp/conv`

- `BenchmarkConvolve/signal=4096_kernel=32`: `117724 ns/op`, `40960 B/op`, `1 allocs/op`
- `BenchmarkOverlapAddReuse/signal=4096_kernel=64`: `232297 ns/op`, `41077 B/op`, `1 allocs/op`

### `dsp/window`

- `BenchmarkGenerate/hann/1024`: `35215 ns/op`, `8256 B/op`, `2 allocs/op`
- `BenchmarkApply/hann/1024`: `38236 ns/op`, `8256 B/op`, `2 allocs/op`

### `measure/sweep`

- `BenchmarkLogSweepGenerate`: `2996461 ns/op`, `385088 B/op`, `1 allocs/op`
- `BenchmarkLogSweepDeconvolve48k`: `31837860 ns/op`, `41353539 B/op`, `123 allocs/op`

### `measure/ir`

- `BenchmarkAnalyze`: `5331844 ns/op`, `1155073 B/op`, `1 allocs/op`
- `BenchmarkRT60`: `3787468 ns/op`, `1155089 B/op`, `1 allocs/op`

### `dsp/spectrum`

Before (3 allocs for re/im unpacking):

- `BenchmarkMagnitude/4K`: `53,000 ns/op`, `98,304 B/op`, `3 allocs/op`
- `BenchmarkPower/4K`: `43,400 ns/op`, `98,304 B/op`, `3 allocs/op`

After (pooled scratch buffers):

- `BenchmarkMagnitude/4K`: `15,298 ns/op`, `32,916 B/op`, `1 allocs/op` (3.5x faster)
- `BenchmarkPower/4K`: `17,176 ns/op`, `32,951 B/op`, `1 allocs/op` (2.5x faster)

Zero-allocation fast paths:

- `BenchmarkMagnitudeFromParts/4K`: `2,439 ns/op`, `0 B/op`, `0 allocs/op`
- `BenchmarkPowerFromParts/4K`: `883 ns/op`, `0 B/op`, `0 allocs/op`

### `stats/time`

- `BenchmarkCalculate/4096`: `39405 ns/op`, `831.56 MB/s`, `0 allocs/op`
- `BenchmarkStreamingUpdate/4096`: `108048 ns/op`, `303.27 MB/s`, `0 allocs/op`

### `stats/frequency`

- `BenchmarkCalculate/fft=4096`: `66574 ns/op`, `246.22 MB/s`, `0 allocs/op`
- `BenchmarkFlatness/fft=4096`: `38812 ns/op`, `422.34 MB/s`, `0 allocs/op`

### `dsp/filter/moog`

Command: `go test -bench=. -benchmem -benchtime=2s -run=^$ -cpu=1 ./dsp/filter/moog/`

- `BenchmarkProcessSample/full`: `395.2 ns/op`, `0 allocs/op`
- `BenchmarkProcessSample/fast`: `80.71 ns/op`, `0 allocs/op`
- `BenchmarkProcessInPlace/full/N=4096`: `247191 ns/op`, `132.56 MB/s`, `0 allocs/op`
- `BenchmarkProcessInPlace/fast/N=4096`: `322483 ns/op`, `101.61 MB/s`, `0 allocs/op`
- `BenchmarkProcessOversampled/os=2/N=1024`: `761008 ns/op`, `10.76 MB/s`, `0 allocs/op`
- `BenchmarkProcessOversampled/os=4/N=1024`: `991299 ns/op`, `8.26 MB/s`, `0 allocs/op`
- `BenchmarkProcessOversampled/os=8/N=1024`: `3184478 ns/op`, `2.57 MB/s`, `0 allocs/op`

Note: the fast-tanh path has markedly lower per-sample latency (`ProcessSample`,
a feedback-bound dependency chain), while Go's optimized `math.Tanh` retains
slightly higher block throughput (`ProcessInPlace`). The oversampled high-quality
path costs roughly factor× the base ladder plus anti-alias filtering, trading CPU
for ~25 dB of alias rejection on out-of-band harmonics. All paths are zero-alloc.

## SIMD vs Scalar (internal/vecmath, n=4096)

Command:

```bash
GOCACHE=/tmp/gocache go test -run '^$' -bench 'Benchmark(AddBlock_Generic_Direct|AddBlock_AVX2_Direct|AddBlock_SSE2_Direct|MulBlock_Generic|MulBlock_AVX2|MulBlock_SSE2|ScaleBlock_Generic|ScaleBlock_AVX2|ScaleBlock_SSE2|AddMulBlock_Generic|AddMulBlock_AVX2|AddMulBlock_SSE2|MaxAbs_Generic|MaxAbs_AVX2)$' -benchmem ./internal/vecmath/arch/generic ./internal/vecmath/arch/amd64/avx2 ./internal/vecmath/arch/amd64/sse2
```

| Operation   | Generic ns/op | AVX2 ns/op | SSE2 ns/op | AVX2 speedup | SSE2 speedup |
| ----------- | ------------: | ---------: | ---------: | -----------: | -----------: |
| AddBlock    |          5564 |       2136 |       2003 |        2.61x |        2.78x |
| MulBlock    |          6365 |       1615 |       1896 |        3.94x |        3.36x |
| ScaleBlock  |          3476 |       1346 |       1983 |        2.58x |        1.75x |
| AddMulBlock |          6829 |       2308 |       1669 |        2.96x |        4.09x |
| MaxAbs      |          6081 |       1663 |        n/a |        3.66x |          n/a |

## Notes

- Use this file as the v1.0 baseline reference; future changes should include before/after deltas for affected benchmark families.
- For major optimization work, include architecture and command parity with this baseline.
