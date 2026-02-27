package effectchain

// Context provides environmental information that effect runtimes need.
type Context struct {
	SampleRate float64
}

// IRProvider allows runtimes to load impulse responses without depending on application types.
type IRProvider interface {
	GetIR(index int) (samples [][]float64, sampleRate float64, ok bool)
}
