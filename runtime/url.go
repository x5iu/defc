package defc

import (
	"fmt"
	"net/url"
	"strings"
)

// UnsafeURLError categorises [StrictURL] violations.
type UnsafeURLError struct {
	Kind   string
	Detail string
}

func (e *UnsafeURLError) Error() string {
	return fmt.Sprintf("defc: strict URL check: %s: %s", e.Kind, e.Detail)
}

func (e *UnsafeURLError) Unwrap() error { return ErrUnsafeInterpolation }

// StrictURL validates the runtime-rendered URL against a
// generator-time constant prefix extracted from method.TmplURL. Checks
// are additive and fail-closed:
//
//  1. Byte sweep: rejects CR, LF, NUL, other C0 controls, DEL, space
//     and structural delimiters (", <, >, {, }, backtick) that would
//     indicate smuggled bytes even before URL parsing. Note: apostrophe
//     is permitted (common in legitimate URLs); dedicated quoting via
//     [QueryValue]/[PathSegment] applies when callers need it.
//  2. url.Parse must succeed.
//  3. u.Scheme ∈ {http, https}.
//  4. When constantPrefix is non-empty, u.Scheme/u.Host must match
//     those parsed out of constantPrefix, catching authority-drift
//     payloads of the form "@evil.com/" injected at the start of the
//     path.
//
// Returns rendered unchanged on success.
func StrictURL(constantPrefix, rendered string) (string, error) {
	if err := sweepURLBytes(rendered); err != nil {
		return "", err
	}
	u, err := url.Parse(rendered)
	if err != nil {
		return "", &UnsafeURLError{Kind: "ParseFailed", Detail: err.Error()}
	}
	switch u.Scheme {
	case "http", "https":
	default:
		return "", &UnsafeURLError{Kind: "SchemeDenied", Detail: fmt.Sprintf("scheme %q not in {http,https}", u.Scheme)}
	}
	if u.Host == "" || u.Opaque != "" {
		return "", fmt.Errorf("%w: url must have a non-empty host and no opaque form: %q",
			ErrUnsafeInterpolation, rendered)
	}
	if constantPrefix != "" {
		p, perr := url.Parse(constantPrefix)
		if perr == nil && p.Scheme != "" && p.Host != "" {
			if u.Scheme != p.Scheme {
				return "", &UnsafeURLError{Kind: "SchemeDrift", Detail: fmt.Sprintf("scheme %q != expected %q", u.Scheme, p.Scheme)}
			}
			if !strings.EqualFold(u.Host, p.Host) {
				return "", &UnsafeURLError{Kind: "HostDrift", Detail: fmt.Sprintf("host %q != expected %q", u.Host, p.Host)}
			}
		}
	}
	return rendered, nil
}

func sweepURLBytes(s string) error {
	for i := 0; i < len(s); i++ {
		b := s[i]
		switch {
		case b < 0x20, b == 0x7f:
			return &UnsafeURLError{Kind: "ControlChar", Detail: fmt.Sprintf("forbidden byte 0x%02x at offset %d", b, i)}
		case b == ' ', b == '"', b == '<', b == '>', b == '{', b == '}', b == '`':
			return &UnsafeURLError{Kind: "ControlChar", Detail: fmt.Sprintf("forbidden byte 0x%02x at offset %d", b, i)}
		}
	}
	return nil
}

// PathSegment returns a percent-encoded URL path segment suitable for
// inlining in a path between two literal slashes. NUL bytes are
// stripped prior to encoding; other C0 controls are percent-encoded
// by [url.PathEscape] per RFC 3986 and rendered inert on the wire.
func PathSegment(v string) string {
	for i := 0; i < len(v); i++ {
		if v[i] == 0x00 {
			return url.PathEscape(strings.ReplaceAll(v, "\x00", ""))
		}
	}
	return url.PathEscape(v)
}

// QueryValue returns a percent-encoded URL query value.
func QueryValue(v string) string {
	return url.QueryEscape(v)
}
