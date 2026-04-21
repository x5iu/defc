package lint

import (
	"strings"
	"testing"
)

func TestRunUnsafeFixture(t *testing.T) {
	rep, err := Run([]string{"testdata/unsafe.go"}, Options{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Summary.Blockers == 0 {
		t.Fatalf("expected blockers, got 0 (findings=%d)", len(rep.Findings))
	}
	kinds := map[string]bool{}
	for _, f := range rep.Findings {
		kinds[f.Kind] = true
	}
	for _, want := range []string{"sql.literal", "url.path", "url.query-value", "header.value"} {
		if !kinds[want] {
			t.Errorf("missing expected kind %q; got %v", want, kinds)
		}
	}
}

func TestRunSafeFixture(t *testing.T) {
	rep, err := Run([]string{"testdata/safe.go"}, Options{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(rep.Findings) != 0 {
		var names []string
		for _, f := range rep.Findings {
			names = append(names, f.Kind+":"+f.Action)
		}
		t.Fatalf("expected no findings; got: %s", strings.Join(names, ", "))
	}
}
