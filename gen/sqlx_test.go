package gen

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
