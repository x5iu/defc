package defc

import (
	"errors"
	"strings"
	"testing"
)

func TestStrictURL_Accepts(t *testing.T) {
	cases := []struct {
		prefix   string
		rendered string
	}{
		{"https://api.example.com/", "https://api.example.com/v1/widgets"},
		{"", "http://localhost:8080/healthz"},
		{"https://api.example.com", "https://api.example.com/a/b/c?x=1&y=2"},
	}
	for _, c := range cases {
		got, err := StrictURL(c.prefix, c.rendered)
		if err != nil {
			t.Errorf("StrictURL(%q,%q) err=%v", c.prefix, c.rendered, err)
			continue
		}
		if got != c.rendered {
			t.Errorf("rendered mutated: %q -> %q", c.rendered, got)
		}
	}
}

func TestStrictURL_HostValidation(t *testing.T) {
	reject := []string{
		"http:///path",
		"http:opaque",
		"https://",
	}
	for _, s := range reject {
		_, err := StrictURL("", s)
		if err == nil {
			t.Fatalf("StrictURL(%q) want error", s)
		}
		if !errors.Is(err, ErrUnsafeInterpolation) {
			t.Fatalf("StrictURL(%q) err=%v want ErrUnsafeInterpolation", s, err)
		}
	}
	_, err := StrictURL("https://API.example.com/", "https://api.example.com/path")
	if err != nil {
		t.Fatalf("StrictURL equal-fold host: %v", err)
	}
}

func TestStrictURL_Rejects(t *testing.T) {
	cases := []struct {
		name     string
		prefix   string
		rendered string
		kind     string
	}{
		{"crlf", "", "https://api.example.com/x\r\ny", "ControlChar"},
		{"space", "", "https://api.example.com/ab cd", "ControlChar"},
		{"backtick", "", "https://api.example.com/`cmd`", "ControlChar"},
		{"parse-fail", "", "ht\x01ps://broken", "ControlChar"},
		{"bad-scheme", "", "ftp://host/f", "SchemeDenied"},
		{"bad-scheme2", "", "file:///etc/passwd", "SchemeDenied"},
		{"host-drift", "https://api.example.com/", "https://evil.com/api.example.com/x", "HostDrift"},
		{"userinfo-drift", "https://api.example.com/", "https://api.example.com@evil.com/", "HostDrift"},
		{"scheme-drift", "https://api.example.com/", "http://api.example.com/x", "SchemeDrift"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := StrictURL(c.prefix, c.rendered)
			if err == nil {
				t.Fatalf("expected error for %q", c.rendered)
			}
			if !errors.Is(err, ErrUnsafeInterpolation) {
				t.Errorf("err not wrapping ErrUnsafeInterpolation: %v", err)
			}
			var ue *UnsafeURLError
			if !errors.As(err, &ue) {
				t.Fatalf("expected *UnsafeURLError, got %T (%v)", err, err)
			}
			if ue.Kind != c.kind {
				t.Errorf("kind=%s want %s (err=%v)", ue.Kind, c.kind, err)
			}
		})
	}
}

func TestPathSegment(t *testing.T) {
	if got := PathSegment("hello world"); got != "hello%20world" {
		t.Errorf("PathSegment: %q", got)
	}
	if got := PathSegment("a/b"); !strings.Contains(got, "%2F") {
		t.Errorf("PathSegment did not escape slash: %q", got)
	}
	// NUL stripped
	if got := PathSegment("ab\x00cd"); strings.Contains(got, "\x00") {
		t.Errorf("PathSegment left NUL: %q", got)
	}
}

func TestQueryValue(t *testing.T) {
	if got := QueryValue("a b&c=d"); got != "a+b%26c%3Dd" {
		t.Errorf("QueryValue: %q", got)
	}
}
