package gen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"io"
)

type (
	Builder interface {
		Build(w io.Writer) error
	}

	Config struct {
		Package  string     `json:"package" toml:"package" yaml:"package"`
		Ident    string     `json:"ident" toml:"ident" yaml:"ident"`
		Features []string   `json:"features" toml:"features" yaml:"features"`
		Imports  []string   `json:"imports" toml:"imports" yaml:"imports"`
		Funcs    []string   `json:"funcs" toml:"funcs" yaml:"funcs"`
		Schemas  []*Schema  `json:"schemas" toml:"schemas" yaml:"schemas"`
		Include  string     `json:"include" toml:"include" yaml:"include"`
		Declare  []*Declare `json:"declare" toml:"declare" yaml:"declare"`
	}

	Schema struct {
		Meta   string   `json:"meta" toml:"meta" yaml:"meta"`
		Header string   `json:"header" toml:"header" yaml:"header"`
		In     []*Param `json:"in" toml:"in" yaml:"in"`
		Out    []*Param `json:"out" toml:"out" yaml:"out"`
	}

	Param struct {
		Ident string `json:"ident" toml:"ident" yaml:"ident"`
		Type  string `json:"type" toml:"type" yaml:"type"`
	}

	Declare struct {
		Ident  string   `json:"ident" toml:"ident" yaml:"ident"`
		Fields []*Field `json:"fields" toml:"fields" yaml:"fields"`
	}

	Field struct {
		Ident string `json:"ident" toml:"ident" yaml:"ident"`
		Type  string `json:"type" toml:"type" yaml:"type"`
		Tag   string `json:"tag" toml:"tag" yaml:"tag"`
	}
)

func Generate(w io.Writer, mode Mode, cfg *Config) error {
	builder, err := toBuilder(mode, cfg)
	if err != nil {
		return err
	}
	return builder.Build(w)
}

func toBuilder(mode Mode, cfg *Config) (Builder, error) {
	methods := make([]*Method, len(cfg.Schemas))
	doc := make(Doc, 0, len(cfg.Schemas)*(3+2)*7)

	for i := 0; i < len(cfg.Schemas); i++ {
		schema := cfg.Schemas[i]
		method := &Method{
			Meta:      schema.Meta,
			Header:    schema.Header,
			Ident:     getIdent(schema.Meta),
			OrderedIn: make([]string, len(schema.In)),
			In:        make(map[string]ast.Expr, len(schema.In)),
			UnnamedIn: make([]ast.Expr, 0, 3),
			Out:       make([]ast.Expr, len(schema.Out)),
		}
		for j := 0; j < len(schema.In); j++ {
			in := schema.In[j]
			expr, err := parseExpr(in.Type)
			if err != nil {
				return nil, fmt.Errorf("invalid expr: %w", err)
			}
			wrapped := &Expr{
				Expr:   expr,
				Offset: len(doc),
				Repr:   in.Type,
			}
			if in.Ident == "" {
				method.UnnamedIn = append(method.UnnamedIn, wrapped)
			} else {
				method.OrderedIn[j] = in.Ident
				method.In[in.Ident] = wrapped
			}
			doc = append(doc, in.Type...)
		}
		for k := 0; k < len(schema.Out); k++ {
			out := schema.Out[k]
			expr, err := parseExpr(out.Type)
			if err != nil {
				return nil, fmt.Errorf("invalid expr: %w", err)
			}
			method.Out[k] = &Expr{
				Expr:   expr,
				Offset: len(doc),
				Repr:   out.Type,
			}
			doc = append(doc, out.Type...)
		}
		methods[i] = method
	}

	// lazy update
	defer func() {
		for _, method := range methods {
			method.Source = doc
		}
	}()

	switch mode {
	case ModeApi:
		const (
			ResponseIdent = "Response"
			ResponseType  = "T"
		)

		var (
			ResponseExpr = "__rt.Response"
		)

		if in(cfg.Features, FeatureApiNoRt) {
			ResponseExpr = sprintf("%sResponseInterface", cfg.Ident)
		}

		hasResponse := func(schemas []*Schema) bool {
			for _, schema := range schemas {
				if getIdent(schema.Meta) == ResponseIdent {
					return true
				}
			}
			return false
		}

		// hack generic decl and schema def
		hackCfg := *cfg
		generics := make(map[string]ast.Expr)
		if !hasResponse(hackCfg.Schemas) {
			hackCfg.Ident = sprintf("%s[%s %s]", cfg.Ident, ResponseType, ResponseExpr)
			hackCfg.Schemas = append([]*Schema{
				{
					Meta: ResponseIdent,
					Out: []*Param{
						{
							Type: ResponseType,
						},
					},
				},
			}, hackCfg.Schemas...)

			// generic interface for `Response() T`
			expr, _ := parseExpr(ResponseExpr)
			generics = map[string]ast.Expr{
				ResponseType: &Expr{
					Expr:   expr,
					Offset: len(doc),
					Repr:   ResponseExpr,
				},
			}
			doc = append(doc, ResponseExpr...)

			// response type
			expr, _ = parseExpr(ResponseType)
			methods = append(methods, &Method{
				Ident: ResponseIdent,
				Out: []ast.Expr{
					&Expr{
						Expr:   expr,
						Offset: len(doc),
						Repr:   ResponseType,
					},
				},
			})
			doc = append(doc, ResponseType...)
		}

		return &apiContext{
			Package:  cfg.Package,
			Ident:    cfg.Ident,
			Generics: generics,
			Methods:  methods,
			Features: cfg.Features,
			Imports:  cfg.Imports,
			Funcs:    cfg.Funcs,
			Doc:      doc,
			Schema:   format(&hackCfg),
		}, nil
	case ModeSqlx:
		return &sqlxContext{
			Package:  cfg.Package,
			Ident:    cfg.Ident,
			Methods:  methods,
			Features: cfg.Features,
			Imports:  cfg.Imports,
			Funcs:    cfg.Funcs,
			Doc:      doc,
			Schema:   format(cfg),
		}, nil
	}

	return nil, fmt.Errorf("unimplemented mode %q", mode.String())
}

func format(cfg *Config) string {
	var buf bytes.Buffer
	buf.WriteString("type " + cfg.Ident + " interface {")
	buf.WriteByte('\n')
	for _, schema := range cfg.Schemas {
		buf.WriteString(getIdent(schema.Meta))
		buf.WriteByte('(')
		for _, in := range schema.In {
			buf.WriteString(in.Ident + " " + in.Type + ", ")
		}
		buf.WriteByte(')')
		buf.WriteByte('(')
		for _, out := range schema.Out {
			buf.WriteString(out.Ident + " " + out.Type + ", ")
		}
		buf.WriteByte(')')
		buf.WriteByte('\n')
	}
	buf.WriteByte('}')
	buf.WriteByte('\n')
	buf.WriteByte('\n')
	buf.WriteString(cfg.Include)
	buf.WriteByte('\n')
	buf.WriteByte('\n')
	for _, declare := range cfg.Declare {
		buf.WriteString("type " + declare.Ident + " struct {")
		buf.WriteByte('\n')
		for _, field := range declare.Fields {
			buf.WriteString(field.Ident)
			buf.WriteByte(' ')
			buf.WriteString(field.Type)
			buf.WriteByte(' ')
			buf.WriteString("`" + field.Tag + "`")
			buf.WriteByte('\n')
		}
		buf.WriteByte('}')
		buf.WriteByte('\n')
		buf.WriteByte('\n')
	}
	return buf.String()
}

type Expr struct {
	ast.Expr
	Offset int
	Repr   string
}

func (expr *Expr) Pos() token.Pos {
	return token.Pos(expr.Offset + 1)
}

func (expr *Expr) End() token.Pos {
	return expr.Pos() + (expr.Expr.End() - expr.Expr.Pos())
}

func (expr *Expr) String() string {
	return expr.Repr
}

func (expr *Expr) Unwrap() ast.Node {
	return expr.Expr
}
