package defc

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

func TestWarnDroppedArgs_emitsWhenCollectedGreaterThanUsed(t *testing.T) {
	t.Cleanup(func() { resetDroppedArgWarnStateForTest() })
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	WarnDroppedArgs("R.FindUser", "SELECT id FROM user WHERE name = 'x'", 2, 0)
	out := buf.String()
	if !strings.Contains(out, `method "R.FindUser"`) {
		t.Fatalf("missing method name: %q", out)
	}
	if !strings.Contains(out, "consumed 0 argument(s) but 2 argument(s)") {
		t.Fatalf("missing counts: %q", out)
	}
	if !strings.Contains(out, "2 argument(s) were discarded") {
		t.Fatalf("missing discarded: %q", out)
	}
}

func TestWarnDroppedArgs_silentWhenEqual(t *testing.T) {
	t.Cleanup(func() { resetDroppedArgWarnStateForTest() })
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	WarnDroppedArgs("Q", "SELECT ? FROM t", 1, 1)
	if buf.Len() != 0 {
		t.Fatalf("unexpected output: %q", buf.String())
	}
}

func TestWarnDroppedArgs_oncePerMethodAndSameShape(t *testing.T) {
	t.Cleanup(func() { resetDroppedArgWarnStateForTest() })
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	for i := 0; i < 10; i++ {
		WarnDroppedArgs("A.Repeat", "SELECT 1", 2, 0)
	}
	if strings.Count(buf.String(), "defc: method") != 1 {
		t.Fatalf("expected single defc: method line, got %q", buf.String())
	}
}

func TestWarnDroppedArgs_sameMethodSameShapeDifferentLiterals_oneWarning(t *testing.T) {
	t.Cleanup(func() { resetDroppedArgWarnStateForTest() })
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	WarnDroppedArgs("M.X", "SELECT * FROM t WHERE name = 'alice'", 2, 0)
	WarnDroppedArgs("M.X", "SELECT * FROM t WHERE name = 'bob'", 2, 0)
	if strings.Count(buf.String(), "defc: method") != 1 {
		t.Fatalf("expected 1 defc: method line, got %q", buf.String())
	}
}

func TestWarnDroppedArgs_sameMethodWhitespaceCommentCaseNumeric_oneWarning(t *testing.T) {
	t.Cleanup(func() { resetDroppedArgWarnStateForTest() })
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	WarnDroppedArgs("M.Y", "SELECT 123 FROM t", 2, 0)
	WarnDroppedArgs("M.Y", "  SeLeCt\n/*c*/\t123 FROM t  ", 2, 0)
	if strings.Count(buf.String(), "defc: method") != 1 {
		t.Fatalf("expected 1 defc: method line, got %q", buf.String())
	}
}

func TestWarnDroppedArgs_sameMethodDifferentShapes_twoWarnings(t *testing.T) {
	t.Cleanup(func() { resetDroppedArgWarnStateForTest() })
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	WarnDroppedArgs("M.Z", "DELETE FROM user WHERE id = '1'", 2, 0)
	WarnDroppedArgs("M.Z", "UPDATE user SET name = 'x' WHERE id = '1'", 2, 0)
	if strings.Count(buf.String(), "defc: method") != 2 {
		t.Fatalf("expected 2 defc: method lines, got %q", buf.String())
	}
}

func TestWarnDroppedArgs_separateWarningForDifferentDoubleQuotedIdentifiers(t *testing.T) {
	t.Cleanup(func() { resetDroppedArgWarnStateForTest() })
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	WarnDroppedArgs("M.Q", `SELECT "name" FROM user WHERE id = 1`, 2, 0)
	WarnDroppedArgs("M.Q", `SELECT "email" FROM user WHERE id = 2`, 2, 0)
	if strings.Count(buf.String(), "defc: method") != 2 {
		t.Fatalf("expected 2 defc: method lines, got %q", buf.String())
	}
}

func TestWarnDroppedArgs_fqKeyDoesNotCoalesce(t *testing.T) {
	t.Cleanup(func() { resetDroppedArgWarnStateForTest() })
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	WarnDroppedArgs("One.Find", "SELECT 1", 2, 0)
	WarnDroppedArgs("Two.Find", "SELECT 1", 2, 0)
	if strings.Count(buf.String(), "defc: method") != 2 {
		t.Fatalf("expected two defc: method lines, got %q", buf.String())
	}
}

func TestWarnDroppedArgs_sqlPreviewTruncates(t *testing.T) {
	t.Cleanup(func() { resetDroppedArgWarnStateForTest() })
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	longSQL := strings.Repeat("a", 250)
	WarnDroppedArgs("T", longSQL, 2, 0)
	out := buf.String()
	if !strings.Contains(out, "...") {
		t.Fatalf("expected ellipsis in preview: %q", out)
	}
	if !strings.Contains(out, strings.Repeat("a", 180)) {
		t.Fatalf("expected long preview prefix in output: %q", out)
	}
}

func TestNormalizeSQLShape(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"SELECT 'a'", "select ?"},
		{"SELECT 'a''b'", "select ?"},
		{"SELECT \"X\"", "select \"X\""},
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
		got := normalizeSQLShape(tc.in)
		if got != tc.want {
			t.Errorf("normalizeSQLShape(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
	n1 := normalizeSQLShape(`SELECT "name" FROM t`)
	n2 := normalizeSQLShape(`SELECT "email" FROM t`)
	if n1 == n2 {
		t.Fatalf("normalizeSQLShape(name) == normalizeSQLShape(email): %q", n1)
	}
}
