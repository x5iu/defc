package gen

import (
	"strings"
	"testing"
	"text/template/parse"
)

func stubFuncs(extra map[string]any) map[string]any {
	fm := map[string]any{
		"bind":     func(...any) any { return "" },
		"bindvars": func(...any) any { return "" },
	}
	for k, v := range extra {
		fm[k] = v
	}
	return fm
}

func parseMust(t *testing.T, name, text string, extra map[string]any) *parse.Tree {
	t.Helper()
	trees, err := parse.Parse(name, text, "{{", "}}", stubFuncs(extra))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return trees[name]
}

func TestScanUnsafeRawInterpolation_field(t *testing.T) {
	tree := parseMust(t, "m", "SELECT * FROM t WHERE x = {{ .name }}", nil)
	findings := scanUnsafeRawInterpolation(tree, nil, map[string]struct{}{"name": {}}, map[string]struct{}{"bind": {}, "bindvars": {}})
	if len(findings) != 1 || findings[0].MethodArg != "name" {
		t.Fatalf("got %#v", findings)
	}
}

func TestScanUnsafeRawInterpolation_bindSafe(t *testing.T) {
	tree := parseMust(t, "m", "SELECT * FROM t WHERE x = {{ bind $.name }}", nil)
	findings := scanUnsafeRawInterpolation(tree, nil, map[string]struct{}{"name": {}}, map[string]struct{}{"bind": {}, "bindvars": {}})
	if len(findings) != 0 {
		t.Fatalf("got %#v", findings)
	}
}

func TestScanUnsafeRawInterpolation_ifGuard(t *testing.T) {
	tree := parseMust(t, "m", "{{if .cond}}SELECT 1{{end}}", nil)
	findings := scanUnsafeRawInterpolation(tree, nil, map[string]struct{}{"cond": {}}, map[string]struct{}{"bind": {}, "bindvars": {}})
	if len(findings) != 0 {
		t.Fatalf("got %#v", findings)
	}
}

func TestScanUnsafeRawInterpolation_rangeBind(t *testing.T) {
	tree := parseMust(t, "m", "{{range .xs}}{{ bind . }}{{end}}", nil)
	findings := scanUnsafeRawInterpolation(tree, nil, map[string]struct{}{"xs": {}}, map[string]struct{}{"bind": {}, "bindvars": {}})
	if len(findings) != 0 {
		t.Fatalf("got %#v", findings)
	}
}

func TestScanUnsafeRawInterpolation_rangeDot(t *testing.T) {
	tree := parseMust(t, "m", "{{range .xs}}{{ . }}{{end}}", nil)
	findings := scanUnsafeRawInterpolation(tree, nil, map[string]struct{}{"xs": {}}, map[string]struct{}{"bind": {}, "bindvars": {}})
	if len(findings) != 1 || findings[0].MethodArg != "xs" {
		t.Fatalf("got %#v", findings)
	}
}

func TestScanUnsafeRawInterpolation_dollarField(t *testing.T) {
	tree := parseMust(t, "m", "ORDER BY {{ $.table }}", nil)
	findings := scanUnsafeRawInterpolation(tree, nil, map[string]struct{}{"table": {}}, map[string]struct{}{"bind": {}, "bindvars": {}})
	if len(findings) != 1 || findings[0].MethodArg != "table" {
		t.Fatalf("got %#v", findings)
	}
}

func TestScanUnsafeRawInterpolation_userFuncNotSafe(t *testing.T) {
	tree := parseMust(t, "m", "SELECT {{ myFunc .x }}", map[string]any{"myFunc": func(...any) any { return "" }})
	findings := scanUnsafeRawInterpolation(tree, nil, map[string]struct{}{"x": {}}, map[string]struct{}{"bind": {}, "bindvars": {}})
	if len(findings) != 1 || findings[0].MethodArg != "x" {
		t.Fatalf("got %#v", findings)
	}
}

func TestScanUnsafeRawInterpolation_sharedTemplateNoMethodArg(t *testing.T) {
	shared := `{{ define "audit" }}/* ok */{{ end }}`
	sharedTrees, err := parse.Parse("shared", shared, "{{", "}}", stubFuncs(nil))
	if err != nil {
		t.Fatal(err)
	}
	tree := parseMust(t, "m", "{{ template \"audit\" . }}", nil)
	findings := scanUnsafeRawInterpolation(tree, sharedTrees, map[string]struct{}{"ctx": {}}, map[string]struct{}{"bind": {}, "bindvars": {}})
	if len(findings) != 0 {
		t.Fatalf("got %#v", findings)
	}
}

func TestParseInvalidTemplateNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic: %v", r)
		}
	}()
	_, err := parse.Parse("bad", "{{ .x }", "{{", "}}", stubFuncs(nil))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "error") && !strings.Contains(err.Error(), "unclosed") {
		t.Logf("err: %v", err)
	}
}

func TestScanUnsafeRawInterpolation_withField(t *testing.T) {
	tree := parseMust(t, "m", "{{ with .user }}{{ .Name }}{{ end }}", nil)
	findings := scanUnsafeRawInterpolation(tree, nil, map[string]struct{}{"user": {}}, map[string]struct{}{"bind": {}, "bindvars": {}})
	if len(findings) != 1 || findings[0].MethodArg != "user" {
		t.Fatalf("got %#v", findings)
	}
}

func TestScanUnsafeRawInterpolation_rangeUserField(t *testing.T) {
	tree := parseMust(t, "m", "{{ range .users }}{{ .Name }}{{ end }}", nil)
	findings := scanUnsafeRawInterpolation(tree, nil, map[string]struct{}{"users": {}}, map[string]struct{}{"bind": {}, "bindvars": {}})
	if len(findings) != 1 || findings[0].MethodArg != "users" {
		t.Fatalf("got %#v", findings)
	}
}

func TestScanUnsafeRawInterpolation_rangeBindLocal(t *testing.T) {
	tree := parseMust(t, "m", "{{ range $e := .list }}{{ $e }}{{ end }}", nil)
	findings := scanUnsafeRawInterpolation(tree, nil, map[string]struct{}{"list": {}}, map[string]struct{}{"bind": {}, "bindvars": {}})
	if len(findings) != 1 || findings[0].MethodArg != "list" {
		t.Fatalf("got %#v", findings)
	}
}

func TestScanUnsafeRawInterpolation_withBindOther(t *testing.T) {
	tree := parseMust(t, "m", "{{ with .user }}{{ bind $.other }}{{ end }}", nil)
	findings := scanUnsafeRawInterpolation(tree, nil, map[string]struct{}{"user": {}, "other": {}}, map[string]struct{}{"bind": {}, "bindvars": {}})
	if len(findings) != 0 {
		t.Fatalf("got %#v", findings)
	}
}

func TestScanUnsafeRawInterpolation_mixedBindAndRaw(t *testing.T) {
	tree := parseMust(t, "m", "SELECT * FROM t WHERE id = {{ bind $.id }} AND r = {{ .role }}", nil)
	findings := scanUnsafeRawInterpolation(tree, nil, map[string]struct{}{"id": {}, "role": {}}, map[string]struct{}{"bind": {}, "bindvars": {}})
	if len(findings) != 1 || findings[0].MethodArg != "role" {
		t.Fatalf("got %#v", findings)
	}
}

func TestTemplateForestReferencesBind(t *testing.T) {
	ctx := &sqlxContext{}
	trees, err := parse.Parse("root", `{{ define "a" }}{{ bind .x }}{{ end }}`, "{{", "}}", sqlxParseStubFuncs(ctx))
	if err != nil {
		t.Fatal(err)
	}
	if !templateForestReferencesBind(trees) {
		t.Fatal("expected bind reference")
	}
	trees2, err := parse.Parse("root2", `{{ define "a" }}plain{{ end }}`, "{{", "}}", sqlxParseStubFuncs(ctx))
	if err != nil {
		t.Fatal(err)
	}
	if templateForestReferencesBind(trees2) {
		t.Fatal("unexpected bind")
	}
}

