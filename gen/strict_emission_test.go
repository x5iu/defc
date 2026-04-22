package gen

import (
	"bytes"
	"strings"
	"testing"
)

// TestStrictMergeEmission asserts the sqlx/strict-merge feature flag
// rewrites NAMED arg collection to MergeNamedArgsStrict with
// error-propagation (non-nort) or inlines the strict merger body
// (nort). See CHANGELOG for the v1.45.x rollout.
func TestStrictMergeEmission(t *testing.T) {
	const namedSchema = `package test

import (
	"context"
	"fmt"
)

type Iface interface {
	WithTx(ctx context.Context, fn func(Iface) error) error

	// Run exec named
	// DELETE FROM t WHERE id = :id
	Run(ctx context.Context, id fmt.Stringer) error
}
`

	build := func(t *testing.T, mode Mode, src string, feats []string) string {
		t.Helper()
		var buf bytes.Buffer
		pos := 1
		for i, ln := range strings.Split(src, "\n") {
			if strings.HasPrefix(strings.TrimSpace(ln), "type Iface interface") {
				pos = i + 1
				break
			}
		}
		b := NewCliBuilder(mode).
			WithFeats(feats).
			WithPkg("test").
			WithFile("test.go", []byte(src)).
			WithPos(pos - 1)
		if err := b.Build(&buf); err != nil {
			t.Fatalf("build: %v", err)
		}
		return buf.String()
	}

	t.Run("sqlx_strict_merge_nonnort", func(t *testing.T) {
		out := build(t, ModeSqlx, namedSchema, []string{FeatureSqlxStrictMerge})
		if !strings.Contains(out, "__rt.MergeNamedArgsStrict") {
			t.Error("missing MergeNamedArgsStrict emission under sqlx/strict-merge")
		}
		if !strings.Contains(out, "error merging") {
			t.Error("missing merge error-propagation block")
		}
	})

	t.Run("sqlx_strict_merge_nort", func(t *testing.T) {
		out := build(t, ModeSqlx, namedSchema,
			[]string{FeatureSqlxNoRt, FeatureSqlxStrictMerge})
		if !strings.Contains(out, "(map[string]any, error)") {
			t.Error("nort+strict-merge must emit two-return inline body")
		}
		if !strings.Contains(out, "ToNamedArgs(") {
			t.Error("nort+strict-merge inline body missing source labels")
		}
	})
}
