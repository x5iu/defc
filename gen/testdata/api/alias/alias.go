//go:build !no_test
// +build !no_test

package alias

import (
	"context"
	"net/http"

	defc "github.com/x5iu/defc/runtime"
)

//go:generate defc [mode] [output] [features...] TestBuildApiAlias/alias/plain
type AliasPlain interface {
	Response() Generic[defc.Response, defc.FutureResponse]

	// Run POST https://localhost:port/path
	// Content-Type: application/json
	//
	// { "data": "test" }
	Run(ctx context.Context) error
}

//go:generate defc [mode] [output] [features...] TestBuildApiAlias/alias/nort
type AliasNort interface {
	Response() Generic[defc.Response, defc.FutureResponse]

	// Run POST https://localhost:port/path
	// Content-Type: application/json
	//
	// { "data": "test" }
	Run(ctx context.Context) error
}

type Generic[T any, U any] struct{}

// Ensure unused import warnings do not trigger on the http package, which the
// generated file will import when response handling is emitted.
var _ = http.MethodPost
