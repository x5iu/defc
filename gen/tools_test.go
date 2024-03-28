package gen

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/importer"
	"go/token"
	"go/types"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unsafe"
)

func randStr() string {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		panic(fmt.Errorf("randStr: %w", err))
	}
	return hex.EncodeToString(b)
}

func TestRunCommand(t *testing.T) {
	t.Run("backquoted", func(t *testing.T) {
		commandOutput, err := runCommand([]string{
			"echo",
			"`echo test`",
		})
		if err != nil {
			t.Errorf("runCommand: %s", err)
			return
		}
		if commandOutput != "test" {
			t.Errorf("runCommand: %q != %q", commandOutput, "test")
			return
		}
		t.Run("error", func(t *testing.T) {
			commandOutput, err := runCommand([]string{
				"echo",
				"`a_binary_name_that_will_never_appear_in_syspath_" + randStr() + "`",
			})
			if err == nil || commandOutput != "" {
				t.Errorf("runCommand: expects errors, got nil")
				return
			} else if !strings.HasPrefix(err.Error(), "exec: ") ||
				!strings.Contains(err.Error(), "executable file not found in $PATH") {
				t.Errorf("runCommand: expects NotFoundError, got => %s", err)
				return
			}
		})
	})
	t.Run("paren", func(t *testing.T) {
		commandOutput, err := runCommand([]string{
			"echo",
			"$(echo test)",
		})
		if err != nil {
			t.Errorf("runCommand: %s", err)
			return
		}
		if commandOutput != "test" {
			t.Errorf("runCommand: %q != %q", commandOutput, "test")
			return
		}
		t.Run("error", func(t *testing.T) {
			commandOutput, err := runCommand([]string{
				"echo",
				"$(a_binary_name_that_will_never_appear_in_syspath_" + randStr() + ")",
			})
			if err == nil || commandOutput != "" {
				t.Errorf("runCommand: expects errors, got nil")
				return
			} else if !strings.HasPrefix(err.Error(), "exec: ") ||
				!strings.Contains(err.Error(), "executable file not found in $PATH") {
				t.Errorf("runCommand: expects NotFoundError, got => %s", err)
				return
			}
		})
	})
	t.Run("braces", func(t *testing.T) {
		commandOutput, err := runCommand([]string{
			"echo",
			"${echo test}",
		})
		if err != nil {
			t.Errorf("runCommand: %s", err)
			return
		}
		if commandOutput != "test" {
			t.Errorf("runCommand: %q != %q", commandOutput, "test")
			return
		}
		t.Run("error", func(t *testing.T) {
			commandOutput, err := runCommand([]string{
				"echo",
				"${a_binary_name_that_will_never_appear_in_syspath_" + randStr() + "}",
			})
			if err == nil || commandOutput != "" {
				t.Errorf("runCommand: expects errors, got nil")
				return
			} else if !strings.HasPrefix(err.Error(), "exec: ") ||
				!strings.Contains(err.Error(), "executable file not found in $PATH") {
				t.Errorf("runCommand: expects NotFoundError, got => %s", err)
				return
			}
		})
	})
	t.Run("nested", func(t *testing.T) {
		commandOutput, err := runCommand([]string{
			"echo",
			"${echo $(echo `echo \"test\"`)}",
		})
		if err != nil {
			t.Errorf("runCommand: %s", err)
			return
		}
		if commandOutput != "test" {
			t.Errorf("runCommand: %q != %q", commandOutput, "test")
			return
		}
		t.Run("error", func(t *testing.T) {
			commandOutput, err := runCommand([]string{
				"echo",
				"${$(`a_binary_name_that_will_never_appear_in_syspath_" + randStr() + "`)}",
			})
			if err == nil || commandOutput != "" {
				t.Errorf("runCommand: expects errors, got nil")
				return
			} else if !strings.HasPrefix(err.Error(), "exec: ") ||
				!strings.Contains(err.Error(), "executable file not found in $PATH") {
				t.Errorf("runCommand: expects NotFoundError, got => %s", err)
				return
			}
		})
	})
	t.Run("empty", func(t *testing.T) {
		commandOutput, err := runCommand([]string{
			"${}",
		})
		if err != nil {
			t.Errorf("runCommand: %s", err)
			return
		}
		if commandOutput != "" {
			t.Errorf("runCommand: expects empty output, got %q", commandOutput)
			return
		}
	})
}

func TestSplitArgs(t *testing.T) {
	type TestCase struct {
		Name   string
		Data   string
		Expect []string
	}
	var testcases = []*TestCase{
		{
			Name:   "single_quote",
			Data:   "'test' \\'",
			Expect: []string{"'test'", "\\'"},
		},
		{
			Name:   "back_quote",
			Data:   "`test` \\`",
			Expect: []string{"`test`", "\\`"},
		},
	}
	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			if args := splitArgs(testcase.Data); !reflect.DeepEqual(args, testcase.Expect) {
				t.Errorf("split: %v != %v", args, testcase.Expect)
				return
			}
		})
	}
}

func TestGetIdent(t *testing.T) {
	type TestCase struct {
		Name   string
		Data   string
		Expect string
	}
	var testcases = []*TestCase{
		{
			Name:   "with_space",
			Data:   "test",
			Expect: "test",
		},
		{
			Name:   "without_space",
			Data:   "test 0327",
			Expect: "test",
		},
	}
	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			if ident := getIdent(testcase.Data); ident != testcase.Expect {
				t.Errorf("ident: %q != %q", ident, testcase.Expect)
				return
			}
		})
	}
}

func TestParseExpr(t *testing.T) {
	if expr, err := parseExpr("json.RawMessage"); err != nil {
		t.Errorf("expr: %s", err)
		return
	} else if _, ok := expr.(*ast.SelectorExpr); !ok {
		t.Errorf("expr: expects *ast.SelectorExpr, got %T", expr)
		return
	}
}

func TestImporter(t *testing.T) {
	testImporter := &Importer{
		imported:      map[string]*types.Package{},
		tokenFileSet:  token.NewFileSet(),
		defaultImport: importer.Default(),
	}
	t.Run("unsafe", func(t *testing.T) {
		if pkg, err := testImporter.Import("unsafe"); err != nil {
			t.Errorf("import: %s", err)
			return
		} else if pkg == nil {
			t.Errorf("import: expects non-nil *types.Package, got nil")
			return
		}
	})
	t.Run("C", func(t *testing.T) {
		if pkg, err := testImporter.Import("C"); err == nil || pkg != nil {
			t.Errorf("import: expects errors, got nil")
			return
		} else if err.Error() != "unreachable: import \"C\"" {
			t.Errorf("import: expects unreachable error, got => %s", err)
			return
		}
	})
	t.Run("twice", func(t *testing.T) {
		pkg1, err := testImporter.Import("github.com/x5iu/defc/runtime")
		if err != nil {
			t.Errorf("import: %s", err)
			return
		} else if pkg1 == nil {
			t.Errorf("import: expects non-nil *types.Package, got nil")
			return
		}
		pkg2, err := testImporter.Import("github.com/x5iu/defc/runtime")
		if err != nil {
			t.Errorf("import: %s", err)
			return
		} else if pkg2 == nil {
			t.Errorf("import: expects non-nil *types.Package, got nil")
			return
		}
		if uintptr(unsafe.Pointer(pkg1)) != uintptr(unsafe.Pointer(pkg2)) {
			t.Errorf("import: cache not effective")
			return
		}
	})
	t.Run("cycle_import", func(t *testing.T) {
		for _, dir := range [][]string{
			{"testdata", "cycle", "a"},
			{"testdata", "cycle", "b"},
		} {
			cycleImporter := &Importer{
				imported:      map[string]*types.Package{},
				tokenFileSet:  token.NewFileSet(),
				defaultImport: importer.Default(),
			}
			_, err := cycleImporter.ImportFrom(
				"github.com/x5iu/defc/gen/"+strings.Join(dir, "/"),
				filepath.Join(dir...),
				0,
			)
			if err == nil {
				t.Errorf("import: expects errors, got nil")
				return
			} else if !strings.Contains(err.Error(), "cycle importing ") {
				t.Errorf("import: expects CycleImporting error, got => %s", err)
				return
			}
		}
	})
}
