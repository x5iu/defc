//go:build !no_test
// +build !no_test

package test

//go:generate defc [mode] [output] [features...] TestBuildRpc/success
type Success interface {
	Multiply(args chan int) (int, error)
	unexported()
}

//go:generate defc [mode] [output] [features...] TestBuildRpc/success_pointer_reply
type SuccessPointerReply interface {
	GetUser(id int) (*User, error)
}

type User struct {
	ID   int
	Name string
}

//go:generate defc [mode] [output] [features...] TestBuildRpc/fail_no_input
type FailNoInput interface {
	NoInput() (int, error)
}

//go:generate defc [mode] [output] [features...] TestBuildRpc/fail_too_many_inputs
type FailTooManyInputs interface {
	TooManyInputs(a, b int) (int, error)
}

//go:generate defc [mode] [output] [features...] TestBuildRpc/fail_no_output
type FailNoOutput interface {
	NoOutput(a int)
}

//go:generate defc [mode] [output] [features...] TestBuildRpc/fail_one_output
type FailOneOutput interface {
	OneOutput(a int) error
}

//go:generate defc [mode] [output] [features...] TestBuildRpc/fail_too_many_outputs
type FailTooManyOutputs interface {
	TooManyOutputs(a int) (int, int, error)
}

//go:generate defc [mode] [output] [features...] TestBuildRpc/fail_no_error
type FailNoError interface {
	NoError(a int) (int, int)
}

//go:generate defc [mode] [output] [features...] TestBuildRpc/fail_no_name_type
type FailNoNameType interface {
	NoNameType(int) (int, error)
}

//go:generate defc [mode] [output] [features...] TestBuildRpc/fail_no_type_decl
var FailNoTypeDecl struct{}

//go:generate defc [mode] [output] [features...] TestBuildRpc/fail_no_iface_type
type FailNoIfaceType struct{}

//go:generate defc [mode] [output] [features...] TestBuildRpc/success_nort
type SuccessNoRt interface {
	Multiply(args chan int) (int, error)
}
