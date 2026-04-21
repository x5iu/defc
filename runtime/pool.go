package defc

import (
	"bytes"
	"sync"
)

// pool is the unexported interface that GetBuffer / PutBuffer dispatch through.
// Keeping the indirection private lets test code substitute an alternative pool
// implementation (see swapBufferPool in pool_testing.go) without widening the
// public API surface.
type pool interface {
	Get() *bytes.Buffer
	Put(*bytes.Buffer)
}

type syncPoolAdapter struct {
	p *sync.Pool
}

func (a syncPoolAdapter) Get() *bytes.Buffer { return a.p.Get().(*bytes.Buffer) }
func (a syncPoolAdapter) Put(b *bytes.Buffer) { a.p.Put(b) }

var defaultBufferPool = &sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

var bufferPool pool = syncPoolAdapter{p: defaultBufferPool}

func GetBuffer() *bytes.Buffer {
	return bufferPool.Get()
}

func PutBuffer(buffer *bytes.Buffer) {
	// Belt-and-suspenders: reset before returning to the pool so that a caller
	// which forgot its own defer Reset() cannot leak a dirty buffer to the next
	// consumer. The production template also defers Reset(), so this is a
	// defence-in-depth measure, not the primary cleanup path.
	if buffer != nil {
		buffer.Reset()
	}
	bufferPool.Put(buffer)
}
