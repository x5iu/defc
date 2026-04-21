package defc

import "errors"

// ErrUnsafeInterpolation is the shared sentinel returned (via Unwrap) from
// every strict-interpolation validator in this package. Callers wiring
// bespoke error handling should match against this sentinel with
// [errors.Is] rather than against the concrete types, so new violation
// shapes added in future releases remain matched.
var ErrUnsafeInterpolation = errors.New("defc: unsafe template interpolation")
