package defc

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"strings"

	tok "github.com/x5iu/defc/runtime/token"
)

type NotAnArg interface {
	NotAnArg()
}

type ToArgs interface {
	ToArgs() []any
}

type ToNamedArgs interface {
	ToNamedArgs() map[string]any
}

var bytesType = reflect.TypeOf([]byte{})

func MergeArgs(args ...any) []any {
	dst := make([]any, 0, len(args))
	for _, arg := range args {
		rv := reflect.ValueOf(arg)
		if _, notAnArg := arg.(NotAnArg); notAnArg {
			continue
		} else if toArgs, ok := arg.(ToArgs); ok {
			dst = append(dst, MergeArgs(toArgs.ToArgs()...)...)
		} else if _, ok = arg.(driver.Valuer); ok {
			dst = append(dst, arg)
		} else if (rv.Kind() == reflect.Slice && !rv.Type().AssignableTo(bytesType)) ||
			rv.Kind() == reflect.Array {
			for i := 0; i < rv.Len(); i++ {
				dst = append(dst, MergeArgs(rv.Index(i).Interface())...)
			}
		} else {
			dst = append(dst, arg)
		}
	}
	return dst
}

func MergeNamedArgs(argsMap map[string]any) map[string]any {
	namedMap := make(map[string]any, len(argsMap))
	for name, arg := range argsMap {
		rv := reflect.ValueOf(arg)
		if _, notAnArg := arg.(NotAnArg); notAnArg {
			continue
		} else if toNamedArgs, ok := arg.(ToNamedArgs); ok {
			for k, v := range toNamedArgs.ToNamedArgs() {
				namedMap[k] = v
			}
		} else if _, ok = arg.(driver.Valuer); ok {
			namedMap[name] = arg
		} else if _, ok = arg.(ToArgs); ok {
			namedMap[name] = arg
		} else if rv.Kind() == reflect.Map {
			iter := rv.MapRange()
			for iter.Next() {
				k, v := iter.Key(), iter.Value()
				if k.Kind() == reflect.String {
					namedMap[k.String()] = v.Interface()
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
							namedMap[tag] = rv.FieldByIndex([]int{i, j}).Interface()
						}
					}
				} else if tag, exists := sf.Tag.Lookup("db"); exists {
					for pos, char := range tag {
						if !(('0' <= char && char <= '9') || ('a' <= char && char <= 'z') || ('A' <= char && char <= 'Z') || char == '_') {
							tag = tag[:pos]
							break
						}
					}
					namedMap[tag] = rv.Field(i).Interface()
				}
			}
		} else {
			namedMap[name] = arg
		}
	}
	return namedMap
}

// ErrNamedArgsCollision is the sentinel wrapped by every error
// returned from [MergeNamedArgsStrict]. Use [errors.Is] to detect it.
var ErrNamedArgsCollision = errors.New("defc: duplicate bind key")

// NamedArgsCollisionError describes a single duplicate bind key. The
// Sources slice carries the provenance labels of the two (or more)
// contributors, in discovery order. Because Go randomises map
// iteration the slice order is not stable across runs; treat it as a
// set.
type NamedArgsCollisionError struct {
	Key     string
	Sources []string
}

func (e *NamedArgsCollisionError) Error() string {
	if e == nil {
		return "<nil>"
	}
	var joined string
	switch len(e.Sources) {
	case 0:
		joined = "<unknown>"
	case 1:
		joined = e.Sources[0]
	case 2:
		joined = e.Sources[0] + " and " + e.Sources[1]
	default:
		joined = strings.Join(e.Sources[:len(e.Sources)-1], ", ") + " and " + e.Sources[len(e.Sources)-1]
	}
	return fmt.Sprintf("defc: duplicate bind key %q contributed by %s", e.Key, joined)
}

func (e *NamedArgsCollisionError) Unwrap() error { return ErrNamedArgsCollision }

// NamedArgsCollisionErrors aggregates multiple collisions discovered
// during a single merge call. It reports `errors.Is(err,
// ErrNamedArgsCollision)` true and exposes each entry for
// [errors.As] traversal.
type NamedArgsCollisionErrors []NamedArgsCollisionError

func (es NamedArgsCollisionErrors) Error() string {
	parts := make([]string, 0, len(es))
	for i := range es {
		parts = append(parts, (&es[i]).Error())
	}
	return strings.Join(parts, "; ")
}

func (es NamedArgsCollisionErrors) Is(target error) bool {
	return target == ErrNamedArgsCollision
}

func (es NamedArgsCollisionErrors) Unwrap() []error {
	out := make([]error, len(es))
	for i := range es {
		e := es[i]
		out[i] = &e
	}
	return out
}

// MergeNamedArgsStrict behaves like [MergeNamedArgs] except that any
// duplicate bind key contributed by two distinct sources returns an
// error that wraps [ErrNamedArgsCollision]. The concrete error type
// is either *[NamedArgsCollisionError] (single collision) or
// [NamedArgsCollisionErrors] (multiple collisions). On error the
// returned map is nil.
func containsString(s []string, e string) bool {
	for i := range s {
		if s[i] == e {
			return true
		}
	}
	return false
}

func MergeNamedArgsStrict(argsMap map[string]any) (map[string]any, error) {
	namedMap := make(map[string]any, len(argsMap))
	provenance := make(map[string]string, len(argsMap))
	collisionIndex := map[string]int{}
	var collisions []NamedArgsCollisionError

	put := func(k string, v any, src string) {
		if prev, ok := provenance[k]; ok && prev != src {
			if idx, ok2 := collisionIndex[k]; ok2 {
				existing := &collisions[idx]
				if !containsString(existing.Sources, src) {
					existing.Sources = append(existing.Sources, src)
				}
			} else {
				collisions = append(collisions, NamedArgsCollisionError{
					Key:     k,
					Sources: []string{prev, src},
				})
				collisionIndex[k] = len(collisions) - 1
			}
			return
		}
		namedMap[k] = v
		provenance[k] = src
	}

	for name, arg := range argsMap {
		rv := reflect.ValueOf(arg)
		if _, notAnArg := arg.(NotAnArg); notAnArg {
			continue
		} else if toNamedArgs, ok := arg.(ToNamedArgs); ok {
			src := "ToNamedArgs(" + name + ")"
			for k, v := range toNamedArgs.ToNamedArgs() {
				put(k, v, src)
			}
		} else if _, ok = arg.(driver.Valuer); ok {
			put(name, arg, "driver.Valuer("+name+")")
		} else if _, ok = arg.(ToArgs); ok {
			put(name, arg, "ToArgs("+name+")")
		} else if rv.Kind() == reflect.Map {
			src := "map(" + name + ")"
			iter := rv.MapRange()
			for iter.Next() {
				k, v := iter.Key(), iter.Value()
				if k.Kind() == reflect.String {
					put(k.String(), v.Interface(), src)
				}
			}
		} else if rv.Kind() == reflect.Struct ||
			(rv.Kind() == reflect.Pointer && rv.Type().Elem().Kind() == reflect.Struct) {
			if rv.Kind() == reflect.Pointer {
				if rv.IsNil() {
					continue
				}
				rv = rv.Elem()
			}
			rt := rv.Type()
			for i := 0; i < rt.NumField(); i++ {
				if sf := rt.Field(i); sf.Anonymous {
					sft := sf.Type
					if sft.Kind() == reflect.Pointer {
						sft = sft.Elem()
					}
					for j := 0; j < sft.NumField(); j++ {
						if rawTag, exists := sft.Field(j).Tag.Lookup("db"); exists {
							if rawTag == "-" {
								continue
							}
							tag := rawTag
							if idx := strings.Index(tag, ","); idx >= 0 {
								tag = tag[:idx]
							}
							tag = truncateDBTag(tag)
							put(tag,
								rv.FieldByIndex([]int{i, j}).Interface(),
								"struct("+name+"."+sf.Name+"."+sft.Field(j).Name+")")
						}
					}
				} else if rawTag, exists := sf.Tag.Lookup("db"); exists {
					if rawTag == "-" {
						continue
					}
					tag := rawTag
					if idx := strings.Index(tag, ","); idx >= 0 {
						tag = tag[:idx]
					}
					tag = truncateDBTag(tag)
					put(tag, rv.Field(i).Interface(),
						"struct("+name+"."+sf.Name+")")
				}
			}
		} else {
			put(name, arg, "scalar("+name+")")
		}
	}

	if len(collisions) == 0 {
		return namedMap, nil
	}
	if len(collisions) == 1 {
		c := collisions[0]
		return nil, &c
	}
	return nil, NamedArgsCollisionErrors(collisions)
}

func truncateDBTag(tag string) string {
	for pos, char := range tag {
		if !(('0' <= char && char <= '9') ||
			('a' <= char && char <= 'z') ||
			('A' <= char && char <= 'Z') ||
			char == '_') {
			return tag[:pos]
		}
	}
	return tag
}

func BindVars(data any) string {
	var n int
	switch rv := reflect.ValueOf(data); rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n = int(rv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n = int(rv.Uint())
	case reflect.Slice:
		if rv.Type().AssignableTo(bytesType) {
			n = 1
		} else {
			n = rv.Len()
		}
	default:
		n = 1
	}
	maxInt := func(a, b int) int {
		if a > b {
			return a
		}
		return b
	}
	bindvars := make([]string, 0, maxInt(2*n-1, 2))
	for i := 0; i < n; i++ {
		if i > 0 {
			bindvars = append(bindvars, tok.Comma)
		}
		bindvars = append(bindvars, tok.Question)
	}
	return tok.MergeSqlTokens(bindvars)
}

func In[S ~[]any](query string, args S) (string, S, error) {
	tokens := tok.SplitTokens(query)
	targetArgs := make(S, 0, len(args))
	targetQuery := make([]string, 0, len(tokens))
	n := 0
	for _, token := range tokens {
		switch token {
		case tok.Question:
			if n >= len(args) {
				return "", nil, errors.New("number of bind-vars exceeds arguments")
			}
			nested := MergeArgs(args[n])
			if len(nested) == 0 {
				return "", nil, errors.New("empty slice passed to 'in' query")
			}
			targetArgs = append(targetArgs, nested...)
			targetQuery = append(targetQuery, BindVars(len(nested)))
			n++
		default:
			targetQuery = append(targetQuery, token)
		}
	}
	if n < len(args) {
		return "", nil, errors.New("number of bind-vars less than number arguments")
	}
	return tok.MergeSqlTokens(targetQuery), targetArgs, nil
}

// in is a special function designed to allow the sqlx package to reference it without using import,
// but instead through go:linkname, in order to avoid circular references.
func in(query string, args ...any) (string, []any, error) {
	return In[[]any](query, args)
}

type Arguments []any

func (arguments *Arguments) add(argument any) string {
	merged := MergeArgs(argument)
	*arguments = append(*arguments, merged...)
	return BindVars(len(merged))
}

func (arguments *Arguments) Add(argument any) string    { return arguments.add(argument) }
func (arguments *Arguments) Bind(argument any) string   { return arguments.add(argument) }
func (arguments *Arguments) Push(argument any) string   { return arguments.add(argument) }
func (arguments *Arguments) Append(argument any) string { return arguments.add(argument) }
