package defc

import (
	"fmt"

	"github.com/x5iu/defc/runtime/token"
)

// SQLArityError is returned by [SQLArityCheck] when the rendered SQL
// contains a number of `?` placeholders that does not match the
// caller-supplied argument count.
type SQLArityError struct {
	Have     int
	Want     int
	Rendered string
}

func (e *SQLArityError) Error() string {
	return fmt.Sprintf("defc: SQL placeholder arity mismatch: rendered has %d ?-placeholders but argList has %d arguments", e.Want, e.Have)
}

func (e *SQLArityError) Unwrap() error { return ErrUnsafeInterpolation }

// SQLArityCheck verifies that the number of `?` placeholders in the
// rendered SQL matches argc. Placeholder counting is
// comment/quote-aware via [token.CountPlaceholders]; see its docs for
// exact tokenisation rules.
//
// It should be invoked by generated code *before* [In] rewrites a
// `?` into multiple bindvars for slices, so that the caller's
// rendered-vs-arg mismatch is caught in the original form.
func SQLArityCheck(rendered string, argc int) error {
	want := token.CountPlaceholders(rendered)
	if want == argc {
		return nil
	}
	return &SQLArityError{Have: argc, Want: want, Rendered: rendered}
}
