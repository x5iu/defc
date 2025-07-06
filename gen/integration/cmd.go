package integration

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func runCommand(t *testing.T, name string, args ...string) (success bool) {
	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)
	cmd := exec.Command(name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Logf("the integration test program encountered an error, "+
			"some information is shown below: \n%s\n", stdout.String())
		t.Errorf("run `%s %s`: \n%s", name, strings.Join(args, " "), stderr.String())
		return false
	}
	if stdout.Len() > 0 {
		t.Logf("the integration test program has been successfully completed, "+
			"with detailed information as follows: \n%s\n", stdout.String())
	}
	return true
}
