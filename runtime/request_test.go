package defc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type testRequestJSONBody struct {
	JSONBody[testRequestJSONBody]
	Code    string `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
	List    []int  `json:"list"`
}

type testErrRequestJSONBody struct {
	JSONBody[testErrRequestJSONBody]
}

func (*testErrRequestJSONBody) MarshalJSON() ([]byte, error) {
	return nil, errors.New("test_err_request_body_unmarshal_error")
}

type testPanicRequestJSONBody struct {
	JSONBody[*testPanicRequestJSONBody]
}

type testDislocationRequestJSONBody struct {
	Code string `json:"code"`
	JSONBody[testDislocationRequestJSONBody]
}

func TestJSONBody(t *testing.T) {
	want := map[string]any{
		"code":    "test",
		"message": "0408",
		"success": true,
		"list":    []any{1.0, 2.0, 3.0},
	}
	var r io.Reader = &testRequestJSONBody{
		Code:    "test",
		Message: "0408",
		Success: true,
		List:    []int{1, 2, 3},
	}
	raw, _ := io.ReadAll(r)
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Errorf("json_body: %s", err)
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("json_body: got => %v", got)
		return
	}
	r = &testErrRequestJSONBody{}
	if unexpected, err := io.ReadAll(r); err == nil {
		t.Errorf("json_body: expects errors, got nil, unexpected => %s", string(unexpected))
		return
	} else if !strings.Contains(err.Error(), "test_err_request_body_unmarshal_error") {
		t.Errorf("json_body: expects Error, got => %s", err)
		return
	}
	t.Run("kind_panic", func(t *testing.T) {
		var r io.Reader
		r = &testPanicRequestJSONBody{}
		defer func() {
			if rec := recover(); rec == nil {
				t.Errorf("json_body: expects Panic, got nil")
				return
			} else if lit, ok := rec.(string); !ok || lit != "use the value type of a struct rather than a pointer type as the value for generics" {
				t.Errorf("json_body: unexpected Panic literal => %s", rec)
				return
			}
		}()
		_, _ = io.ReadAll(r)
	})
	t.Run("loc_panic", func(t *testing.T) {
		var r io.Reader
		r = &testDislocationRequestJSONBody{Code: "test"}
		defer func() {
			if rec := recover(); rec == nil {
				t.Errorf("json_body: expects Panic, got nil")
				return
			} else if lit, ok := rec.(string); !ok || lit != "JSONBody is not the first embedded field of struct type T" {
				t.Errorf("json_body: unexpected Panic literal => %s", rec)
				return
			}
		}()
		_, _ = io.ReadAll(r)
	})
}

type testRequestMultipartBody struct {
	MultipartBody[testRequestMultipartBody]
	File *os.File `form:"file"`
	Name string   `form:"name"`
	N    int      `form:"n"`
}

type testPanicRequestMultipartBody struct {
	MultipartBody[*testPanicRequestMultipartBody]
}

type testDislocationRequestMultipartBody struct {
	Name string `form:"name"`
	MultipartBody[testDislocationRequestMultipartBody]
}

func TestMultipartBody(t *testing.T) {
	file, err := os.Open("request_test.go")
	if err != nil {
		t.Errorf("multipart_body: %s", err)
		return
	}
	defer file.Close()
	r := &testRequestMultipartBody{
		File: file,
		Name: "test",
		N:    1,
	}
	var buf bytes.Buffer
	multipartWriter := multipart.NewWriter(&buf)
	if err = multipartWriter.SetBoundary(r.getBoundary()); err != nil {
		t.Errorf("multipart_body: %s", err)
		return
	}
	w, err := multipartWriter.CreateFormFile("file", filepath.Base(file.Name()))
	if err != nil {
		t.Errorf("multipart_body: %s", err)
		return
	}
	io.Copy(w, file)
	if err = multipartWriter.WriteField("name", "test"); err != nil {
		t.Errorf("multipart_body: %s", err)
		return
	}
	if err = multipartWriter.WriteField("n", "1"); err != nil {
		t.Errorf("multipart_body: %s", err)
		return
	}
	multipartWriter.Close()
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		t.Errorf("multipart_body: %s", err)
		return
	}
	requestBody, _ := io.ReadAll(r)
	if !bytes.Equal(requestBody, buf.Bytes()) {
		t.Errorf("multipart_body: unexptected body => \n\n%s", string(requestBody))
		return
	}
	t.Run("content_type", func(t *testing.T) {
		r := &testRequestMultipartBody{}
		r.boundary = r.getBoundary() + "@"
		if want := fmt.Sprintf(`multipart/form-data; boundary="%s"`, r.boundary); r.ContentType() != want {
			t.Errorf("multipart_body: unexptected Content-Type => \n\nwant: %s\ngot: %s", want, r.ContentType())
			return
		}
	})
	t.Run("kind_panic", func(t *testing.T) {
		r := &testPanicRequestMultipartBody{}
		defer func() {
			if rec := recover(); rec == nil {
				t.Errorf("multipart_body: expects Panic, got nil")
				return
			} else if lit, ok := rec.(string); !ok || lit != "use the value type of a struct rather than a pointer type as the value for generics" {
				t.Errorf("multipart_body: unexpected Panic literal => %s", rec)
				return
			}
		}()
		_, _ = io.ReadAll(r)
	})
	t.Run("loc_panic", func(t *testing.T) {
		r := &testDislocationRequestMultipartBody{Name: "test"}
		defer func() {
			if rec := recover(); rec == nil {
				t.Errorf("multipart_body: expects Panic, got nil")
				return
			} else if lit, ok := rec.(string); !ok || lit != "MultipartBody is not the first embedded field of struct type T" {
				t.Errorf("multipart_body: unexpected Panic literal => %s", rec)
				return
			}
		}()
		_, _ = io.ReadAll(r)
	})
}

func TestFieldScanner(t *testing.T) {
	var s *fieldScanner
	s = &fieldScanner{tag: "test", val: reflect.ValueOf("test")}
	for i := 0; i < 8; i++ {
		if s.Scan() != false {
			t.Errorf("field_scanner: expects s.Scan() == false, got true")
			return
		}
	}
	s = &fieldScanner{tag: "test", val: reflect.ValueOf(struct{}{})}
	if s.Scan() != false {
		t.Errorf("field_scanner: expects s.Scan() == false, got true")
		return
	}
	if s.CheckFirstEmbedType(nil) != false {
		t.Errorf("field_scanner: expects s.CheckFirstEmbedType() == false, got true")
		return
	}
}
