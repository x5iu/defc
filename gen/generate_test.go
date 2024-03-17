package gen

import (
	"io"
	"testing"
)

func TestGenerate(t *testing.T) {
	declareType := &Declare{
		Ident: "Type",
		Fields: []*Field{
			{
				Ident: "ID",
				Type:  "int64",
				Tag:   `json:"id" db:"id"`,
			},
		},
	}
	t.Run("api", func(t *testing.T) {
		cfg := &Config{
			Features: []string{
				FeatureApiNoRt,
			},
			Imports: []string{
				"gofmt \"fmt\"",
			},
			Schemas: []*Schema{
				{
					Meta: "Run POST https://localhost:port/path?{{ $.query }}",
					Header: "- Content-Type: application/json; charset=utf-8\n" +
						"- Authorization: Bearer XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX\n\n" +
						"{{ $.body }}",
					In: []*Param{
						{
							Ident: "ctx",
							Type:  "context.Context",
						},
						{
							Ident: "query",
							Type:  "gofmt.Stringer",
						},
						{
							Ident: "body",
							Type:  "gofmt.Stringer",
						},
					},
					Out: []*Param{
						{Type: "*Type"},
						{Type: "error"},
					},
				},
			},
			Declare: []*Declare{
				declareType,
			},
		}
		if err := Generate(io.Discard, ModeApi, cfg); err != nil {
			t.Errorf("generate: %s", err)
			return
		}
	})
	t.Run("sqlx", func(t *testing.T) {
		cfg := &Config{
			Features: []string{
				FeatureSqlxNoRt,
			},
			Imports: []string{
				"gofmt \"fmt\"",
			},
			Schemas: []*Schema{
				{
					Meta:   "Run query many bind",
					Header: "{{ $.query }};",
					In: []*Param{
						{
							Ident: "ctx",
							Type:  "context.Context",
						},
						{
							Ident: "query",
							Type:  "gofmt.Stringer",
						},
					},
					Out: []*Param{
						{Type: "[]Type"},
						{Type: "error"},
					},
				},
			},
			Declare: []*Declare{
				declareType,
			},
		}
		if err := Generate(io.Discard, ModeSqlx, cfg); err != nil {
			t.Errorf("generate: %s", err)
			return
		}
	})
	t.Run("unknown", func(t *testing.T) {
		if err := Generate(io.Discard, 999, &Config{}); err == nil {
			t.Errorf("generate: expects errors, got nil")
			return
		} else if err.Error() != "unimplemented mode \"Mode(999)\"" {
			t.Errorf("generate: expects UnimplementedError, got => %s", err)
			return
		}
	})
}
