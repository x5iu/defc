package defc

import (
	"fmt"
)

// InvalidHeaderValueError is returned by [HeaderValue] when the value
// contains a byte that is forbidden in an HTTP field-value (RFC 9110
// §5.5). It wraps [ErrUnsafeInterpolation].
type InvalidHeaderValueError struct {
	Offset int
	Byte   byte
}

func (e *InvalidHeaderValueError) Error() string {
	return fmt.Sprintf("defc: header value contains forbidden byte 0x%02x at offset %d", e.Byte, e.Offset)
}

func (e *InvalidHeaderValueError) Unwrap() error { return ErrUnsafeInterpolation }

// InvalidHeaderMapError wraps an [InvalidHeaderValueError] with the
// offending map key from [PreExecHeaderMap].
type InvalidHeaderMapError struct {
	Key string
	Err error
}

func (e *InvalidHeaderMapError) Error() string {
	return fmt.Sprintf("defc: header map key %q: %s", e.Key, e.Err.Error())
}

func (e *InvalidHeaderMapError) Unwrap() error { return e.Err }

// HeaderValue validates v as a safe HTTP header field-value.
//
// The returned string is byte-for-byte equal to v when v contains only
// visible-ASCII (0x21..0x7E), space (0x20), horizontal tab (0x09), and
// UTF-8 obs-text bytes (>= 0x80), per RFC 9110. Any CR, LF, NUL, other
// C0 control, or DEL triggers
// an error. This is a fail-closed validator — values are never
// silently stripped, because silent stripping creates "valid looking"
// headers with attacker-chosen prefixes and no audit trail.
//
// Complexity is O(len(v)); zero allocations on the happy path.
func HeaderValue(v string) (string, error) {
	for i := 0; i < len(v); i++ {
		b := v[i]
		switch {
		case b == '\t':
			// HT is the only allowed control byte in HTTP headers.
		case b < 0x20:
			return "", &InvalidHeaderValueError{Offset: i, Byte: b}
		case b == 0x7f:
			return "", &InvalidHeaderValueError{Offset: i, Byte: b}
		}
	}
	return v, nil
}

// PreExecHeaderMap walks the map that a generated API method is about
// to pass to its `headerTmpl.Execute`. For every entry whose dynamic
// type is string, []byte, or fmt.Stringer, [HeaderValue] is applied.
// The first offending entry short-circuits and is wrapped in an
// [InvalidHeaderMapError] carrying the map key.
//
// Entries of other types (numbers, structs meant to be reached via
// {{.x.Field}}) are skipped silently; the generator is expected to
// route those through the `header` template helper instead.
func PreExecHeaderMap(m map[string]any) error {
	for k, v := range m {
		if err := preExecHeaderOne(v); err != nil {
			return &InvalidHeaderMapError{Key: k, Err: err}
		}
	}
	return nil
}

func preExecHeaderOne(v any) error {
	switch t := v.(type) {
	case nil:
		return nil
	case string:
		_, err := HeaderValue(t)
		return err
	case []byte:
		// TODO(perf): byte-sweep variant to avoid []byte → string alloc
		_, err := HeaderValue(string(t))
		return err
	case fmt.Stringer:
		_, err := HeaderValue(t.String())
		return err
	default:
		return nil
	}
}
