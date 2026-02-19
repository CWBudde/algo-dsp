// Package interp provides interpolation primitives used by delay-based DSP blocks.
//
// Available methods, from cheapest to highest quality:
//
//   - [Linear2]:      2-point linear interpolation
//   - [AllpassTick]:  first-order allpass (unity magnitude, phase-only)
//   - [Hermite4]:     4-point cubic Hermite (good default)
//   - [Lagrange4]:    4-point cubic Lagrange
//   - [Lanczos6]:     6-point Lanczos windowed-sinc (a = 3)
//   - [LanczosN]:     variable-width Lanczos windowed-sinc
//   - [SincInterp]:   variable-width Blackman-windowed sinc (highest quality)
//
// The [Mode] enum and the [delay.Line] type allow selecting the
// interpolation algorithm at construction time.
package interp
