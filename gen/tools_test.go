package gen

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

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
				fmt.Sprintf("`%s/a_binary_name_that_will_never_appear_in_syspath`", t.TempDir()),
			})
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
				fmt.Sprintf("`%s/a_binary_name_that_will_never_appear_in_syspath`", t.TempDir()),
			})
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
				fmt.Sprintf("`%s/a_binary_name_that_will_never_appear_in_syspath`", t.TempDir()),
			})
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
				fmt.Sprintf("`%s/a_binary_name_that_will_never_appear_in_syspath`", t.TempDir()),
			})
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
