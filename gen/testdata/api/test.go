//go:build !no_test
// +build !no_test

package test

import "C"
import (
	"context"
	"net/http"

	gofmt "fmt"
	defc "github.com/x5iu/defc/runtime"

	_ "unsafe"
)

//go:generate defc [mode] [output] [features...] TestBuildApi/success
type Success[I any, R interface {
	Err() error
	ScanValues(...any) error
	FromBytes(string, []byte) error
	FromResponse(string, *http.Response) error
	Break() bool
}] interface {
	Inner() I
	Response() Generic[I, R]

	// Run POST https://localhost:port/path?{{ $.query }}
	/*
		- Content-Type: application/json; charset=utf-8
		- Authorization: Bearer XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX

		{{ $.body }}
	*/
	Run(ctx context.Context, query, body gofmt.Stringer) (*struct{}, error)
}

//go:generate defc [mode] [output] [features...] TestBuildApi/success/no_generics
type SuccessNoGenerics interface {
	Response() Generic[defc.Response, defc.FutureResponse]

	// Run GET MANY https://localhost:port/path?{{ $.query }}
	Run(ctx context.Context, query gofmt.Stringer) ([]struct{}, error)

	// Crawl GET https://localhost:port/path?{{ $.query }}
	Crawl(ctx context.Context, query gofmt.Stringer) ([]struct{}, error)
}

//go:generate defc [mode] [output] [features...] TestBuildApi/fail_no_response
type FailNoResponse interface {
	// Run POST https://localhost:port/path?{{ $.query }}
	/*
		- Content-Type: application/json; charset=utf-8
		- Authorization: Bearer XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX

		{{ $.body }}
	*/
	Run(ctx context.Context, query, body gofmt.Stringer) error
}

//go:generate defc [mode] [output] [features...] TestBuildApi/fail_no_error
type FailNoError[I any, R defc.Response] interface {
	Inner() I
	Response() R

	// Run POST https://localhost:port/path?{{ $.query }}
	/*
		- Content-Type: application/json; charset=utf-8
		- Authorization: Bearer XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX

		{{ $.body }}
	*/
	Run(ctx context.Context, query, body gofmt.Stringer)
}

//go:generate defc [mode] [output] [features...] TestBuildApi/fail_no_name_type
type FailNoNameType[I any, R defc.Response] interface {
	Inner() I
	Response() R

	// Run POST https://localhost:port/path?{{ $.query }}
	/*
		- Content-Type: application/json; charset=utf-8
		- Authorization: Bearer XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX

		{{ $.body }}
	*/
	Run(context.Context, gofmt.Stringer) error
}

//go:generate defc [mode] [output] [features...] TestBuildApi/fail_invalid_IR/I
type FailInvalidI[I any, R defc.Response] interface {
	Inner(_ struct{}) I
	Response() R
}

//go:generate defc [mode] [output] [features...] TestBuildApi/fail_invalid_IR/R
type FailInvalidR[I any, R defc.Response] interface {
	Inner() I
	Response(_ struct{}) R
}

//go:generate defc [mode] [output] [features...] TestBuildApi/fail_invalid_IR/R/type
type FailInvalidRType[I any, R defc.Response] interface {
	Inner() I
	Response() struct{}
}

//go:generate defc [mode] [output] [features...] TestBuildApi/fail_no_type_decl
var FailNoTypeDecl struct{}

//go:generate defc [mode] [output] [features...] TestBuildApi/fail_no_iface_type
type FailNoIfaceType struct{}

type Generic[T any, U any] struct{}
