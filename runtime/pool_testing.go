package defc

import (
	"bytes"
	"sync"
	"testing"
)

// ControlledPool is a test-only *bytes.Buffer pool with deterministic
// scheduling semantics. It implements the unexported pool interface from
// pool.go and is installed into the package-global bufferPool via
// swapBufferPool.
//
// Two modes are supported:
//
//   - FIFO mode (default): Preload(b1, b2, ...) queues buffers; Get returns
//     them in insertion order. Draining past the end panics, which catches
//     generator regressions where the emitted code pulls an unexpected extra
//     buffer from the pool.
//   - Sticky mode: Sticky(b) installs a single sentinel; every Get returns b,
//     and Put is a no-op. Counters still tick so callers can assert exact
//     Get/Put counts.
//
// Neither mode resets the buffer on Get — production GetBuffer does not, and
// the aliasing regression tests rely on observing whatever bytes a previous
// iteration wrote.
type ControlledPool struct {
	mu      sync.Mutex
	queue   []*bytes.Buffer
	sticky  *bytes.Buffer
	isStick bool
	gets    int
	puts    int
}

// NewControlledPool constructs an empty FIFO-mode pool.
func NewControlledPool() *ControlledPool {
	return &ControlledPool{}
}

// Preload appends bufs to the FIFO queue. Must not be mixed with Sticky.
func (p *ControlledPool) Preload(bufs ...*bytes.Buffer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.queue = append(p.queue, bufs...)
}

// Sticky switches the pool to sticky mode and installs buf as the sentinel.
func (p *ControlledPool) Sticky(buf *bytes.Buffer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sticky = buf
	p.isStick = true
}

// Gets returns the number of Get calls observed.
func (p *ControlledPool) Gets() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.gets
}

// Puts returns the number of Put calls observed.
func (p *ControlledPool) Puts() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.puts
}

// Get returns the next buffer. In FIFO mode the queue head is popped; when
// the queue is empty the pool panics to surface missing template emissions.
// In sticky mode the sentinel buffer is returned on every call.
func (p *ControlledPool) Get() *bytes.Buffer {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.gets++
	if p.isStick {
		return p.sticky
	}
	if len(p.queue) == 0 {
		panic("controlledPool: FIFO exhausted")
	}
	buf := p.queue[0]
	p.queue = p.queue[1:]
	return buf
}

// Put returns the buffer to the pool. In sticky mode it is a no-op aside from
// counter tracking.
func (p *ControlledPool) Put(buf *bytes.Buffer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.puts++
	if p.isStick {
		return
	}
	p.queue = append(p.queue, buf)
}

// swapBufferPool installs p as the package-global bufferPool and registers a
// t.Cleanup that restores the original. The testing.TB argument makes it
// impossible to call from non-test code.
func swapBufferPool(t testing.TB, p pool) (restore func()) {
	t.Helper()
	prev := bufferPool
	bufferPool = p
	restore = func() {
		bufferPool = prev
	}
	t.Cleanup(restore)
	return restore
}
