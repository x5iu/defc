package defc

import (
	"errors"
	"strings"
	"testing"
)

func TestQuoteIdentifier(t *testing.T) {
	cases := []struct {
		in      string
		dialect SQLDialect
		want    string
	}{
		{"users", DialectMySQL, "`users`"},
		{"us`ers", DialectMySQL, "`us``ers`"},
		{"users", DialectPostgres, `"users"`},
		{`us"ers`, DialectPostgres, `"us""ers"`},
		{"users", DialectSQLite, `"users"`},
	}
	for _, c := range cases {
		got, err := QuoteIdentifier(c.in, c.dialect)
		if err != nil {
			t.Errorf("QuoteIdentifier(%q,%d) err=%v", c.in, c.dialect, err)
			continue
		}
		if got != c.want {
			t.Errorf("QuoteIdentifier(%q,%d)=%q want %q", c.in, c.dialect, got, c.want)
		}
	}
}

func TestQuoteIdentifier_RejectsNUL(t *testing.T) {
	_, err := QuoteIdentifier("ab\x00cd", DialectMySQL)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrUnsafeInterpolation) {
		t.Errorf("not wrapping sentinel: %v", err)
	}
	if !strings.Contains(err.Error(), "NUL") {
		t.Errorf("msg=%q", err.Error())
	}
}
