package defc

import (
	"encoding/binary"
	"errors"
	"reflect"
	"sort"
	"testing"
)

type fuzzReqBody struct {
	TenantID string `db:"tenant_id"`
	Name     string `db:"name"`
}

type fuzzTenantCtx struct {
	ID string `db:"tenant_id"`
}

type fuzzNilable struct {
	X int `db:"x"`
}

// buildCorpus walks raw bytes into a synthetic map[string]any
// drawing from primitives, maps, slices, pointers and small
// struct implementors. The exact layout does not matter; it just
// needs to exercise every branch of MergeNamedArgsStrict.
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
			m[k] = string(data[i%len(data) : fuzzMin(i%len(data)+2, len(data))])
		case 2:
			m[k] = []int{1, 2, 3}
		case 3:
			m[k] = map[string]any{"tenant_id": "x", "extra": int(data[i%len(data)])}
		case 4:
			m[k] = fuzzReqBody{TenantID: "r", Name: "n"}
		case 5:
			m[k] = fuzzTenantCtx{ID: "t"}
		case 6:
			m[k] = (map[string]any)(nil)
		case 7:
			m[k] = (*fuzzNilable)(nil)
		case 8:
			m[k] = nil
		case 9:
			m[k] = map[int]string{1: "a"}
		}
	}
	return m
}

func fuzzMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func FuzzMergeNamedArgsStrict(f *testing.F) {
	f.Add([]byte{0x00, 0x01, 0x02, 0x03})
	f.Add([]byte{0x04, 0x05, 0x01, 0x03})
	f.Add(binary.BigEndian.AppendUint32(nil, 0xdeadbeef))
	f.Fuzz(func(t *testing.T, data []byte) {
		corpus := buildCorpus(data)
		// Property 1: strict never panics on arbitrary synthetic inputs.
		strictMap, strictErr := MergeNamedArgsStrict(corpus)
		// Property 2: any error wraps ErrNamedArgsCollision.
		if strictErr != nil {
			if !errors.Is(strictErr, ErrNamedArgsCollision) {
				t.Fatalf("strict error not Is(ErrNamedArgsCollision): %T %v", strictErr, strictErr)
			}
			if strictMap != nil {
				t.Fatalf("strict returned partial map alongside error")
			}
		}
		// Property 3: on success, key set matches legacy MergeNamedArgs.
		if strictErr == nil {
			legacy := MergeNamedArgs(corpus)
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
