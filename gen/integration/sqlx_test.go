package integration

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/x5iu/defc/gen"
	goimport "golang.org/x/tools/imports"
)

func TestSqlx(t *testing.T) {
	var (
		testPk      = "main"
		testDir     = "sqlx"
		testFile    = "main.go"
		testGenFile = "main.gen.go"
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
	doc, err := os.ReadFile(testFile)
	if err != nil {
		t.Errorf("read %s: %s", testFile, err)
		return
	}
	var (
		pos      int
		features []string
	)
	lineScanner := bufio.NewScanner(bytes.NewReader(doc))
	for i := 1; lineScanner.Scan(); i++ {
		text := lineScanner.Text()
		if strings.HasPrefix(text, "//go:generate") {
			pos = i
			featReg := regexp.MustCompile(`--features(?:\s|=)([\w,/]+)`)
			featureList := featReg.FindAllStringSubmatch(text, -1)
			for _, sublist := range featureList {
				features = append(features, strings.Split(sublist[1], ",")...)
			}
			break
		}
	}
	if err = lineScanner.Err(); err != nil {
		t.Errorf("scan %s: %s", testFile, err)
		return
	}
	runTest := func(t *testing.T, feats ...string) {
		generator := gen.NewCliBuilder(gen.ModeSqlx).
			WithPkg(testPk).
			WithPwd(pwd).
			WithFile(testFile, doc).
			WithPos(pos).
			WithImports(nil, true).
			WithFeats(append(features, feats...))
		var buf bytes.Buffer
		if err = generator.Build(&buf); err != nil {
			t.Errorf("build: %s", err)
			return
		}
		if err = os.WriteFile(testGenFile, buf.Bytes(), 0644); err != nil {
			t.Errorf("write %s: %s", testGenFile, err)
			return
		}
		code, err := goimport.Process(testGenFile, buf.Bytes(), nil)
		if err != nil {
			t.Errorf("fix import %s: %s", testGenFile, err)
			return
		}
		if err = os.WriteFile(testGenFile, code, 0644); err != nil {
			t.Errorf("write %s: %s", testGenFile, err)
			return
		}
		defer os.Remove(testGenFile)
		var (
			stdout bytes.Buffer
			stderr bytes.Buffer
		)
		cmd := exec.Command("go", "run",
			"-tags",
			"test",
			filepath.Join(pwd, testDir))
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err = cmd.Run(); err != nil {
			t.Errorf("run: %s\n%s", err, stderr.String())
			return
		}
		if stdout.Len() > 0 {
			t.Logf("output: \n%s", stdout.String())
		}
	}
	t.Run("rt", func(t *testing.T) { runTest(t) })
	t.Run("nort", func(t *testing.T) { runTest(t, gen.FeatureSqlxNoRt) })
}
