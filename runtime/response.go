package defc

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Response interface {
	Err() error
	ScanValues(...any) error
	FromBytes(string, []byte) error
	Break() bool
}

// FutureResponse represents Response interface which would be used in next
// major version of defc, who may cause breaking changes.
type FutureResponse interface {
	Err() error
	ScanValues(...any) error
	FromResponse(string, *http.Response) error
	Break() bool
}

type ResponseError interface {
	error
	Status() int
	Body() []byte
}

func NewResponseError(caller string, status int, body []byte) ResponseError {
	return &implResponseError{
		caller: caller,
		status: status,
		body:   body,
	}
}

type implResponseError struct {
	caller string
	status int
	body   []byte
}

func (e *implResponseError) Error() string {
	return fmt.Sprintf("response status code %d for '%s' with body: \n\n%s\n\n", e.status, e.caller, string(e.body))
}

func (e *implResponseError) Status() int {
	return e.status
}

func (e *implResponseError) Body() []byte {
	return e.body
}

// FutureResponseError represents Response error interface which would be used
// in next major version of defc, who may cause breaking changes.
type FutureResponseError interface {
	error
	Response() *http.Response
}

func NewFutureResponseError(caller string, response *http.Response) FutureResponseError {
	return &implFutureResponseError{
		caller:   caller,
		response: response,
	}
}

type implFutureResponseError struct {
	caller   string
	response *http.Response
}

func (e *implFutureResponseError) Error() string {
	return fmt.Sprintf("response status code %d for '%s'", e.response.StatusCode, e.caller)
}

func (e *implFutureResponseError) Response() *http.Response {
	return e.response
}

// JSON is a Response handler that quickly adapts to interfaces with Content-Type: application/json.
// You can directly use *JSON as the return type for response methods in the API Schema to handle
// the JSON data returned by the interface.
//
// NOTE: Not suitable for pagination query interfaces. If your interface involves pagination queries,
// please implement a custom Response handler.
type JSON struct {
	Raw json.RawMessage
}

func (j *JSON) Err() error {
	return nil
}

func (j *JSON) FromBytes(_ string, bytes []byte) error {
	j.Raw = bytes
	return nil
}

func (j *JSON) FromResponse(_ string, r *http.Response) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	return decoder.Decode(&j.Raw)
}

func (j *JSON) ScanValues(vs ...any) error {
	for _, v := range vs {
		if err := json.Unmarshal(j.Raw, v); err != nil {
			return err
		}
	}
	return nil
}

func (j *JSON) Break() bool {
	panic("JSON is not well-suited for pagination query requests")
}
