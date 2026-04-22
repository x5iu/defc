//go:build !no_test
// +build !no_test

// Package alias is a defc test fixture exercising the pool-aliasing fix.
//
// The //go:generate directives in this file are NOT invoked by `go generate`.
// They are pseudo-directives consumed by the TestBuildApiAlias driver in
// gen/api_alias_test.go, which scans this file for lines ending with the
// current sub-test's t.Name() (e.g. "TestBuildApiAlias/alias/plain") to
// locate the target interface and feed the correct feature set into the
// generator. The [mode], [output], and [features...] tokens are deliberate
// placeholders.
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
