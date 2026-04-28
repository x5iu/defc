package gen

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildApi(t *testing.T) {
	const (
		testPk = "test"
		testGo = testPk + ".go"
	)
	var (
		testDir  = filepath.Join("testdata", "api")
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
		return NewCliBuilder(ModeApi).
			WithFeats([]string{FeatureApiNoRt, FeatureApiFuture}).
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
		builder = builder.WithFeats([]string{FeatureApiLogx}).
			WithImports([]string{"url net/url"}).
			WithFuncs([]string{"escape=url.QueryEscape"})
		if err := runTest(genFile, builder); err != nil {
			t.Errorf("build: %s", err)
			return
		}
		t.Run("no_generics", func(t *testing.T) {
			builder, ok := newBuilder(t)
			if !ok {
				return
			}
			if err := runTest(genFile, builder); err != nil {
				t.Errorf("build: %s", err)
				return
			}
		})
	})
	t.Run("fail_no_response", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		if err := runTest(genFile, builder); err == nil {
			t.Errorf("build: expects errors, got nil")
			return
		} else if !strings.Contains(err.Error(), "checkResponse: ") {
			t.Errorf("build: expects checkResponse error, got => %s", err)
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
	t.Run("fail_invalid_IR", func(t *testing.T) {
		t.Run("I", func(t *testing.T) {
			builder, ok := newBuilder(t)
			if !ok {
				return
			}
			if err := runTest(genFile, builder); err == nil {
				t.Errorf("build: expects errors, got nil")
				return
			} else if !strings.Contains(err.Error(),
				"method can only have no income params and 1 returned value") {
				t.Errorf("build: expects InvalidI error, got => %s", err)
				return
			}
		})
		t.Run("R", func(t *testing.T) {
			builder, ok := newBuilder(t)
			if !ok {
				return
			}
			if err := runTest(genFile, builder); err == nil {
				t.Errorf("build: expects errors, got nil")
				return
			} else if !strings.Contains(err.Error(),
				"method can only have no income params and 1 returned value") {
				t.Errorf("build: expects InvalidR error, got => %s", err)
				return
			}
			t.Run("type", func(t *testing.T) {
				builder, ok := newBuilder(t)
				if !ok {
					return
				}
				if err := runTest(genFile, builder); err == nil {
					t.Errorf("build: expects errors, got nil")
					return
				} else if !strings.Contains(err.Error(), "checkResponseType: ") {
					t.Errorf("build: expects checkResponseType error, got => %s", err)
					return
				}
			})
		})
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
	t.Run("success_get_body", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		builder = builder.WithFeats([]string{FeatureApiGetBody, FeatureApiNoRt, FeatureApiFuture})
		if err := runTest(genFile, builder); err != nil {
			t.Errorf("build: %s", err)
			return
		}
	})
	t.Run("fail_requires_options", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		builder = builder.WithFeats([]string{FeatureApiLog, FeatureApiNoRt, FeatureApiFuture})
		if err := runTest(genFile, builder); err == nil {
			t.Errorf("build: expects errors, got nil")
			return
		} else if !strings.Contains(err.Error(),
			"api/cache, api/log, api/logx and api/client features require an `Options` method") {
			t.Errorf("build: expects Options method requirement error, got => %s", err)
			return
		}
	})
	t.Run("success_with_options", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		builder = builder.WithFeats([]string{FeatureApiLog, FeatureApiNoRt, FeatureApiFuture})
		if err := runTest(genFile, builder); err != nil {
			t.Errorf("build: %s", err)
			return
		}
	})
	t.Run("fail_header_empty_block", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		if err := runTest(genFile, builder); err == nil {
			t.Errorf("build: expects errors, got nil")
			return
		} else if !strings.Contains(err.Error(), "API header block is empty") || !strings.Contains(err.Error(), `"Run"`) {
			t.Errorf("build: expects empty API header error for Run, got => %s", err)
			return
		}
	})
	t.Run("fail_body_template_get", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		if err := runTest(genFile, builder); err == nil {
			t.Errorf("build: expects errors, got nil")
			return
		} else if !strings.Contains(err.Error(), "only allowed for POST, PUT, or PATCH") || !strings.Contains(err.Error(), `"Run"`) {
			t.Errorf("build: expects body-on-GET error, got => %s", err)
			return
		}
	})
	t.Run("fail_body_template_head", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		if err := runTest(genFile, builder); err == nil {
			t.Errorf("build: expects errors, got nil")
			return
		} else if !strings.Contains(err.Error(), "only allowed for POST, PUT, or PATCH") || !strings.Contains(err.Error(), `"Run"`) {
			t.Errorf("build: expects body-on-HEAD error, got => %s", err)
			return
		}
	})
	t.Run("success_body_template_post", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		if err := runTest(genFile, builder); err != nil {
			t.Errorf("build: %s", err)
			return
		}
	})
	t.Run("success_body_only", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		if err := runTest(genFile, builder); err != nil {
			t.Errorf("build: %s", err)
			return
		}
	})
	t.Run("success_gzip_no_headers", func(t *testing.T) {
		builder, ok := newBuilder(t)
		if !ok {
			return
		}
		builder = builder.WithFeats([]string{FeatureApiFuture, FeatureApiGzip})
		if err := runTest(genFile, builder); err != nil {
			t.Errorf("build: %s", err)
			return
		}
	})
}

func TestGenApiOutputHasNoMIMEParsing(t *testing.T) {
	const (
		testPk = "test"
		testGo = testPk + ".go"
	)
	testDir := filepath.Join("testdata", "api")
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(pwd) }()
	if err := os.Chdir(testDir); err != nil {
		t.Fatal(err)
	}
	doc, err := os.ReadFile(testGo)
	if err != nil {
		t.Fatal(err)
	}
	var pos int
	lineScanner := bufio.NewScanner(bytes.NewReader(doc))
	for i := 1; lineScanner.Scan(); i++ {
		text := lineScanner.Text()
		if strings.HasPrefix(text, "//go:generate") &&
			strings.HasSuffix(text, "TestBuildApi/success") {
			pos = i
			break
		}
	}
	if err := lineScanner.Err(); err != nil {
		t.Fatal(err)
	}
	if pos == 0 {
		t.Fatal("pos not found")
	}
	testDirAbs, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	builder := NewCliBuilder(ModeApi).
		WithFeats([]string{FeatureApiNoRt, FeatureApiFuture}).
		WithPkg(testPk).
		WithPwd(testDirAbs).
		WithFile(testGo, doc).
		WithPos(pos)
	var buf bytes.Buffer
	if err := builder.Build(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, sub := range []string{"bufio", "net/textproto", "ReadMIMEHeader"} {
		if strings.Contains(out, sub) {
			t.Errorf("generated code must not contain %q", sub)
		}
	}
}

func TestApiGzipNoHeadersEmitsStringsImport(t *testing.T) {
	const (
		testPk = "test"
		testGo = testPk + ".go"
	)
	testDir := filepath.Join("testdata", "api")
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(pwd) }()
	if err := os.Chdir(testDir); err != nil {
		t.Fatal(err)
	}
	doc, err := os.ReadFile(testGo)
	if err != nil {
		t.Fatal(err)
	}
	var pos int
	lineScanner := bufio.NewScanner(bytes.NewReader(doc))
	for i := 1; lineScanner.Scan(); i++ {
		text := lineScanner.Text()
		if strings.HasPrefix(text, "//go:generate") &&
			strings.HasSuffix(text, "TestBuildApi/success_gzip_no_headers") {
			pos = i
			break
		}
	}
	if err = lineScanner.Err(); err != nil {
		t.Fatal(err)
	}
	if pos == 0 {
		t.Fatal("pos not found")
	}
	testDirAbs, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err = NewCliBuilder(ModeApi).
		WithFeats([]string{FeatureApiFuture, FeatureApiGzip}).
		WithPkg(testPk).
		WithPwd(testDirAbs).
		WithFile(testGo, doc).
		WithPos(pos).
		Build(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"strings"`) {
		t.Fatalf("api/gzip without custom headers: generated code must import strings, got snippet:\n%s", out)
	}
}

func TestApiGeneratedCodeHeaderValueValidation(t *testing.T) {
	const (
		testPk = "test"
		testGo = testPk + ".go"
	)
	testDir := filepath.Join("testdata", "api")
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(pwd) }()
	if err := os.Chdir(testDir); err != nil {
		t.Fatal(err)
	}
	doc, err := os.ReadFile(testGo)
	if err != nil {
		t.Fatal(err)
	}
	var pos int
	lineScanner := bufio.NewScanner(bytes.NewReader(doc))
	for i := 1; lineScanner.Scan(); i++ {
		text := lineScanner.Text()
		if strings.HasPrefix(text, "//go:generate") &&
			strings.HasSuffix(text, "TestBuildApi/success") {
			pos = i
			break
		}
	}
	if err = lineScanner.Err(); err != nil {
		t.Fatal(err)
	}
	if pos == 0 {
		t.Fatal("pos not found")
	}
	testDirAbs, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err = NewCliBuilder(ModeApi).
		WithFeats([]string{FeatureApiNoRt, FeatureApiFuture}).
		WithPkg(testPk).
		WithPwd(testDirAbs).
		WithFile(testGo, doc).
		WithPos(pos).
		Build(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, s := range []string{
		"contains disallowed control character 0x%02x",
		"__c < 0x20 && __c != '\\t'",
		"__c == 0x7f",
	} {
		if !strings.Contains(out, s) {
			t.Fatalf("expected generated code to contain %q", s)
		}
	}
}
