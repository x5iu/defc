//go:build test
// +build test

package integration

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/x5iu/defc/gen"
)

func TestCodegenUnsafeInterpWarningStderr(t *testing.T) {
	const doc = `package main

//go:generate noop
type Repo interface {
	// Bad query
	// SELECT * FROM users WHERE name = '{{ .name }}';
	Bad(ctx context.Context, name string) error
}
`
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	old := os.Stderr
	os.Stderr = w
	var buf bytes.Buffer
	dir := t.TempDir()
	b := gen.NewCliBuilder(gen.ModeSqlx).
		WithPkg("main").
		WithPwd(dir).
		WithFile("schema.go", []byte(doc)).
		WithPos(3).
		WithImports(nil).
		WithFeats(nil).
		WithTemplate("").
		WithFuncs(nil)
	errBuild := b.Build(&buf)
	if cerr := w.Close(); cerr != nil {
		t.Error(cerr)
	}
	os.Stderr = old
	var stderr bytes.Buffer
	if _, err := io.Copy(&stderr, r); err != nil {
		t.Fatal(err)
	}
	if errBuild != nil {
		t.Fatalf("build: %v", errBuild)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("defc: warning")) {
		t.Fatalf("expected warning on stderr, got: %q", stderr.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("Raw interpolation")) {
		t.Fatalf("expected raw interpolation hint, got: %q", stderr.String())
	}
}
