package buffer

import "sync"

// Pool provides sync.Pool-based Buffer reuse to reduce GC pressure
// in real-time processing loops.
type Pool struct {
	pool sync.Pool
}

// NewPool returns a Pool ready for use.
func NewPool() *Pool {
	return &Pool{
		pool: sync.Pool{
			New: func() any {
				return &Buffer{}
			},
		},
	}
}

// Get returns a Buffer with the requested length. The buffer is zeroed.
// Callers must return it via Put when done.
func (p *Pool) Get(length int) *Buffer {
	b := p.pool.Get().(*Buffer)
	b.Resize(length)
	b.Zero()
	return b
}

// Put returns a Buffer to the pool for reuse.
// The caller must not use the buffer after calling Put.
func (p *Pool) Put(b *Buffer) {
	if b == nil {
		return
	}
	p.pool.Put(b)
}
