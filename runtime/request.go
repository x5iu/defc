package defc

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"unsafe"

	"github.com/x5iu/defc/runtime/token"
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

// MultipartBody is a shortcut type used for quickly constructing an io.Reader.
//
// Embed MultipartBody as the first field in the struct, and set the generic parameter of
// MultipartBody to the type of the embedded struct (ensure to use a value type rather than a
// pointer type). This will allow the struct to be converted to an io.Reader and used as the
// body of an HTTP request.
//
// Use the ContentType method to obtain the Content-Type Header with a boundary. Use the "form"
// tag to specify the name of fields in multipart/form-data, its usage is similar to the
// encoding/json package, and it also supports the omitempty syntax.
//
// For file types, you can directly use os.File as a value, or you can use types that implement
// the namedReader interface (os.File has implemented the namedReader interface).
//
// Note that incorrectly setting the value of the generic type T (for example, not setting it
// to a type consistent with the embedding struct) can lead to severe errors. Please adhere
// to the aforementioned rule.
type MultipartBody[T any] struct {
	reader   io.Reader
	boundary string
	once     sync.Once
}

func (b *MultipartBody[T]) getBoundary() string {
	if b.boundary == "" {
		var buf [32]byte
		io.ReadFull(rand.Reader, buf[:])
		b.boundary = hex.EncodeToString(buf[:])
	}
	return b.boundary
}

func (b *MultipartBody[T]) ContentType() string {
	boundary := b.getBoundary()
	if strings.ContainsAny(boundary, `()<>@,;:\"/[]?= `) {
		boundary = `"` + boundary + `"`
	}
	return "multipart/form-data; boundary=" + boundary
}

type namedReader interface {
	io.Reader
	Name() string
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

func (b *MultipartBody[T]) Read(p []byte) (n int, err error) {
	b.once.Do(func() {
		var x T
		x = *(*T)(unsafe.Pointer(b))
		s := &fieldScanner{tag: "form", val: reflect.ValueOf(x)}
		if s.val.Kind() != reflect.Struct {
			panic("use the value type of a struct rather than a pointer type as the value for generics")
		}
		if !s.CheckFirstEmbedType(reflect.TypeOf(b).Elem()) {
			panic("MultipartBody is not the first embedded field of struct type T")
		}
		readers := make([]io.Reader, 0, s.val.NumField())
		for i := 0; s.Scan(); i++ {
			tag := s.Tag()
			if tag != "-" && s.Exported() {
				if tagContains(tag, "omitempty") && s.Empty() {
					continue
				}
				var buf bytes.Buffer
				if i == 0 {
					buf.WriteString("--" + b.getBoundary() + "\r\n")
				} else {
					buf.WriteString("\r\n--" + b.getBoundary() + "\r\n")
				}
				val := s.Val()
				if file, ok := val.(namedReader); ok {
					buf.WriteString(fmt.Sprintf(`Content-Disposition: form-data; name="%s"; filename="%s"`+"\r\n",
						escapeQuotes(getTag(tag)),
						escapeQuotes(file.Name())))
					buf.WriteString("Content-Type: application/octet-stream\r\n\r\n")
					readers = append(readers, &buf)
					readers = append(readers, file)
				} else {
					var fieldvalue string
					if fieldvalue, ok = val.(string); !ok {
						fieldvalue = fmt.Sprintf("%v", val)
					}
					buf.WriteString(fmt.Sprintf(`Content-Disposition: form-data; name="%s"`+"\r\n\r\n",
						escapeQuotes(getTag(tag))))
					buf.WriteString(fieldvalue)
					readers = append(readers, &buf)
				}
			}
		}
		readers = append(readers, strings.NewReader("\r\n--"+b.getBoundary()+"--\r\n"))
		b.reader = io.MultiReader(readers...)
	})
	return b.reader.Read(p)
}

type fieldScanner struct {
	tag string
	val reflect.Value
	typ reflect.Type
	idx int
}

func (s *fieldScanner) CheckFirstEmbedType(target reflect.Type) bool {
	if s.typ == nil {
		s.typ = s.val.Type()
	}
	if s.typ.NumField() == 0 {
		return false
	}
	sf := s.typ.Field(0)
	return sf.Anonymous && sf.Type == target
}

func (s *fieldScanner) pos() int {
	return s.idx - 1
}

func (s *fieldScanner) Scan() bool {
	if s.val.Kind() != reflect.Struct {
		return false
	}
	if s.typ == nil {
		s.typ = s.val.Type()
	}
	s.idx++
	if s.pos() >= s.typ.NumField() {
		return false
	}
	if sf := s.typ.Field(s.pos()); sf.Anonymous {
		s.idx++
	}
	return s.pos() < s.typ.NumField()
}

func (s *fieldScanner) Tag() string {
	return s.typ.Field(s.pos()).Tag.Get(s.tag)
}

func (s *fieldScanner) Val() any {
	return s.val.Field(s.pos()).Interface()
}

func (s *fieldScanner) Empty() bool {
	return s.val.Field(s.pos()).IsZero()
}

func (s *fieldScanner) Exported() bool {
	return s.typ.Field(s.pos()).IsExported()
}

func getTag(tag string) string {
	tag, _, _ = strings.Cut(tag, token.Comma)
	return tag
}

func tagContains(tag string, option string) bool {
	parts := strings.Split(tag, token.Comma)
	if len(parts) <= 1 {
		return false
	}
	options := parts[1:]
	for _, o := range options {
		if strings.TrimSpace(o) == option {
			return true
		}
	}
	return false
}
