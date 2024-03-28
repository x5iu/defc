package defc

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"testing"
)

func TestNewResponseError(t *testing.T) {
	err := NewResponseError("test", http.StatusInternalServerError, []byte("test"))
	if status := err.Status(); status != http.StatusInternalServerError {
		t.Errorf("status: %d != %d", status, http.StatusInternalServerError)
		return
	}
	if body := err.Body(); !bytes.Equal(body, []byte("test")) {
		t.Errorf("body: %s != %s", string(body), "test")
		return
	}
	expectedString := `response status code ` +
		strconv.Itoa(http.StatusInternalServerError) +
		` for 'test' with body: 

test

`
	if errorString := err.Error(); errorString != expectedString {
		t.Errorf("error: unexpected error string => %s", errorString)
		return
	}
}

func TestNewFutureResponseError(t *testing.T) {
	err := NewFutureResponseError("test", &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(bytes.NewReader([]byte("test"))),
	})
	response := err.Response()
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if !bytes.Equal(body, []byte("test")) {
		t.Errorf("body: %s != %s", string(body), "test")
		return
	}
	expectedString := "response status code " + strconv.Itoa(http.StatusInternalServerError) + " for 'test'"
	if errorString := err.Error(); errorString != expectedString {
		t.Errorf("error: unexpected error string => %s", errorString)
		return
	}
}

func TestJSON(t *testing.T) {
	body := []byte(`{"code": 200}`)
	r := &http.Response{
		Body: io.NopCloser(bytes.NewReader(body)),
	}
	j := new(JSON)
	if err := j.Err(); err != nil {
		t.Errorf("json: %s", err)
		return
	}
	if err := j.FromBytes("test", body); err != nil {
		t.Errorf("json: %s", err)
		return
	}
	if err := j.FromResponse("test", r); err != nil {
		t.Errorf("json: %s", err)
		return
	}
	if err := j.ScanValues([]any{}...); err != nil {
		t.Errorf("json: %s", err)
		return
	}
	var val struct {
		Code int `json:"code"`
	}
	if err := j.ScanValues(val); err == nil {
		t.Errorf("json: expects UnmarshalError, got nil")
		return
	} else if val.Code != 0 {
		t.Errorf("json: %v != 0", val.Code)
		return
	}
	if err := j.ScanValues(&val); err != nil {
		t.Errorf("json: %s", err)
		return
	} else if val.Code != 200 {
		t.Errorf("json: %v != 200", val.Code)
		return
	}
	defer func() {
		if err := recover(); err == nil {
			t.Errorf("json: expects Panic, got nil")
			return
		}
	}()
	if j.Break() {
		t.Errorf("json: unreachable")
		return
	}
}
