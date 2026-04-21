package defc

import (
	"bytes"
	"io"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"testing"
	"unsafe"
)

// TestPoolAliasingDetached_ResponseError asserts that NewResponseError owns
// an independent copy of body, so mutating the caller-side buffer (as happens
// when a sync.Pool-owned *bytes.Buffer is recycled) does not corrupt the
// previously-returned error's body.
func TestPoolAliasingDetached_ResponseError(t *testing.T) {
	cp := NewControlledPool()
	sentinel := new(bytes.Buffer)
	sentinel.Grow(64)
	cp.Sticky(sentinel)
	swapBufferPool(t, cp)

	buf := GetBuffer()
	buf.Reset()
	buf.Write(bytes.Repeat([]byte{'A'}, 64))
	err := NewResponseError("m", 500, buf.Bytes())

	buf.Reset()
	buf.Write(bytes.Repeat([]byte{'B'}, 64))

	if want := bytes.Repeat([]byte{'A'}, 64); !bytes.Equal(err.Body(), want) {
		t.Fatalf("body mutated via pool aliasing: got=%q want=%q", err.Body(), want)
	}
	if len(err.Body()) > 0 && uintptr(unsafe.Pointer(&err.Body()[0])) == uintptr(unsafe.Pointer(&buf.Bytes()[0])) {
		t.Fatalf("err.Body() aliases caller buffer: backing arrays identical")
	}
}

// TestPoolAliasingDetached_JSONFromBytes asserts that DetachBytes correctly
// severs the alias used by JSON.FromBytes, which stores its argument by
// reference into j.Raw.
func TestPoolAliasingDetached_JSONFromBytes(t *testing.T) {
	cp := NewControlledPool()
	sentinel := new(bytes.Buffer)
	sentinel.Grow(64)
	cp.Sticky(sentinel)
	swapBufferPool(t, cp)

	buf := GetBuffer()
	buf.Reset()
	buf.WriteString(`{"k":"A"}`)
	helper := DetachBytes(buf)
	j := &JSON{}
	if err := j.FromBytes("m", helper); err != nil {
		t.Fatalf("FromBytes returned err: %v", err)
	}

	buf.Reset()
	buf.WriteString(`{"k":"B"}`)

	if got := string(j.Raw); got != `{"k":"A"}` {
		t.Fatalf("j.Raw mutated via pool aliasing: got=%q want=%q", got, `{"k":"A"}`)
	}
	if len(j.Raw) > 0 && len(buf.Bytes()) > 0 &&
		uintptr(unsafe.Pointer(&j.Raw[0])) == uintptr(unsafe.Pointer(&buf.Bytes()[0])) {
		t.Fatalf("j.Raw aliases caller buffer")
	}
}

// TestPoolAliasingDetachBytes_Helper is a unit test for DetachBytes that
// touches no pool machinery.
func TestPoolAliasingDetachBytes_Helper(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	buf.WriteString("hello")
	got := DetachBytes(&buf)
	if len(got) != buf.Len() {
		t.Fatalf("len mismatch: got=%d want=%d", len(got), buf.Len())
	}
	if !bytes.Equal(got, []byte("hello")) {
		t.Fatalf("content mismatch: got=%q", string(got))
	}
	if len(got) > 0 && &got[0] == &buf.Bytes()[0] {
		t.Fatalf("DetachBytes returned aliased slice")
	}

	var empty bytes.Buffer
	emptyGot := DetachBytes(&empty)
	if len(emptyGot) != 0 {
		t.Fatalf("DetachBytes on empty buffer: len=%d want 0", len(emptyGot))
	}
}

// TestPoolAliasingSyntheticFuture pins the property that the api/future path
// (via JSON.FromResponse) never materialises the response body through the
// package bufferPool. A poisoned sticky sentinel makes any accidental pool use
// observable via the counters or via parse failure.
func TestPoolAliasingSyntheticFuture(t *testing.T) {
	cp := NewControlledPool()
	poison := new(bytes.Buffer)
	poison.WriteString("POISON")
	cp.Sticky(poison)
	swapBufferPool(t, cp)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"k":"V"}`)),
	}
	j := &JSON{}
	if err := j.FromResponse("m", resp); err != nil {
		t.Fatalf("FromResponse returned err: %v", err)
	}
	if err := j.Err(); err != nil {
		t.Fatalf("Err returned: %v", err)
	}
	// json.Decoder trims whitespace; allow exact match.
	if got := strings.TrimSpace(string(j.Raw)); got != `{"k":"V"}` {
		t.Fatalf("j.Raw: got=%q want=%q", got, `{"k":"V"}`)
	}
	if cp.Gets() != 0 || cp.Puts() != 0 {
		t.Fatalf("future path touched bufferPool: gets=%d puts=%d", cp.Gets(), cp.Puts())
	}
}

// TestPoolAliasingPutResetsBuffer asserts the belt-and-suspenders Reset inside
// PutBuffer. Uses the real sync.Pool (no swap) since the assertion is about
// the real production pool behaviour.
func TestPoolAliasingPutResetsBuffer(t *testing.T) {
	runtime.GC()
	warm := GetBuffer()
	PutBuffer(warm)

	b := GetBuffer()
	b.WriteString("dirty")
	PutBuffer(b)
	b2 := GetBuffer()
	defer PutBuffer(b2)

	if uintptr(unsafe.Pointer(b)) != uintptr(unsafe.Pointer(b2)) {
		t.Logf("pool returned fresh buffer; reset contract not exercised")
		return
	}
	if b2.Len() != 0 {
		t.Fatalf("PutBuffer did not reset: Len=%d", b2.Len())
	}
}

// TestPoolAliasingDeterministicReuse_NoRace deterministically reproduces the
// pool-aliasing hazard without relying on -race. Serial single-thread reuse
// with GOMAXPROCS=1 forces the sync.Pool private slot to hand back the same
// buffer on the second call; a regressed NewResponseError would store an
// aliased slice that the second io.Copy would overwrite in place.
//
// Truly concurrent deterministic failure in plain `go test` is not possible
// because the Go memory model permits the observed race to fire at any time;
// this case proves the property via forced serial reuse instead.
func TestPoolAliasingDeterministicReuse_NoRace(t *testing.T) {
	prev := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(prev)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	cp := NewControlledPool()
	sentinel := new(bytes.Buffer)
	sentinel.Grow(4096)
	cp.Sticky(sentinel)
	swapBufferPool(t, cp)

	doCall := func(payload []byte) ResponseError {
		buf := GetBuffer()
		defer PutBuffer(buf)
		defer buf.Reset()
		// mirror api.tmpl :488 io.Copy -> :511 NewResponseError -> :531 Reset
		if _, err := io.Copy(buf, bytes.NewReader(payload)); err != nil {
			t.Fatalf("io.Copy: %v", err)
		}
		return NewResponseError("m", 500, buf.Bytes())
	}

	payloadA := bytes.Repeat([]byte{'A'}, 4096)
	payloadB := bytes.Repeat([]byte{'B'}, 4096)

	err1 := doCall(payloadA)
	// PutBuffer + Reset have now fired via the outer defer stack; the sticky
	// sentinel is re-used in call 2 with the same backing array.
	err2 := doCall(payloadB)

	if len(err1.Body()) == 0 || len(err2.Body()) == 0 {
		t.Fatalf("unexpected empty bodies")
	}
	if !bytes.Equal(err1.Body(), payloadA) {
		t.Fatalf("pool aliasing regression: err1.Body()[0]=%q, sample=%q", err1.Body()[0], err1.Body()[:16])
	}
	if !bytes.Equal(err2.Body(), payloadB) {
		t.Fatalf("err2 body unexpected: sample=%q", err2.Body()[:16])
	}
}

// TestPoolAliasingRace_NewResponseError is a concurrent reproduction of the
// pool-aliasing hazard. Under `go test -race` a pre-fix runtime emits
// "DATA RACE" warnings between bytes.Buffer.Write in one goroutine and the
// body read in another. Post-fix, the defensive copy inside NewResponseError
// detaches the backing array and the test is race-free.
//
// The test compiles and runs without -race (no race-detector-specific APIs);
// splitting it into a separate //go:build race file would hide it from
// contributors without -race and let the code drift.
func TestPoolAliasingRace_NewResponseError(t *testing.T) {
	const N = 2
	const M = 10000
	payloads := [][]byte{
		bytes.Repeat([]byte{'A'}, 1024),
		bytes.Repeat([]byte{'B'}, 1024),
	}

	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		payload := payloads[i]
		go func() {
			defer wg.Done()
			for j := 0; j < M; j++ {
				buf := GetBuffer()
				buf.Write(payload)
				err := NewResponseError("m", 500, buf.Bytes())
				body := err.Body()
				if !bytes.Equal(body, payload) {
					// Observed across-goroutine corruption. Failed copy fix.
					t.Errorf("body corruption: got first=%q want first=%q", body[0], payload[0])
					buf.Reset()
					PutBuffer(buf)
					return
				}
				buf.Reset()
				PutBuffer(buf)
			}
		}()
	}
	wg.Wait()
}
