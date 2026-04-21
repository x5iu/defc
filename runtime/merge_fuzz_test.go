package defc

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"reflect"
	"sort"
	"testing"
)

// buildCorpus walks raw bytes into a synthetic map[string]any
// drawing from primitives, maps, slices, pointers and small
// ToNamedArgs / struct implementors. The exact layout does not
// matter; it just needs to exercise every branch of mergeNamedArgs.
func buildCorpus(data []byte) map[string]any {
	m := make(map[string]any)
	if len(data) == 0 {
		return m
	}
	keys := []string{"a", "b", "c", "d", "tenant_id", "name"}
	for i := 0; i < len(data); {
		k := keys[int(data[i])%len(keys)]
		i++
		if i >= len(data) {
			break
		}
		kind := data[i] % 10
		i++
		switch kind {
		case 0:
			m[k] = int(data[i%len(data)])
		case 1:
			m[k] = string(data[i%len(data) : min(i%len(data)+2, len(data))])
		case 2:
			m[k] = []int{1, 2, 3}
		case 3:
			m[k] = map[string]any{"tenant_id": "x", "extra": int(data[i%len(data)])}
		case 4:
			m[k] = reqBody{TenantID: "r", Name: "n"}
		case 5:
			m[k] = tenantCtxT{ID: "t"}
		case 6:
			m[k] = (map[string]any)(nil)
		case 7:
			m[k] = (*nilable)(nil)
		case 8:
			m[k] = nil
		case 9:
			m[k] = map[int]string{1: "a"}
		}
	}
	return m
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func FuzzMergeNamedArgs(f *testing.F) {
	log.SetOutput(io.Discard)
	f.Add([]byte{0x00, 0x01, 0x02, 0x03})
	f.Add([]byte{0x04, 0x05, 0x01, 0x03})
	f.Add(binary.BigEndian.AppendUint32(nil, 0xdeadbeef))
	f.Fuzz(func(t *testing.T, data []byte) {
		corpus := buildCorpus(data)
		// Property 1: strict never panics.
		strictMap, strictErr := MergeNamedArgsStrict(corpus)
		// Property 3: strict error is typed.
		if strictErr != nil {
			var col *NamedArgsCollisionError
			var agg NamedArgsCollisionErrors
			if !errors.As(strictErr, &col) && !errors.As(strictErr, &agg) {
				t.Fatalf("strict returned bare error type: %T", strictErr)
			}
			if !errors.Is(strictErr, ErrNamedArgsCollision) {
				t.Fatalf("strict error not Is(ErrNamedArgsCollision)")
			}
			if strictMap != nil {
				t.Fatalf("strict returned partial map alongside error")
			}
		}

		// Property 4: legacy and strict agree on key set when strict succeeded.
		legacy := MergeNamedArgs(corpus)
		if strictErr == nil {
			if !reflect.DeepEqual(sortedKeys(legacy), sortedKeys(strictMap)) {
				t.Fatalf("key sets diverged legacy=%v strict=%v",
					sortedKeys(legacy), sortedKeys(strictMap))
			}
		}
	})
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
