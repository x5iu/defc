package defc

import (
	"errors"
	"strings"
	"testing"
)

func TestHeaderValue_Accepts(t *testing.T) {
	for _, in := range []string{
		"",
		"Bearer abc123",
		"application/json; charset=utf-8",
		"tab\there", // HT is allowed
		"utf8-obs-\xc2\xa0text",
	} {
		got, err := HeaderValue(in)
		if err != nil {
			t.Errorf("HeaderValue(%q): unexpected error %v", in, err)
		}
		if got != in {
			t.Errorf("HeaderValue(%q) mutated to %q", in, got)
		}
	}
}

func TestHeaderValue_Rejects(t *testing.T) {
	cases := map[string]string{
		"crlf":    "abc\r\nX-Injected: evil",
		"cr":      "abc\revil",
		"lf":      "abc\nevil",
		"nul":     "abc\x00evil",
		"del":     "abc\x7fevil",
		"c0":      "abc\x01evil",
		"vt":      "abc\x0bevil",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := HeaderValue(in)
			if err == nil {
				t.Fatalf("HeaderValue(%q): expected error", in)
			}
			if !errors.Is(err, ErrUnsafeInterpolation) {
				t.Errorf("HeaderValue(%q): error %v not wrapping ErrUnsafeInterpolation", in, err)
			}
			var ihe *InvalidHeaderValueError
			if !errors.As(err, &ihe) {
				t.Errorf("HeaderValue(%q): expected *InvalidHeaderValueError, got %T", in, err)
			}
		})
	}
}

type stringerT string

func (s stringerT) String() string { return string(s) }

func TestPreExecHeaderMap(t *testing.T) {
	clean := map[string]any{
		"Authorization": "Bearer abcdef",
		"X-Req-Id":      []byte("abc"),
		"X-Stringer":    stringerT("hello"),
		"X-Nil":         nil,
		"X-Int":         42, // unknown type, ignored
	}
	if err := PreExecHeaderMap(clean); err != nil {
		t.Fatalf("PreExecHeaderMap(clean) err=%v", err)
	}

	dirty := map[string]any{
		"Authorization": "Bearer abc\r\nX-Injected: evil",
	}
	err := PreExecHeaderMap(dirty)
	if err == nil {
		t.Fatal("expected error on dirty map")
	}
	var ime *InvalidHeaderMapError
	if !errors.As(err, &ime) {
		t.Fatalf("expected *InvalidHeaderMapError, got %T", err)
	}
	if ime.Key != "Authorization" {
		t.Errorf("key=%q want Authorization", ime.Key)
	}
	if !strings.Contains(err.Error(), "Authorization") {
		t.Errorf("error message missing key: %s", err.Error())
	}

	if !errors.Is(err, ErrUnsafeInterpolation) {
		t.Errorf("error not wrapping ErrUnsafeInterpolation")
	}
}
