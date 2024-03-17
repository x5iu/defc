package __rt

import (
	"runtime"
	"testing"
	"unsafe"
)

func TestPool(t *testing.T) {
	buffer := GetBuffer()
	if buffer == nil {
		t.Errorf("pool: buffer = %v", buffer)
		return
	}
	bufferAddress := (uintptr)(unsafe.Pointer(buffer))
	if bufferAddress == 0 {
		t.Errorf("pool: uintptr = %d", bufferAddress)
		return
	}
	PutBuffer(buffer)
	nextBuffer := GetBuffer()
	if nextBuffer == nil {
		t.Errorf("pool: buffer = %v", nextBuffer)
		return
	}
	nextBufferAddress := (uintptr)(unsafe.Pointer(nextBuffer))
	if nextBufferAddress == 0 {
		t.Errorf("pool: uintptr = %d", nextBufferAddress)
		return
	}
	if nextBufferAddress != bufferAddress {
		// sync/pool.go:121
		// Get may choose to ignore the pool and treat it as empty.
		// Callers should not assume any relation between values passed to Put and
		// the values returned by Get.
		t.Logf("pool: %d != %d", nextBufferAddress, bufferAddress)
		return
	}
	runtime.KeepAlive(buffer)
}
