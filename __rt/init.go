package __rt

import "reflect"

func New[T any]() (v T) {
	val := reflect.ValueOf(&v).Elem()
	switch val.Kind() {
	case reflect.Slice, reflect.Map, reflect.Chan, reflect.Func, reflect.Pointer:
		val.Set(newType(val.Type()))
	}
	return v
}

func newType(typ reflect.Type) reflect.Value {
	switch typ.Kind() {
	case reflect.Slice:
		return reflect.MakeSlice(typ, 0, 0)
	case reflect.Map:
		return reflect.MakeMap(typ)
	case reflect.Chan:
		return reflect.MakeChan(typ, 0)
	case reflect.Func:
		return reflect.MakeFunc(typ, func(_ []reflect.Value) (results []reflect.Value) {
			results = make([]reflect.Value, typ.NumOut())
			for i := 0; i < typ.NumOut(); i++ {
				results[i] = newType(typ.Out(i))
			}
			return results
		})
	case reflect.Pointer:
		return reflect.New(typ.Elem())
	default:
		return reflect.Zero(typ)
	}
}
