package integration

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	goimport "golang.org/x/tools/imports"

	"github.com/x5iu/defc/gen"
)

func TestApi(t *testing.T) {
	var (
		testPk      = "main"
		testDir     = "api"
		testFile    = "main.go"
		testGenFile = "client.gen.go"
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
	defer os.Remove(testGenFile)
	doc, err := os.ReadFile(testFile)
	if err != nil {
		t.Errorf("read %s: %s", testFile, err)
		return
	}
	var (
		featReg = regexp.MustCompile(`--features(?:\s|=)([\w-,/]+)`)
		funcReg = regexp.MustCompile(`--function(?:\s|=)([\w=]+)`)

		pos       int
		features  []string
		functions []string
	)
	lineScanner := bufio.NewScanner(bytes.NewReader(doc))
	for i := 1; lineScanner.Scan(); i++ {
		text := lineScanner.Text()
		if strings.HasPrefix(text, "//go:generate") {
			pos = i
			featureList := featReg.FindAllStringSubmatch(text, -1)
			for _, sublist := range featureList {
				features = append(features, strings.Split(sublist[1], ",")...)
			}
			functionList := funcReg.FindAllStringSubmatch(text, -1)
			for _, sublist := range functionList {
				functions = append(functions, sublist[1])
			}
			break
		}
	}
	if err = lineScanner.Err(); err != nil {
		t.Errorf("scan %s: %s", testFile, err)
		return
	}
	runTest := func(t *testing.T, feats ...string) {
		generator := gen.NewCliBuilder(gen.ModeApi).
			WithPkg(testPk).
			WithPwd(pwd).
			WithFile(testFile, doc).
			WithPos(pos).
			WithImports(nil).
			WithFeats(append(features, feats...)).
			WithFuncs(functions)
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
		if !runCommand(t, "go", "mod", "tidy") {
			return
		}
		if !runCommand(t, "go", "run", "-tags", "test", filepath.Join(pwd, testDir)) {
			return
		}
	}
	t.Run("rt", func(t *testing.T) { runTest(t) })
	t.Run("nort", func(t *testing.T) { runTest(t, gen.FeatureApiNoRt) })
}
