package gen

import (
	"bytes"
	"os"
	"testing"

	goformat "go/format"
	ximport "golang.org/x/tools/imports"
)

func runTest(path string, builder Builder) (err error) {
	var bf bytes.Buffer
	if err = builder.Build(&bf); err != nil {
		return err
	}
	code := bf.Bytes()
	code, err = goformat.Source(code)
	if err != nil {
		return err
	}
	if err = os.WriteFile(path, code, 0644); err != nil {
		return err
	}
	code, err = ximport.Process(path, code, nil)
	if err != nil {
		return err
	}
	if err = os.WriteFile(path, code, 0644); err != nil {
		return err
	}
	if err = os.Remove(path); err != nil {
		return err
	}
	return nil
}

func TestMode(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		type TestCase struct {
			Mode    Mode
			String  string
			IsValid bool
		}
		var testcases = []*TestCase{
			{Mode: 0, String: "Mode(0)", IsValid: false},
			{Mode: 1, String: "api", IsValid: true},
			{Mode: 2, String: "sqlx", IsValid: true},
			{Mode: 3, String: "Mode(3)", IsValid: false},
			{Mode: 999, String: "Mode(999)", IsValid: false},
		}
		for _, testcase := range testcases {
			if testcase.Mode.String() != testcase.String {
				t.Errorf("mode: %q != %q", testcase.Mode.String(), testcase.String)
				return
			}
			if testcase.Mode.IsValid() != testcase.IsValid {
				t.Errorf("mode: %v != %v", testcase.Mode.IsValid(), testcase.IsValid)
				return
			}
		}
	})
}
