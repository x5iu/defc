//go:build !no_test
// +build !no_test

package test

import "C"
import (
	"context"
	"database/sql"

	gofmt "fmt"

	_ "unsafe"
)

//go:generate defc [mode] [output] [features...] TestBuildSqlx/success
type Success interface {
	gofmt.GoStringer
	WithTx(context.Context, func(tx Success) error) error

	// Run exec bind
	// #include "test.sql"
	/*
		#script cat
		 "test.sql"
	*/
	// {{ $.query }}
	Run(ctx context.Context, query gofmt.Stringer) error

	// C query one bind
	// SELECT * FROM C WHERE type = {{ bind $.c }};
	C(c *C.char) (struct{}, error)
}

//go:generate defc [mode] [output] [features...] TestBuildSqlx/fail_no_error
type FailNoError interface {
	// Run exec bind
	// {{ $.query }}
	Run(ctx context.Context, query gofmt.Stringer) sql.Result
}

//go:generate defc [mode] [output] [features...] TestBuildSqlx/fail_single_scan
type FailSingleScan interface {
	// Run exec bind scan(obj)
	// {{ $.query }}
	Run(ctx context.Context, obj any, query gofmt.Stringer) (sql.Result, error)
}

//go:generate defc [mode] [output] [features...] TestBuildSqlx/fail_2_values
type Fail2Values interface {
	// Run exec bind
	// {{ $.query }}
	Run(ctx context.Context, query gofmt.Stringer) (sql.Result, struct{}, error)
}

//go:generate defc [mode] [output] [features...] TestBuildSqlx/fail_no_name_type
type FailNoNameType interface {
	// Run exec bind
	// {{ $.query }}
	Run(context.Context, gofmt.Stringer) (sql.Result, error)
}

//go:generate defc [mode] [output] [features...] TestBuildSqlx/fail_no_type_decl
var FailNoTypeDecl struct{}

//go:generate defc [mode] [output] [features...] TestBuildSqlx/fail_no_iface_type
type FailNoIfaceType struct{}
