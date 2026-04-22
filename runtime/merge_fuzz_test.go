package defc

import (
	"database/sql/driver"
	"encoding/binary"
	"errors"
	"reflect"
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

func stringSetEqual(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

func isNilStructPointerArg(arg any) bool {
	if arg == nil {
		return false
	}
	rv := reflect.ValueOf(arg)
	return rv.Kind() == reflect.Pointer &&
		rv.Type().Elem().Kind() == reflect.Struct &&
		rv.IsNil()
}

func dropNilStructPointerEntries(argsMap map[string]any) map[string]any {
	out := make(map[string]any, len(argsMap))
	for k, v := range argsMap {
		if isNilStructPointerArg(v) {
			continue
		}
		out[k] = v
	}
	return out
}

func FuzzMergeNamedArgsStrict(f *testing.F) {
	f.Add([]byte{0x00, 0x01, 0x02, 0x03})
	f.Add([]byte{0x04, 0x05, 0x01, 0x03})
	f.Add(binary.BigEndian.AppendUint32(nil, 0xdeadbeef))
	f.Fuzz(func(t *testing.T, data []byte) {
		corpus := buildCorpus(data)
		strictMap, strictErr := MergeNamedArgsStrict(corpus)
		if strictErr != nil {
			if !errors.Is(strictErr, ErrNamedArgsCollision) {
				t.Fatalf("strict error not Is(ErrNamedArgsCollision): %T %v", strictErr, strictErr)
			}
			if strictMap != nil {
				t.Fatalf("strict returned partial map alongside error")
			}
			saw := false
			eachNamedArgsCollision(strictErr, func(_ NamedArgsCollisionError) { saw = true })
			if !saw {
				t.Fatalf("expected at least one NamedArgsCollisionError in chain: %T", strictErr)
			}
			want := legacyMergeOverwrittenKeys(corpus)
			got := keysFromCollisions(strictErr)
			if !stringSetEqual(got, want) {
				t.Fatalf("collision key set: got %v want %v", got, want)
			}
			return
		}
		legacy := MergeNamedArgs(dropNilStructPointerEntries(corpus))
		if !reflect.DeepEqual(legacy, strictMap) {
			t.Fatalf("maps diverged legacy=%#v strict=%#v", legacy, strictMap)
		}
	})
}

func assignLegacy(namedMap map[string]any, overwrites map[string]struct{}, k string, v any) {
	if _, ok := namedMap[k]; ok {
		overwrites[k] = struct{}{}
	}
	namedMap[k] = v
}

func legacyMergeOverwrittenKeys(argsMap map[string]any) map[string]struct{} {
	overwrites := make(map[string]struct{})
	namedMap := make(map[string]any, len(argsMap))
	for name, arg := range argsMap {
		if isNilStructPointerArg(arg) {
			continue
		}
		rv := reflect.ValueOf(arg)
		if _, notAnArg := arg.(NotAnArg); notAnArg {
			continue
		} else if toNamedArgs, ok := arg.(ToNamedArgs); ok {
			for k, v := range toNamedArgs.ToNamedArgs() {
				assignLegacy(namedMap, overwrites, k, v)
			}
		} else if _, ok = arg.(driver.Valuer); ok {
			assignLegacy(namedMap, overwrites, name, arg)
		} else if _, ok = arg.(ToArgs); ok {
			assignLegacy(namedMap, overwrites, name, arg)
		} else if rv.Kind() == reflect.Map {
			iter := rv.MapRange()
			for iter.Next() {
				k, v := iter.Key(), iter.Value()
				if k.Kind() == reflect.String {
					assignLegacy(namedMap, overwrites, k.String(), v.Interface())
				}
			}
		} else if rv.Kind() == reflect.Struct ||
			(rv.Kind() == reflect.Pointer && rv.Elem().Kind() == reflect.Struct) {
			rv = reflect.Indirect(rv)
			rt := rv.Type()
			for i := 0; i < rt.NumField(); i++ {
				if sf := rt.Field(i); sf.Anonymous {
					sft := sf.Type
					if sft.Kind() == reflect.Pointer {
						sft = sft.Elem()
					}
					for j := 0; j < sft.NumField(); j++ {
						if tag, exists := sft.Field(j).Tag.Lookup("db"); exists {
							for pos, char := range tag {
								if !(('0' <= char && char <= '9') || ('a' <= char && char <= 'z') || ('A' <= char && char <= 'Z') || char == '_') {
									tag = tag[:pos]
									break
								}
							}
							assignLegacy(namedMap, overwrites, tag, rv.FieldByIndex([]int{i, j}).Interface())
						}
					}
				} else if tag, exists := sf.Tag.Lookup("db"); exists {
					for pos, char := range tag {
						if !(('0' <= char && char <= '9') || ('a' <= char && char <= 'z') || ('A' <= char && char <= 'Z') || char == '_') {
							tag = tag[:pos]
							break
						}
					}
					assignLegacy(namedMap, overwrites, tag, rv.Field(i).Interface())
				}
			}
		} else {
			assignLegacy(namedMap, overwrites, name, arg)
		}
	}
	return overwrites
}

func eachNamedArgsCollision(err error, fn func(NamedArgsCollisionError)) {
	var agg NamedArgsCollisionErrors
	if errors.As(err, &agg) {
		for i := range agg {
			fn(agg[i])
		}
		return
	}
	var p *NamedArgsCollisionError
	if errors.As(err, &p) {
		fn(*p)
	}
}

func keysFromCollisions(err error) map[string]struct{} {
	out := make(map[string]struct{})
	eachNamedArgsCollision(err, func(e NamedArgsCollisionError) {
		out[e.Key] = struct{}{}
	})
	return out
}
