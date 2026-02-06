// Package buffer provides a reusable float64 buffer type and pool for
// allocation-friendly DSP processing. All DSP functions accept raw
// []float64 slices; Buffer is an optional convenience that helps callers
// manage allocation and reuse in hot paths.
package buffer
