//go:build !test || no_test
// +build !test no_test

package main

func NewClient(*TestOptions) Client {
	panic("Please use `go run -tags test ...` to enable testing; " +
		"this is just a placeholder function for static analysis to proceed.")
}
