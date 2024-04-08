package defc

import (
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

type testRequestBody struct {
	JSONBody[testRequestBody]
	Code    string `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
	List    []int  `json:"list"`
}

type testErrRequestBody struct {
	JSONBody[testErrRequestBody]
}

func (*testErrRequestBody) MarshalJSON() ([]byte, error) {
	return nil, errors.New("test_err_request_body_unmarshal_error")
}

type testPanicReqeustBody struct {
	JSONBody[*testPanicReqeustBody]
}

type testDislocationRequestBody struct {
	Code string `json:"code"`
	JSONBody[testDislocationRequestBody]
}

func TestJSONBody(t *testing.T) {
	want := map[string]any{
		"code":    "test",
		"message": "0408",
		"success": true,
		"list":    []any{1.0, 2.0, 3.0},
	}
	var r io.Reader = &testRequestBody{
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
	r = &testErrRequestBody{}
	if unexpected, err := io.ReadAll(r); err == nil {
		t.Errorf("json_body: expects errors, got nil, unexpected => %s", string(unexpected))
		return
	} else if !strings.Contains(err.Error(), "test_err_request_body_unmarshal_error") {
		t.Errorf("json_body: expects Error, got => %s", err)
		return
	}
	t.Run("kind_panic", func(t *testing.T) {
		var r io.Reader
		r = &testPanicReqeustBody{}
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
		r = &testDislocationRequestBody{Code: "test"}
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
