package gen

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRunCommand(t *testing.T) {
	ctx := context.Background()
	const to = 10 * time.Second
	t.Run("backquoted", func(t *testing.T) {
		commandOutput, err := runCommand(ctx, []string{
			"echo",
			"`echo test`",
		}, to, nil)
		if err != nil {
			t.Errorf("runCommand: %s", err)
			return
		}
		if commandOutput != "test" {
			t.Errorf("runCommand: %q != %q", commandOutput, "test")
			return
		}
		t.Run("error", func(t *testing.T) {
			commandOutput, err := runCommand(ctx, []string{
				"echo",
				fmt.Sprintf("`%s/a_binary_name_that_will_never_appear_in_syspath`", t.TempDir()),
			}, to, nil)
			if err == nil || commandOutput != "" {
				t.Errorf("runCommand: expects errors, got nil")
				return
			} else if !strings.Contains(err.Error(), "no such file or directory") {
				t.Errorf("runCommand: expects NotFoundError, got => %s", err)
				return
			}
		})
	})
	t.Run("paren", func(t *testing.T) {
		commandOutput, err := runCommand(ctx, []string{
			"echo",
			"$(echo test)",
		}, to, nil)
		if err != nil {
			t.Errorf("runCommand: %s", err)
			return
		}
		if commandOutput != "test" {
			t.Errorf("runCommand: %q != %q", commandOutput, "test")
			return
		}
		t.Run("error", func(t *testing.T) {
			commandOutput, err := runCommand(ctx, []string{
				"echo",
				fmt.Sprintf("`%s/a_binary_name_that_will_never_appear_in_syspath`", t.TempDir()),
			}, to, nil)
			if err == nil || commandOutput != "" {
				t.Errorf("runCommand: expects errors, got nil")
				return
			} else if !strings.Contains(err.Error(), "no such file or directory") {
				t.Errorf("runCommand: expects NotFoundError, got => %s", err)
				return
			}
		})
	})
	t.Run("braces", func(t *testing.T) {
		commandOutput, err := runCommand(ctx, []string{
			"echo",
			"${echo test}",
		}, to, nil)
		if err != nil {
			t.Errorf("runCommand: %s", err)
			return
		}
		if commandOutput != "test" {
			t.Errorf("runCommand: %q != %q", commandOutput, "test")
			return
		}
		t.Run("error", func(t *testing.T) {
			commandOutput, err := runCommand(ctx, []string{
				"echo",
				fmt.Sprintf("`%s/a_binary_name_that_will_never_appear_in_syspath`", t.TempDir()),
			}, to, nil)
			if err == nil || commandOutput != "" {
				t.Errorf("runCommand: expects errors, got nil")
				return
			} else if !strings.Contains(err.Error(), "no such file or directory") {
				t.Errorf("runCommand: expects NotFoundError, got => %s", err)
				return
			}
		})
	})
	t.Run("nested", func(t *testing.T) {
		commandOutput, err := runCommand(ctx, []string{
			"echo",
			"${echo $(echo `echo \"test\"`)}",
		}, to, nil)
		if err != nil {
			t.Errorf("runCommand: %s", err)
			return
		}
		if commandOutput != "test" {
			t.Errorf("runCommand: %q != %q", commandOutput, "test")
			return
		}
		t.Run("error", func(t *testing.T) {
			commandOutput, err := runCommand(ctx, []string{
				"echo",
				fmt.Sprintf("`%s/a_binary_name_that_will_never_appear_in_syspath`", t.TempDir()),
			}, to, nil)
			if err == nil || commandOutput != "" {
				t.Errorf("runCommand: expects errors, got nil")
				return
			} else if !strings.Contains(err.Error(), "no such file or directory") {
				t.Errorf("runCommand: expects NotFoundError, got => %s", err)
				return
			}
		})
	})
	t.Run("empty", func(t *testing.T) {
		commandOutput, err := runCommand(ctx, []string{
			"${}",
		}, to, nil)
		if err != nil {
			t.Errorf("runCommand: %s", err)
			return
		}
		if commandOutput != "" {
			t.Errorf("runCommand: expects empty output, got %q", commandOutput)
			return
		}
	})
	t.Run("timeout_kills_slow_process", func(t *testing.T) {
		start := time.Now()
		_, err := runCommand(ctx, []string{"sleep", "5"}, 100*time.Millisecond, nil)
		elapsed := time.Since(start)
		if err == nil {
			t.Fatalf("runCommand: expected timeout error, got nil")
		}
		if !strings.Contains(err.Error(), "timeout") {
			t.Fatalf("runCommand: expected timeout error, got %s", err)
		}
		if elapsed > 4*time.Second {
			t.Fatalf("runCommand: timeout did not kill child promptly (took %s)", elapsed)
		}
	})
	t.Run("env_scrubbed", func(t *testing.T) {
		t.Setenv("DEFC_TEST_SECRET", "sentinel-xyz-shouldnt-leak")
		out, err := runCommand(ctx, []string{"env"}, to, nil)
		if err != nil {
			t.Fatalf("runCommand: %s", err)
		}
		if strings.Contains(out, "DEFC_TEST_SECRET") || strings.Contains(out, "sentinel-xyz-shouldnt-leak") {
			t.Fatalf("runCommand: expected scrubbed env, DEFC_TEST_SECRET leaked into child\n%s", out)
		}
	})
	t.Run("env_allowlist_passes_through", func(t *testing.T) {
		t.Setenv("DEFC_TEST_ALLOWED", "allowed-value")
		out, err := runCommand(ctx, []string{"env"}, to, []string{"DEFC_TEST_ALLOWED"})
		if err != nil {
			t.Fatalf("runCommand: %s", err)
		}
		if !strings.Contains(out, "DEFC_TEST_ALLOWED=allowed-value") {
			t.Fatalf("runCommand: expected DEFC_TEST_ALLOWED to pass through, got:\n%s", out)
		}
	})
	t.Run("relative_argv0_rejected", func(t *testing.T) {
		_, err := runCommand(ctx, []string{"./attacker"}, to, nil)
		if err == nil {
			t.Fatalf("runCommand: expected rejection for relative argv[0]")
		}
		if !strings.Contains(err.Error(), "relative path") &&
			!strings.Contains(err.Error(), "not found in PATH") {
			t.Fatalf("runCommand: expected relative-path rejection, got %s", err)
		}
	})
	t.Run("context_cancel_propagates", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := runCommand(cctx, []string{"sleep", "5"}, 10*time.Second, nil)
		if err == nil {
			t.Fatalf("runCommand: expected error from cancelled context")
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

func TestDetectTargetDecl(t *testing.T) {
	var src = []byte(`
package test

type TestApi1[I any] interface {
	Inner() I
}

type TestApi2[I any] interface {
	// Test POST https://localhost:port/test
	Test(r any) error

	Inner() I
}

type TestSqlx1 interface {
	WithTx(func(TestSqlx) error) error
}

type TestSqlx2 interface {
	// Select Query Scan(obj)
	Select(obj any) error
}
`)
	pkg, mod, pos, err := DetectTargetDecl("test.go", src, "")
	if err != nil {
		t.Errorf("detect: %s", err)
		return
	} else if pkg != "test" || mod != ModeApi || pos != 3 {
		t.Errorf("detect: pkg = %q; mod = %q; pos = %d", pkg, mod, pos)
		return
	}
	pkg, mod, pos, err = DetectTargetDecl("test.go", src, "TestApi2")
	if err != nil {
		t.Errorf("detect: %s", err)
		return
	} else if pkg != "test" || mod != ModeApi || pos != 7 {
		t.Errorf("detect: pkg = %q; mod = %q; pos = %d", pkg, mod, pos)
		return
	}
	pkg, mod, pos, err = DetectTargetDecl("test.go", src, "TestSqlx1")
	if err != nil {
		t.Errorf("detect: %s", err)
		return
	} else if pkg != "test" || mod != ModeSqlx || pos != 14 {
		t.Errorf("detect: pkg = %q; mod = %q; pos = %d", pkg, mod, pos)
		return
	}
	pkg, mod, pos, err = DetectTargetDecl("test.go", src, "TestSqlx2")
	if err != nil {
		t.Errorf("detect: %s", err)
		return
	} else if pkg != "test" || mod != ModeSqlx || pos != 18 {
		t.Errorf("detect: pkg = %q; mod = %q; pos = %d", pkg, mod, pos)
		return
	}
	_, _, _, err = DetectTargetDecl("test.go", src, "Test")
	if err == nil {
		t.Errorf("detect: expects errors, got nil")
		return
	} else if !errors.Is(err, ErrNoTargetDeclFound) {
		t.Errorf("detect: expects ErrNoTargetDeclFound, got => %s", err)
		return
	}
}

func TestMaybeRpcDecl(t *testing.T) {
	t.Run("rpc_interface", func(t *testing.T) {
		const src = `
package test

type RPCService interface {
	Do(*Args) (*Reply, error)
}
`

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "test.go", src, 0)
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}

		var iface *ast.InterfaceType
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if it, ok := typeSpec.Type.(*ast.InterfaceType); ok {
					iface = it
					break
				}
			}
		}
		if iface == nil {
			t.Fatalf("no interface type found")
		}

		if !maybeRpcDecl(iface) {
			t.Fatalf("maybeRpcDecl() = false, want true for rpc-like interface")
		}
	})

	t.Run("non_rpc_interface", func(t *testing.T) {
		const src = `
package test

type NotRPC interface {
	Do(a, b int) error
}
`

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "test.go", src, 0)
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}

		var iface *ast.InterfaceType
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if it, ok := typeSpec.Type.(*ast.InterfaceType); ok {
					iface = it
					break
				}
			}
		}
		if iface == nil {
			t.Fatalf("no interface type found")
		}

		if maybeRpcDecl(iface) {
			t.Fatalf("maybeRpcDecl() = true, want false for non-rpc interface")
		}
	})
}
