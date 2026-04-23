//go:build test
// +build test

package integration

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	goimport "golang.org/x/tools/imports"

	"github.com/x5iu/defc/gen"
)

func runSqlxDroppedArgsModule(t *testing.T, testDir string, wantDiscarded bool) string {
	t.Helper()
	const (
		testPk      = "main"
		testFile    = "main.go"
		testGenFile = "repo.gen.go"
	)
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err = os.Chdir(pwd); err != nil {
			t.Fatal(err)
		}
	}()
	if err = os.Chdir(testDir); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(testGenFile)
	doc, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	pos := 1
	lineScanner := bytes.Split(doc, []byte{'\n'})
	for i, line := range lineScanner {
		if bytes.HasPrefix(line, []byte("//go:generate")) {
			pos = i + 1
			break
		}
	}
	generator := gen.NewCliBuilder(gen.ModeSqlx).
		WithPkg(testPk).
		WithPwd(filepath.Join(pwd, testDir)).
		WithFile(testFile, doc).
		WithPos(pos).
		WithImports(nil).
		WithFeats([]string{gen.FeatureSqlxFuture}).
		WithTemplate("").
		WithFuncs(nil)
	var buf bytes.Buffer
	if err = generator.Build(&buf); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err = os.WriteFile(testGenFile, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	code, err := goimport.Process(testGenFile, buf.Bytes(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(testGenFile, code, 0644); err != nil {
		t.Fatal(err)
	}
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = filepath.Join(pwd, testDir)
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %v\n%s", err, out)
	}
	var stdout, stderr bytes.Buffer
	run := exec.Command("go", "run", "-tags", "test", ".")
	run.Dir = filepath.Join(pwd, testDir)
	run.Stdout = &stdout
	run.Stderr = &stderr
	if err := run.Run(); err != nil {
		t.Logf("stdout:\n%s", stdout.String())
		t.Fatalf("stderr:\n%s\nerr: %v", stderr.String(), err)
	}
	s := stderr.String()
	if wantDiscarded {
		if !strings.Contains(s, "discarded") {
			t.Fatalf("stderr missing discarded, got:\n%s", s)
		}
	} else {
		if strings.Contains(s, "discarded") {
			t.Fatalf("stderr unexpectedly contained discarded, got:\n%s", s)
		}
	}
	if !strings.Contains(stdout.String(), "ok") {
		t.Fatalf("stdout want ok, got %q", stdout.String())
	}
	return s
}

func TestSqlxDroppedArgsWarning(t *testing.T) {
	s := runSqlxDroppedArgsModule(t, "dropped_args", true)
	for _, w := range []string{"DeleteByName", "discarded"} {
		if !strings.Contains(s, w) {
			t.Fatalf("stderr missing %q, got:\n%s", w, s)
		}
	}
}

func TestSqlxMultistmt_noFalsePositiveWhenPlaceholdersMatchArgs(t *testing.T) {
	runSqlxDroppedArgsModule(t, "dropped_args_ms_ok", false)
}

func TestSqlxMultistmt_emitsWhenTotalPlaceholdersLessThanArgCount(t *testing.T) {
	s := runSqlxDroppedArgsModule(t, "dropped_args_ms_short", true)
	if !strings.Contains(s, "Repo.DoShort") {
		t.Fatalf("expected Repo.DoShort in stderr, got:\n%s", s)
	}
}
