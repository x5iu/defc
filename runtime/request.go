package defc

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sync"
	"unsafe"
)

// JSONBody is a shortcut type used for quickly constructing an io.Reader.
//
// Embed JSONBody as the first field in the struct, and set the generic parameter of JSONBody
// to the type of the embedded struct (ensure to use a value type rather than a pointer type).
// This will allow the struct to be converted to an io.Reader and used as the body of an HTTP
// request.
//
// Note that incorrectly setting the value of the generic type T (for example, not setting it
// to a type consistent with the embedding struct) can lead to severe errors. Please adhere
// to the aforementioned rule.
type JSONBody[T any] struct {
	data bytes.Buffer
	once sync.Once
}

func (b *JSONBody[T]) Read(p []byte) (n int, err error) {
	b.once.Do(func() {
		var x T
		vf := reflect.ValueOf(x)
		if vf.Kind() != reflect.Struct {
			panic("use the value type of a struct rather than a pointer type as the value for generics")
		}
		vt := vf.Type()
		// estimate the size of the body in advance
		var toGrow = 0
		for i := 0; i < vt.NumField(); i++ {
			sf, sv := vt.Field(i), vf.Field(i)
			if i == 0 {
				if !sf.Anonymous || sf.Type != reflect.TypeOf(b).Elem() {
					panic("JSONBody is not the first embedded field of struct type T")
				}
			} else {
				toGrow += 8 // object keys
				switch sf.Type.Kind() {
				case reflect.String:
					toGrow += 2 + sv.Len()
				case reflect.Slice:
					toGrow += sv.Len() * 2
				default:
					toGrow += 4
				}
			}
		}
		b.data.Grow(toGrow)
		encoder := json.NewEncoder(&b.data)
		x = *(*T)(unsafe.Pointer(b))
		err = encoder.Encode(&x)
	})
	if err != nil {
		return 0, err
	}
	return b.data.Read(p)
}
