package gen

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildSqlx(t *testing.T) {
	const (
		testPk = "test"
		testGo = testPk + ".go"
	)
	var (
		testDir  = filepath.Join("testdata", "sqlx")
		testFile = testGo
		genFile  = testPk + "." + strings.ReplaceAll(t.Name(), "/", "_") + ".go"
	)
	pwd, err := os.Getwd()
	if err != nil {
		t.Errorf("getwd: %s", err)
		return
	}
	defer func() {
		if err = os.Chdir(pwd); err != nil {
			t.Errorf("chdir: %s", err)
			return
		}
	}()
	if err = os.Chdir(testDir); err != nil {
		t.Errorf("chdir: %s", err)
		return
	}
	newBuilder := func(t *testing.T) (*CliBuilder, bool) {
		doc, err := os.ReadFile(testFile)
		if err != nil {
			t.Errorf("build: error reading %s file => %s", testGo, err)
			return nil, false
		}
		var pos int
		lineScanner := bufio.NewScanner(bytes.NewReader(doc))
		for i := 1; lineScanner.Scan(); i++ {
			text := lineScanner.Text()
			if strings.HasPrefix(text, "//go:generate") &&
				strings.HasSuffix(text, t.Name()) {
				pos = i
				break
			}
		}
		if err = lineScanner.Err(); err != nil {
			t.Errorf("build: error scanning %s lines => %s", testGo, err)
			return nil, false
		}
		if pos == 0 {
			t.Errorf("build: unable to get pos in %s", testGo)
			return nil, false
		}
		testDirAbs, err := os.Getwd()
		if err != nil {
			t.Errorf("getwd: %s", err)
			return nil, false
		}
		return NewCliBuilder(ModeSqlx).
			WithFeats([]string{FeatureSqlxNoRt}).
			WithPkg(testPk).
			WithPwd(testDirAbs).
			WithFile(testGo, doc).
			WithPos(pos).
			WithAllowScript(true).
			WithScriptTimeout(10 * time.Second).
			WithTemplate(quote(`{{ define "test_template" }} test_template {{ end }}`)), true
	}
	t.Run("success", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		if err := runTest(genFile, builder); err != nil {
			t.Errorf("build: %s", err)
			return
		}
		builder = builder.WithFeats([]string{FeatureSqlxFuture, FeatureSqlxLog}).
			WithImports([]string{"C", "json encoding/json"}).
			WithFuncs([]string{"marshal: json.Marshal"})
		if err := runTest(genFile, builder); err != nil {
			t.Errorf("build: %s", err)
			return
		}
	})
	t.Run("success_named_tx", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		if err := runTest(genFile, builder); err != nil {
			t.Errorf("build: %s", err)
			return
		}
		builder = builder.WithFeats([]string{FeatureSqlxFuture, FeatureSqlxLog}).
			WithImports([]string{"C", "json encoding/json"}).
			WithFuncs([]string{"marshal: json.Marshal"})
		if err := runTest(genFile, builder); err != nil {
			t.Errorf("build: %s", err)
			return
		}
	})
	t.Run("fail_no_error", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		if err := runTest(genFile, builder); err == nil {
			t.Errorf("build: expects errors, got nil")
			return
		} else if !strings.Contains(err.Error(), "checkErrorType: ") {
			t.Errorf("build: expects checkErrorType error, got => %s", err)
			return
		}
	})
	t.Run("fail_single_scan", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		if err := runTest(genFile, builder); err == nil {
			t.Errorf("build: expects errors, got nil")
			return
		} else if !strings.Contains(err.Error(),
			" expects only error returned value when `scan(expr)` option has been specified") {
			t.Errorf("build: expects SingleScan error, got => %s", err)
			return
		}
	})
	t.Run("fail_2_values", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		if err := runTest(genFile, builder); err == nil {
			t.Errorf("build: expects errors, got nil")
			return
		} else if !strings.Contains(err.Error(),
			" method expects 2 returned value at most") {
			t.Errorf("build: expects 2ValuesAtMost error, got => %s", err)
			return
		}
	})
	t.Run("fail_no_name_type", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		if err := runTest(genFile, builder); err == nil {
			t.Errorf("build: expects errors, got nil")
			return
		} else if !strings.Contains(err.Error(),
			"should contain 'Name' and 'Type' both") {
			t.Errorf("build: expects NoNameType error, got => %s", err)
			return
		}
	})
	t.Run("fail_no_type_decl", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		if err := runTest(genFile, builder); err == nil {
			t.Errorf("build: expects errors, got nil")
			return
		} else if !strings.Contains(err.Error(),
			"no available 'Interface' type declaration (*ast.GenDecl) found, ") {
			t.Errorf("build: expects NoTypeDecl error, got => %s", err)
			return
		}
	})
	t.Run("fail_no_iface_type", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		if err := runTest(genFile, builder); err == nil {
			t.Errorf("build: expects errors, got nil")
			return
		} else if !strings.Contains(err.Error(),
			"no available 'Interface' type declaration (*ast.InterfaceType) found, ") {
			t.Errorf("build: expects NoIfaceType error, got => %s", err)
			return
		}
	})
	t.Run("success_constbind", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		if err := runTest(genFile, builder); err != nil {
			t.Errorf("build: %s", err)
			return
		}
		builder = builder.WithFeats([]string{FeatureSqlxFuture, FeatureSqlxLog}).
			WithImports([]string{"C", "json encoding/json"}).
			WithFuncs([]string{"marshal: json.Marshal"})
		if err := runTest(genFile, builder); err != nil {
			t.Errorf("build: %s", err)
			return
		}
	})
	t.Run("fail_constbind_bind_conflict", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		if err := runTest(genFile, builder); err == nil {
			t.Errorf("build: expects errors, got nil")
			return
		} else if !strings.Contains(err.Error(),
			"CONSTBIND and BIND options are mutually exclusive") {
			t.Errorf("build: expects ConstBindBindConflict error, got => %s", err)
			return
		}
	})
}

// TestReadHeader_IncludeBoundary covers the #INCLUDE hardening: paths that
// escape the schema directory (via absolute paths, ".." segments, or
// symlinks) are rejected; per-file and aggregate size caps are enforced;
// well-formed inputs under the schema directory succeed with stable content.
func TestReadHeader_IncludeBoundary(t *testing.T) {
schemaDir := t.TempDir()
if err := os.WriteFile(filepath.Join(schemaDir, "ok.sql"), []byte("SELECT 1;\n"), 0644); err != nil {
t.Fatal(err)
}

base := func() *readHeaderConfig {
return &readHeaderConfig{
schemaDir:     schemaDir,
directiveFile: filepath.Join(schemaDir, "schema.go"),
lineOffset:    0,
}
}

t.Run("absolute_outside_root", func(t *testing.T) {
_, err := readHeader(`#INCLUDE "/etc/passwd"`, base())
if err == nil || !strings.Contains(err.Error(), "absolute path") {
t.Fatalf("expected absolute-path rejection, got %v", err)
}
})

t.Run("relative_escapes_schemadir", func(t *testing.T) {
_, err := readHeader(`#INCLUDE "../../outside.sql"`, base())
if err == nil || !strings.Contains(err.Error(), "outside the schema directory") {
t.Fatalf("expected escape rejection, got %v", err)
}
})

t.Run("symlink_rejected", func(t *testing.T) {
real := filepath.Join(schemaDir, "real.sql")
if err := os.WriteFile(real, []byte("SELECT 2;\n"), 0644); err != nil {
t.Fatal(err)
}
link := filepath.Join(schemaDir, "link.sql")
if err := os.Symlink(real, link); err != nil {
t.Skipf("symlink unsupported: %s", err)
}
_, err := readHeader(`#INCLUDE "link.sql"`, base())
if err == nil || !strings.Contains(err.Error(), "symlink not allowed") {
t.Fatalf("expected symlink rejection, got %v", err)
}
})

t.Run("per_file_cap", func(t *testing.T) {
big := filepath.Join(schemaDir, "big.sql")
if err := os.WriteFile(big, bytes.Repeat([]byte("a"), 1024*1024+1), 0644); err != nil {
t.Fatal(err)
}
defer os.Remove(big)
_, err := readHeader(`#INCLUDE "big.sql"`, base())
if err == nil || !strings.Contains(err.Error(), "1 MiB per-file limit") {
t.Fatalf("expected per-file cap rejection, got %v", err)
}
})

t.Run("aggregate_cap", func(t *testing.T) {
aggDir := t.TempDir()
cfg := &readHeaderConfig{
schemaDir:     aggDir,
directiveFile: filepath.Join(aggDir, "schema.go"),
}
// Five ~900 KiB files; 4 MiB cap → 5th must trip.
chunk := bytes.Repeat([]byte("b"), 900*1024)
for i := 0; i < 5; i++ {
name := filepath.Join(aggDir, fmt.Sprintf("f%02d.sql", i))
if err := os.WriteFile(name, chunk, 0644); err != nil {
t.Fatal(err)
}
}
_, err := readHeader(`#INCLUDE "*.sql"`, cfg)
if err == nil || !strings.Contains(err.Error(), "4 MiB aggregate limit") {
t.Fatalf("expected aggregate cap rejection, got %v", err)
}
})

t.Run("happy_path", func(t *testing.T) {
out, err := readHeader(`#INCLUDE "ok.sql"`, base())
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if !strings.Contains(out, "SELECT 1;") {
t.Fatalf("expected SELECT 1 in output, got %q", out)
}
})

t.Run("no_match", func(t *testing.T) {
_, err := readHeader(`#INCLUDE "nonexistent.sql"`, base())
if err == nil || !strings.Contains(err.Error(), "matched no files") {
t.Fatalf("expected no-match rejection, got %v", err)
}
})

t.Run("absolute_under_include_root", func(t *testing.T) {
rootDir := t.TempDir()
data := filepath.Join(rootDir, "ext.sql")
if err := os.WriteFile(data, []byte("SELECT 3;"), 0644); err != nil {
t.Fatal(err)
}
cfg := base()
cfg.includeRoots = []string{rootDir}
out, err := readHeader(fmt.Sprintf(`#INCLUDE %q`, data), cfg)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if !strings.Contains(out, "SELECT 3;") {
t.Fatalf("expected content, got %q", out)
}
})
}

// TestReadHeader_ScriptGating covers the #SCRIPT gating/timeout/env hardening.
func TestReadHeader_ScriptGating(t *testing.T) {
schemaDir := t.TempDir()
baseCfg := func() *readHeaderConfig {
return &readHeaderConfig{
schemaDir:     schemaDir,
directiveFile: filepath.Join(schemaDir, "schema.go"),
}
}

t.Run("disabled_by_default", func(t *testing.T) {
_, err := readHeader(`#SCRIPT echo hi`, baseCfg())
if err == nil || !strings.Contains(err.Error(), "disabled by default") {
t.Fatalf("expected gating error, got %v", err)
}
if !strings.Contains(err.Error(), "--allow-script") {
t.Fatalf("expected mention of --allow-script, got %v", err)
}
})

t.Run("allowed_with_flag", func(t *testing.T) {
cfg := baseCfg()
cfg.allowScript = true
cfg.scriptTimeout = 10 * time.Second
out, err := readHeader(`#SCRIPT echo hi`, cfg)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if !strings.Contains(out, "hi") {
t.Fatalf("expected 'hi' in output, got %q", out)
}
})

t.Run("timeout_trips", func(t *testing.T) {
cfg := baseCfg()
cfg.allowScript = true
cfg.scriptTimeout = 100 * time.Millisecond
_, err := readHeader(`#SCRIPT sleep 5`, cfg)
if err == nil || !strings.Contains(err.Error(), "timeout") {
t.Fatalf("expected timeout error, got %v", err)
}
})

t.Run("env_scrubbed", func(t *testing.T) {
t.Setenv("DEFC_TEST_SECRET_HDR", "leaked")
cfg := baseCfg()
cfg.allowScript = true
cfg.scriptTimeout = 10 * time.Second
out, err := readHeader(`#SCRIPT env`, cfg)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if strings.Contains(out, "DEFC_TEST_SECRET_HDR") {
t.Fatalf("expected DEFC_TEST_SECRET_HDR to be absent from scrubbed child env, got:\n%s", out)
}
})
}
