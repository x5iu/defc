//go:build !test || no_test
// +build !test no_test

package main

func NewArith(rpcClient *rpc.Client) Arith {
	panic("Please use `go run -tags test ...` to enable testing; " +
		"this is just a placeholder function for static analysis to proceed.")
}

func NewArithServer(impl Arith) *ArithServer {
	panic("Please use `go run -tags test ...` to enable testing; " +
		"this is just a placeholder function for static analysis to proceed.")
}

type ArithServer struct {}