package gen

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildApiAlias(t *testing.T) {
	const (
		testPk = "alias"
		testGo = testPk + ".go"
	)
	var (
		testDir = filepath.Join("testdata", "api", "alias")
		genFile = testPk + "." + strings.ReplaceAll(t.Name(), "/", "_") + ".go"
	)
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %s", err)
	}
	defer func() {
		if err = os.Chdir(pwd); err != nil {
			t.Errorf("chdir: %s", err)
		}
	}()
	if err = os.Chdir(testDir); err != nil {
		t.Fatalf("chdir: %s", err)
	}
	newBuilder := func(t *testing.T, feats []string) *CliBuilder {
		doc, err := os.ReadFile(testGo)
		if err != nil {
			t.Fatalf("read %s: %s", testGo, err)
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
			t.Fatalf("scan %s: %s", testGo, err)
		}
		if pos == 0 {
			t.Fatalf("unable to locate //go:generate for %s in %s", t.Name(), testGo)
		}
		testDirAbs, err := os.Getwd()
		if err != nil {
			t.Fatalf("getwd: %s", err)
		}
		return NewCliBuilder(ModeApi).
			WithFeats(feats).
			WithPkg(testPk).
			WithPwd(testDirAbs).
			WithFile(testGo, doc).
			WithPos(pos)
	}

	t.Run("alias", func(t *testing.T) {
		t.Run("plain", func(t *testing.T) {
			builder := newBuilder(t, []string{FeatureApiError})
			out, err := runTestRetain(genFile, builder)
			t.Cleanup(func() { _ = os.Remove(genFile) })
			if err != nil {
				t.Fatalf("build: %s", err)
			}
			src := string(out)
			mustContain := []string{
				"bodyCopyRun := __rt.DetachBytes(responseBodyRun)",
				"__rt.NewResponseError(\"Run\", httpResponseRun.StatusCode, bodyCopyRun)",
				"responseRun.FromBytes(\"Run\", bodyCopyRun)",
			}
			for _, s := range mustContain {
				if !strings.Contains(src, s) {
					t.Errorf("plain: generated output missing %q", s)
				}
			}
			if t.Failed() {
				t.Logf("\n%s\n", src)
			}
		})

		t.Run("nort", func(t *testing.T) {
			builder := newBuilder(t, []string{FeatureApiError, FeatureApiNoRt})
			out, err := runTestRetain(genFile, builder)
			t.Cleanup(func() { _ = os.Remove(genFile) })
			if err != nil {
				t.Fatalf("build: %s", err)
			}
			src := string(out)
			mustContain := []string{
				"bodyCopyRun := __AliasNortDetachBytes(responseBodyRun)",
				"__AliasNortNewResponseError(\"Run\", httpResponseRun.StatusCode, bodyCopyRun)",
				"responseRun.FromBytes(\"Run\", bodyCopyRun)",
				"func __AliasNortDetachBytes(",
				"append([]byte(nil), body...)",
			}
			for _, s := range mustContain {
				if !strings.Contains(src, s) {
					t.Errorf("nort: generated output missing %q", s)
				}
			}
			if t.Failed() {
				t.Logf("\n%s\n", src)
			}
		})
	})
}
