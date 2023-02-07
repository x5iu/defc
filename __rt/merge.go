package __rt

import (
	"database/sql/driver"
	"errors"
	"reflect"
	"strings"
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
			for _, v := range toArgs.ToArgs() {
				dst = append(dst, v)
			}
		} else if _, ok = arg.(driver.Valuer); ok {
			dst = append(dst, arg)
		} else if (rv.Kind() == reflect.Slice && rv.Type() != bytesType) ||
			rv.Kind() == reflect.Array {
			for i := 0; i < rv.Len(); i++ {
				dst = append(dst, rv.Index(i).Interface())
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
			rt := reflect.Indirect(rv).Type()
			for i := 0; i < rt.NumField(); i++ {
				if tag, ok := rt.Field(i).Tag.Lookup("db"); ok {
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
		if rv.Type() == bytesType {
			n = 1
		} else {
			n = rv.Len()
		}
	default:
		n = 1
	}

	bindVars := make([]string, n)
	for i := 0; i < n; i++ {
		bindVars[i] = "?"
	}

	return strings.Join(bindVars, ", ")
}

func In(query string, args []any) (string, []any, error) {
	tokens := splitTokens(query)
	targetArgs := make([]any, 0, len(args))
	targetQuery := make([]string, 0, len(tokens))
	n := 0
	for _, token := range tokens {
		if n >= len(args) {
			return "", nil, errors.New("number of BindVars exceeds arguments")
		}
		switch token {
		case "?":
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
		return "", nil, errors.New("number of bindVars less than number arguments")
	}
	return strings.Join(targetQuery, " "), targetArgs, nil
}
