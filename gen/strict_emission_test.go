package gen

import (
	"bytes"
	"strings"
	"testing"
)

// TestStrictEmission asserts the v1.45 strict-* feature flags inject
// the expected runtime helper calls into the generated code and, when
// the flags are omitted (the v1.45 default), leave the output
// untouched. See §1.6 in the Round-3 C spec for the rollout table.
func TestStrictEmission(t *testing.T) {
	const apiSchema = `package test

import (
	"context"
	"fmt"
	"net/http"

	defc "github.com/x5iu/defc/runtime"
)

type Iface interface {
	Response() defc.FutureResponse

	// Run GET https://api.example.com/v1/{{ .path }}
	/*
		- Authorization: Bearer {{ .token }}
	*/
	Run(ctx context.Context, path, token fmt.Stringer) (*http.Response, error)
}
`
	const sqlxSchema = `package test

import (
	"context"
	"fmt"
)

type Iface interface {
	WithTx(ctx context.Context, fn func(Iface) error) error

	// Run exec bind
	// DELETE FROM t WHERE id = {{ bind $.id }}
	Run(ctx context.Context, id fmt.Stringer) error
}
`

	build := func(t *testing.T, mode Mode, src string, feats []string) string {
		t.Helper()
		var buf bytes.Buffer
		// position: find line containing "type Iface interface"
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

	t.Run("api_defaults_off", func(t *testing.T) {
		out := build(t, ModeApi, apiSchema, []string{FeatureApiFuture})
		if strings.Contains(out, "__rt.PreExecHeaderMap") {
			t.Error("expected no PreExecHeaderMap without api/strict-headers")
		}
		if strings.Contains(out, "__rt.StrictURL") {
			t.Error("expected no StrictURL without api/strict-url")
		}
	})

	t.Run("api_strict_headers_emits", func(t *testing.T) {
		out := build(t, ModeApi, apiSchema, []string{FeatureApiFuture, FeatureApiStrictHeaders})
		if !strings.Contains(out, "__rt.PreExecHeaderMap") {
			t.Error("missing PreExecHeaderMap emission under api/strict-headers")
		}
		if !strings.Contains(out, "__rt.HeaderValue") {
			t.Error("missing post-sweep HeaderValue emission under api/strict-headers")
		}
	})

	t.Run("api_strict_url_emits", func(t *testing.T) {
		out := build(t, ModeApi, apiSchema, []string{FeatureApiFuture, FeatureApiStrictURL})
		if !strings.Contains(out, "__rt.StrictURL") {
			t.Error("missing StrictURL emission")
		}
		if !strings.Contains(out, `"https://api.example.com/v1/"`) {
			t.Error("missing constantPrefix literal; got output without prefix quote")
		}
	})

	t.Run("sqlx_defaults_off", func(t *testing.T) {
		out := build(t, ModeSqlx, sqlxSchema, nil)
		if strings.Contains(out, "__rt.SQLArityCheck") {
			t.Error("expected no SQLArityCheck without sqlx/strict")
		}
	})

	t.Run("sqlx_strict_emits", func(t *testing.T) {
		out := build(t, ModeSqlx, sqlxSchema, []string{FeatureSqlxStrict})
		if !strings.Contains(out, "__rt.SQLArityCheck") {
			t.Error("missing SQLArityCheck under sqlx/strict")
		}
	})

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

	t.Run("sqlx_strict_merge_mutual_exclusion", func(t *testing.T) {
		var buf bytes.Buffer
		b := NewCliBuilder(ModeSqlx).
			WithFeats([]string{FeatureSqlxStrictMerge, FeatureSqlxLenientMerge}).
			WithPkg("test").
			WithFile("test.go", []byte(namedSchema)).
			WithPos(findTypeLine(namedSchema))
		if err := b.Build(&buf); err == nil ||
			!strings.Contains(err.Error(), "mutually exclusive") {
			t.Fatalf("expected mutual-exclusion error, got %v", err)
		}
	})
}

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

func findTypeLine(src string) int {
	for i, ln := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "type Iface interface") {
			return i
		}
	}
	return 0
}
