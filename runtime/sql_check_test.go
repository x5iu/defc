package defc

import (
	"errors"
	"testing"

	"github.com/x5iu/defc/runtime/token"
)

func TestCountPlaceholders(t *testing.T) {
	cases := map[string]int{
		"":                                      0,
		"SELECT 1":                              0,
		"SELECT ? FROM t":                       1,
		"SELECT ?, ?, ? FROM t":                 3,
		"SELECT '?' FROM t":                     0,
		`SELECT "?" FROM t`:                     0,
		"SELECT `?` FROM t":                     0,
		"SELECT 1 -- trailing ?\n":              0,
		"SELECT /* ? comment ? */ ? FROM t":     1,
		"INSERT INTO t VALUES ('a''b', ?)":      1,
		"SELECT 'can''t' FROM t WHERE x=?":      1,
		"SELECT * FROM t WHERE x='a' AND y=?":   1,
		"SELECT * FROM t -- ?\nWHERE x=? AND ?": 2,
	}
	for sql, want := range cases {
		if got := token.CountPlaceholders(sql); got != want {
			t.Errorf("CountPlaceholders(%q)=%d want %d", sql, got, want)
		}
	}
}

func TestSQLArityCheck(t *testing.T) {
	if err := SQLArityCheck("SELECT ? FROM t", 1); err != nil {
		t.Errorf("unexpected err: %v", err)
	}
	err := SQLArityCheck("SELECT ?, ? FROM t", 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrUnsafeInterpolation) {
		t.Errorf("not wrapping sentinel: %v", err)
	}
	var ae *SQLArityError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *SQLArityError, got %T", err)
	}
	if ae.Have != 1 || ae.Want != 2 {
		t.Errorf("have=%d want=%d", ae.Have, ae.Want)
	}
}
