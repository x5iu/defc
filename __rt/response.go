package __rt

import (
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
