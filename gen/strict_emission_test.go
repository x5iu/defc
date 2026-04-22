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
}

