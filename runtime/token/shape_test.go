package token

import "testing"

func TestNormalizeShape(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"SELECT 'a'", "select ?"},
		{"SELECT 'a''b'", "select ?"},
		{`SELECT "X"`, `select "X"`},
		{"SELECT 42", "select ?"},
		{"SELECT 3.14", "select ?"},
		{"SELECT 1e2", "select ?"},
		{"SELECT ?", "select ?"},
		{"DELETE FROM t; SELECT 1", "delete from t; select ?"},
		{"SELECT x FROM t WHERE y=1", "select x from t where y=?"},
		{"SELECT--c\n1", "select ?"},
		{"SELECT/*c*/1", "select ?"},
		{"SELECT 'x'--c", "select ?"},
		{`SELECT "name" FROM t`, `select "name" from t`},
		{`SELECT "a--b" FROM t`, `select "a--b" from t`},
		{`SELECT "a'b" FROM t`, `select "a'b" from t`},
		{`SELECT '"x"' FROM t`, "select ? from t"},
		{`SELECT "a""b" FROM t`, `select "a""b" from t`},
	}
	for _, tc := range cases {
		got := NormalizeShape(tc.in)
		if got != tc.want {
			t.Errorf("NormalizeShape(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
	n1 := NormalizeShape(`SELECT "name" FROM t`)
	n2 := NormalizeShape(`SELECT "email" FROM t`)
	if n1 == n2 {
		t.Fatalf("NormalizeShape(name) == NormalizeShape(email): %q", n1)
	}
}

func TestTokenizeShapeSQL_kinds(t *testing.T) {
	sql := "SELECT 'a'--c\n1"
	toks := TokenizeShapeSQL(sql)
	var kinds []ShapeTokenKind
	for _, tok := range toks {
		kinds = append(kinds, tok.Kind)
	}
	if len(toks) < 4 {
		t.Fatalf("tokens: %+v", toks)
	}
	if kinds[0] != ShapeUnquoted || toks[0].Raw != "S" {
		t.Fatalf("first: %+v", toks[0])
	}
	var sawNum, sawStr bool
	for _, x := range toks {
		if x.Kind == ShapeNumericLiteral {
			sawNum = true
		}
		if x.Kind == ShapeSingleQuoted {
			sawStr = true
		}
	}
	if !sawNum || !sawStr {
		t.Fatalf("expected numeric and single-quoted tokens: %+v", toks)
	}
}
