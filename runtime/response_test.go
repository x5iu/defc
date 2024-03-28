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
