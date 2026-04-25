package gen

import (
	"strings"
	"testing"
)

func TestSplitApiHeaderAndBody(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		wantHeader string
		wantBody   string
	}{
		{"crlfcrlf_separator", "A: 1\r\n\r\nBODY", "A: 1", "BODY"},
		{"lf_separator", "A: 1\n\nBODY", "A: 1", "BODY"},
		{"no_separator", "A: 1\r\n", "A: 1", ""},
		{"whitespace_only", "   \t\r\n", "", ""},
		{"crlf_takes_precedence_over_lf", "A: 1\r\n\r\nB1\n\nB2", "A: 1", "B1\n\nB2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotH, gotB := splitApiHeaderAndBody(tt.raw)
			if gotH != tt.wantHeader || gotB != tt.wantBody {
				t.Fatalf("splitApiHeaderAndBody(%q) = (%q, %q) want (%q, %q)", tt.raw, gotH, gotB, tt.wantHeader, tt.wantBody)
			}
		})
	}
}

func TestParseApiHeaderSpec(t *testing.T) {
	t.Run("header_only", func(t *testing.T) {
		fields, body, err := parseApiHeaderSpec("Content-Type: application/json\r\n\r\n")
		if err != nil {
			t.Fatal(err)
		}
		if body != "" || len(fields) != 1 || fields[0].Name != "Content-Type" || fields[0].Value != "application/json" {
			t.Fatalf("got %+v body=%q", fields, body)
		}
	})
	t.Run("header_and_body", func(t *testing.T) {
		raw := "X-A: 1\r\n\r\n{{ .b }}\r\n"
		fields, body, err := parseApiHeaderSpec(raw)
		if err != nil {
			t.Fatal(err)
		}
		if len(fields) != 1 || fields[0].Name != "X-A" || fields[0].Value != "1" || body != "{{ .b }}" {
			t.Fatalf("got %+v body=%q", fields, body)
		}
	})
	t.Run("repeated_keys", func(t *testing.T) {
		raw := "Set-Cookie: a=b\r\nSet-Cookie: c=d\r\n\r\n"
		fields, body, err := parseApiHeaderSpec(raw)
		if err != nil {
			t.Fatal(err)
		}
		if body != "" || len(fields) != 2 || fields[0].Name != "Set-Cookie" || fields[1].Name != "Set-Cookie" {
			t.Fatalf("got %+v", fields)
		}
	})
	t.Run("value_colon", func(t *testing.T) {
		fields, _, err := parseApiHeaderSpec("Authorization: Bearer a:b:c\r\n\r\n")
		if err != nil {
			t.Fatal(err)
		}
		if len(fields) != 1 || fields[0].Value != "Bearer a:b:c" {
			t.Fatalf("got %+v", fields)
		}
	})
	t.Run("minus_prefix", func(t *testing.T) {
		fields, _, err := parseApiHeaderSpec("- X-Custom: z\r\n\r\n")
		if err != nil {
			t.Fatal(err)
		}
		if len(fields) != 1 || fields[0].Name != "X-Custom" || fields[0].Value != "z" {
			t.Fatalf("got %+v", fields)
		}
	})
	t.Run("reject_dynamic_key", func(t *testing.T) {
		_, _, err := parseApiHeaderSpec("{{ .k }}: v\r\n\r\n")
		if err == nil || !strings.Contains(err.Error(), "static") {
			t.Fatalf("want static key error, got %v", err)
		}
	})
	t.Run("reject_invalid_key_space", func(t *testing.T) {
		_, _, err := parseApiHeaderSpec("Bad Name: x\r\n\r\n")
		if err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("reject_no_colon", func(t *testing.T) {
		_, _, err := parseApiHeaderSpec("nocolon line\r\n\r\n")
		if err == nil || !strings.Contains(err.Error(), "':'") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("lf_only_separator", func(t *testing.T) {
		fields, body, err := parseApiHeaderSpec("X-A: 1\n\n{{ .b }}")
		if err != nil {
			t.Fatal(err)
		}
		if len(fields) != 1 || fields[0].Name != "X-A" || fields[0].Value != "1" || body != "{{ .b }}" {
			t.Fatalf("got %+v body=%q", fields, body)
		}
	})
	t.Run("empty_header_block_returns_no_fields", func(t *testing.T) {
		fields, body, err := parseApiHeaderSpec("   ")
		if err != nil {
			t.Fatal(err)
		}
		if len(fields) != 0 || body != "" {
			t.Fatalf("got %+v body=%q err=%v", fields, body, err)
		}
	})
}
