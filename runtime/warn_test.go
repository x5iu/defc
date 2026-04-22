package defc

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

func TestWarnDroppedArgs_emitsWhenCollectedGreaterThanUsed(t *testing.T) {
	t.Cleanup(func() { resetDroppedArgWarnStateForTest() })
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	WarnDroppedArgs("FindUser", "SELECT id FROM user WHERE name = 'x'", 2, 0)
	out := buf.String()
	if !strings.Contains(out, `method "FindUser"`) {
		t.Fatalf("missing method name: %q", out)
	}
	if !strings.Contains(out, "consumed 0 argument(s) but 2 argument(s)") {
		t.Fatalf("missing counts: %q", out)
	}
	if !strings.Contains(out, "2 argument(s) were discarded") {
		t.Fatalf("missing discarded: %q", out)
	}
}

func TestWarnDroppedArgs_silentWhenEqual(t *testing.T) {
	t.Cleanup(func() { resetDroppedArgWarnStateForTest() })
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	WarnDroppedArgs("Q", "SELECT ? FROM t", 1, 1)
	if buf.Len() != 0 {
		t.Fatalf("unexpected output: %q", buf.String())
	}
}

func TestWarnDroppedArgs_oncePerMethodName(t *testing.T) {
	t.Cleanup(func() { resetDroppedArgWarnStateForTest() })
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	for i := 0; i < 10; i++ {
		WarnDroppedArgs("Repeat", "SELECT 1", 2, 0)
	}
	if strings.Count(buf.String(), "defc: method") != 1 {
		t.Fatalf("expected single defc: method line, got %q", buf.String())
	}
}

func TestWarnDroppedArgs_sqlPreviewTruncates(t *testing.T) {
	t.Cleanup(func() { resetDroppedArgWarnStateForTest() })
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	longSQL := strings.Repeat("a", 250)
	WarnDroppedArgs("T", longSQL, 2, 0)
	out := buf.String()
	if !strings.Contains(out, "...") {
		t.Fatalf("expected ellipsis in preview: %q", out)
	}
	if !strings.Contains(out, strings.Repeat("a", 180)) {
		t.Fatalf("expected long preview prefix in output: %q", out)
	}
}
