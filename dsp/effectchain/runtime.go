package effectchain

// Runtime is the per-node processing and configuration contract.
type Runtime interface {
	Configure(ctx Context, params Params) error
	Process(block []float64)
}

// SidechainProcessor is an optional interface for effects that accept
// a sidechain input (e.g., lookahead limiter, vocoder).
type SidechainProcessor interface {
	ProcessWithSidechain(main, sidechain []float64)
}
