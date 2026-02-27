package effectchain

// stubRuntime is a minimal Runtime implementation for testing.
type stubRuntime struct {
	configureErr   error
	configureCalls int
	processCalls   int
	lastCtx        Context
	lastParams     Params
}

func (s *stubRuntime) Configure(ctx Context, params Params) error {
	s.configureCalls++
	s.lastCtx = ctx
	s.lastParams = params

	return s.configureErr
}

func (s *stubRuntime) Process(_ []float64) {
	s.processCalls++
}

// gainRuntime multiplies every sample by a fixed gain.
type gainRuntime struct {
	gain float64
}

func (g *gainRuntime) Configure(_ Context, params Params) error {
	g.gain = params.GetNum("gain", 1.0)

	return nil
}

func (g *gainRuntime) Process(block []float64) {
	for i := range block {
		block[i] *= g.gain
	}
}

// addRuntime adds a constant to every sample (for testing multi-parent mixing).
type addRuntime struct {
	value float64
}

func (a *addRuntime) Configure(_ Context, params Params) error {
	a.value = params.GetNum("value", 0)

	return nil
}

func (a *addRuntime) Process(block []float64) {
	for i := range block {
		block[i] += a.value
	}
}

// sidechainStubRuntime implements both Runtime and SidechainProcessor.
type sidechainStubRuntime struct {
	mainCopy []float64
	sideCopy []float64
}

func (s *sidechainStubRuntime) Configure(_ Context, _ Params) error {
	return nil
}

func (s *sidechainStubRuntime) Process(_ []float64) {
	// no-op for non-sidechain path
}

func (s *sidechainStubRuntime) ProcessWithSidechain(main, sidechain []float64) {
	s.mainCopy = make([]float64, len(main))
	copy(s.mainCopy, main)

	s.sideCopy = make([]float64, len(sidechain))
	copy(s.sideCopy, sidechain)

	// Mix: main * 0.5 + sidechain * 0.5 for easy verification.
	for i := range main {
		main[i] = 0.5*main[i] + 0.5*sidechain[i]
	}
}

// testRegistry creates a registry with simple test effects.
func testRegistry() *Registry {
	r := NewRegistry()

	r.MustRegister("stub", func(_ Context) (Runtime, error) {
		return &stubRuntime{}, nil
	})
	r.MustRegister("gain", func(_ Context) (Runtime, error) {
		return &gainRuntime{gain: 1.0}, nil
	})
	r.MustRegister("add", func(_ Context) (Runtime, error) {
		return &addRuntime{}, nil
	})

	return r
}

// testRegistryWithSidechain creates a registry with stub + sidechain effects.
func testRegistryWithSidechain() *Registry {
	r := testRegistry()

	r.MustRegister("dyn-lookahead", func(_ Context) (Runtime, error) {
		return &sidechainStubRuntime{}, nil
	})
	r.MustRegister("vocoder", func(_ Context) (Runtime, error) {
		return &sidechainStubRuntime{}, nil
	})

	return r
}
