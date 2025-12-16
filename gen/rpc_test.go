package gen

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildRpc(t *testing.T) {
	const (
		testPk = "test"
		testGo = testPk + ".go"
	)
	var (
		testDir  = filepath.Join("testdata", "rpc")
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
		return NewCliBuilder(ModeRpc).
			WithPkg(testPk).
			WithPwd(testDirAbs).
			WithFile(testGo, doc).
			WithPos(pos), true
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
	})
	t.Run("success_pointer_reply", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		if err := runTest(genFile, builder); err != nil {
			t.Errorf("build: %s", err)
			return
		}
	})
	t.Run("fail_no_input", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		if err := runTest(genFile, builder); err == nil {
			t.Errorf("build: expects errors, got nil")
			return
		} else if !strings.Contains(err.Error(),
			"should have exactly 1 input parameter") {
			t.Errorf("build: expects NoInput error, got => %s", err)
			return
		}
	})
	t.Run("fail_too_many_inputs", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		if err := runTest(genFile, builder); err == nil {
			t.Errorf("build: expects errors, got nil")
			return
		} else if !strings.Contains(err.Error(),
			"should have exactly 1 input parameter") {
			t.Errorf("build: expects TooManyInputs error, got => %s", err)
			return
		}
	})
	t.Run("fail_no_output", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		if err := runTest(genFile, builder); err == nil {
			t.Errorf("build: expects errors, got nil")
			return
		} else if !strings.Contains(err.Error(),
			"should have exactly 2 output parameters") {
			t.Errorf("build: expects NoOutput error, got => %s", err)
			return
		}
	})
	t.Run("fail_one_output", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		if err := runTest(genFile, builder); err == nil {
			t.Errorf("build: expects errors, got nil")
			return
		} else if !strings.Contains(err.Error(),
			"should have exactly 2 output parameters") {
			t.Errorf("build: expects OneOutput error, got => %s", err)
			return
		}
	})
	t.Run("fail_too_many_outputs", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		if err := runTest(genFile, builder); err == nil {
			t.Errorf("build: expects errors, got nil")
			return
		} else if !strings.Contains(err.Error(),
			"should have exactly 2 output parameters") {
			t.Errorf("build: expects TooManyOutputs error, got => %s", err)
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
		} else if !strings.Contains(err.Error(),
			"should have an error as the second output parameter") {
			t.Errorf("build: expects NoError error, got => %s", err)
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
	t.Run("success_nort", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		builder = builder.WithFeats([]string{FeatureRpcNoRt})
		if err := runTest(genFile, builder); err != nil {
			t.Errorf("build: %s", err)
			return
		}
	})
}
