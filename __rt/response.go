package __rt

import "fmt"

type Response interface {
	Err() error
	ScanValues(...any) error
	FromBytes(string, []byte) error
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
