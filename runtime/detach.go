package defc

import "bytes"

// DetachBytes returns an independent copy of the bytes currently held by buf.
//
// It is intended for callers that obtain a *bytes.Buffer from GetBuffer (or any
// sync.Pool-backed source) and need to hand the underlying bytes to a consumer
// that retains them past the buffer's return to the pool. The returned slice
// does not alias buf's backing array; mutating buf (including via Reset or a
// subsequent Write after recycling) is safe.
//
// buf must be non-nil; calling DetachBytes(nil) is a programmer error and
// panics identically to bytes.Buffer.Bytes on a nil receiver. DetachBytes
// handles empty buffers (including those with a nil backing array) safely:
// the returned slice has len == 0.
func DetachBytes(buf *bytes.Buffer) []byte {
	return append([]byte(nil), buf.Bytes()...)
}
