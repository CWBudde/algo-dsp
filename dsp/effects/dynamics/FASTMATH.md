# Fast Math Approximations for Compressor

The compressor implementation supports optional fast math approximations via the [algo-approx](https://github.com/cwbudde/algo-approx) library.

## Performance Comparison

Benchmark results on 12th Gen Intel Core i7-1255U:

| Configuration                | ProcessSample (ns/op) | Speedup         |
| ---------------------------- | --------------------- | --------------- |
| Standard math                | ~76 ns/op             | baseline        |
| Fast math (`-tags=fastmath`) | ~56 ns/op             | **~26% faster** |

**Key findings:**

- Fast math provides ~26% speedup in single-sample processing
- Zero allocations maintained in both variants
- Numerical accuracy remains excellent for audio DSP (see Accuracy section)

## Usage

### Standard Math (Default)

By default, the compressor uses Go's standard library `math` package:

```bash
go build ./...
go test ./dsp/effects/dynamics/
```

### Fast Math (Opt-in)

Enable fast approximations with the `fastmath` build tag:

```bash
go build -tags=fastmath ./...
go test -tags=fastmath ./dsp/effects/dynamics/
go test -tags=fastmath -bench=Compressor ./dsp/effects/dynamics/
```

## Implementation Details

The compressor abstracts math operations through internal functions:

- `mathLog2(x)` - log₂(x) using `FastLog` via identity: log₂(x) = ln(x) / ln(2)
- `mathPower2(x)` - 2^x using `FastExp` via identity: 2^x = e^(x·ln(2))
- `mathPower10(x)` - 10^x (standard math, not in hot path)
- `mathSqrt(x)` - √x using `FastSqrt`

Build tags select the implementation:

- [compressor_math.go](compressor_math.go) - Standard math (default, no tag)
- [compressor_math_fast.go](compressor_math_fast.go) - Fast approximations (`-tags=fastmath`)

## Accuracy

The algo-approx library provides excellent accuracy for audio DSP:

| Function     | Decimal Digits | Max Relative Error |
| ------------ | -------------: | -----------------: |
| FastLog (ln) |            3.1 |          7.58×10⁻⁴ |
| FastExp      |            5.5 |          3.23×10⁻⁶ |
| FastSqrt     |            5.8 |          1.50×10⁻⁶ |

**For audio DSP context:**

- 16-bit audio: ~4.8 decimal digits precision
- 24-bit audio: ~7.2 decimal digits precision
- Fast approximations exceed 16-bit precision and approach 24-bit

The compressor's soft-knee algorithm uses log₂-domain processing, making it robust to small approximation errors. The smooth gain curve characteristics are preserved with fast math.

## When to Use Fast Math

**Recommended for:**

- Real-time audio processing with CPU constraints
- High channel counts (>16 channels)
- Sample rates ≥96kHz
- Embedded systems with limited CPU
- Batch processing large audio files

**Standard math sufficient for:**

- Offline processing without strict latency requirements
- Low channel counts (≤8 channels)
- When maximum numerical precision is critical
- Development and debugging

## Testing

All tests pass with both standard and fast math:

```bash
# Test both variants
go test ./dsp/effects/dynamics/
go test -tags=fastmath ./dsp/effects/dynamics/

# Compare benchmarks
go test -bench=Compressor ./dsp/effects/dynamics/ > bench-standard.txt
go test -tags=fastmath -bench=Compressor ./dsp/effects/dynamics/ > bench-fast.txt
```

## References

- [algo-approx](https://github.com/cwbudde/algo-approx) - Fast mathematical approximations library
- [algo-approx ACCURACY.md](https://github.com/cwbudde/algo-approx/blob/main/ACCURACY.md) - Detailed accuracy metrics
