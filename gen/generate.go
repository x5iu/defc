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
		Package  string    `json:"package"`
		Ident    string    `json:"ident"`
		Features []string  `json:"features"`
		Imports  []string  `json:"imports"`
		Funcs    []string  `json:"funcs"`
		Schemas  []*Schema `json:"schemas"`
	}

	Schema struct {
		Meta   string   `json:"meta"`
		Header string   `json:"header"`
		In     []*Param `json:"in"`
		Out    []*Param `json:"out"`
	}

	Param struct {
		Ident string `json:"ident"`
		Type  string `json:"type"`
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
			method.OrderedIn[j] = in.Ident
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
			ResponseExpr  = "__rt.Response"
			ResponseType  = "T"
		)

		// hack generic decl and schema def
		hackCfg := *cfg
		hackCfg.Ident = sprintf("%s[%s %s]", cfg.Ident, ResponseType, ResponseExpr)
		hackCfg.Schemas = append([]*Schema{
			&Schema{
				Meta: ResponseIdent,
				Out: []*Param{
					&Param{
						Type: ResponseType,
					},
				},
			},
		}, hackCfg.Schemas...)

		// generic interface for `Response() T`
		expr, _ := parseExpr(ResponseExpr)
		generics := map[string]ast.Expr{
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
					Repr:   "T",
				},
			},
		})
		doc = append(doc, ResponseType...)

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

	// package
	/*
		buf.WriteString("package" + cfg.Package)
		buf.WriteByte('\n')
		buf.WriteByte('\n')
	*/

	// imports
	/*
		buf.WriteString("import (")
		buf.WriteByte('\n')
		for _, imp := range cfg.Imports {
			buf.WriteString(parseImport(imp))
			buf.WriteByte('\n')
		}
		buf.WriteByte(')')
		buf.WriteByte('\n')
		buf.WriteByte('\n')
	*/

	// decl
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