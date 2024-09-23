package defc

import (
	"database/sql/driver"
	"errors"
	"reflect"

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

func (arguments *Arguments) Add(argument any) string {
	merged := MergeArgs(argument)
	*arguments = append(*arguments, merged...)
	return BindVars(len(merged))
}
