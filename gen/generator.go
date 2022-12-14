package gen

import (
	"go/ast"
	"io"
)

type Mode int

const (
	ModeStart Mode = iota
	ModeApi
	ModeSqlx
	ModeEnd
)

func (mode Mode) String() string {
	switch mode {
	case ModeApi:
		return "api"
	case ModeSqlx:
		return "sqlx"
	default:
		return sprintf("Mode(%d)", mode)
	}
}

func (mode Mode) isValid() bool {
	return ModeStart < mode && mode < ModeEnd
}

type Doc []byte

func (doc Doc) Bytes() []byte {
	return doc
}

func (doc Doc) Repr(node ast.Node) string {
	return getRepr(node, doc)
}

func (doc Doc) NewType(expr ast.Expr) string {
	return newType(expr, doc)
}

func (doc Doc) InspectMethod(node ast.Node) *Method {
	return inspectMethod(node, doc)
}

func (doc Doc) IsContextType(ident string, expr ast.Expr) bool {
	return isContextType(ident, expr, doc)
}

func NewBuilder(mode Mode) *Builder {
	assert(mode.isValid(), "invalid mode")
	return &Builder{
		mode: mode,
	}
}

type Builder struct {
	// mode
	mode Mode

	// feats
	feats []string

	// imports
	imports []string

	// pkg package name
	pkg string

	// pwd current working directory
	pwd string

	// file current file
	file string

	// doc total content of current file
	doc Doc

	// pos position of `go generate` command
	pos int
}

func (builder *Builder) WithFeats(feats []string) *Builder {
	builder.feats = feats
	return builder
}

func (builder *Builder) WithImports(imports []string) *Builder {
	builder.imports = imports
	return builder
}

func (builder *Builder) WithPkg(pkg string) *Builder {
	builder.pkg = pkg
	return builder
}

func (builder *Builder) WithPwd(pwd string) *Builder {
	builder.pwd = pwd
	return builder
}

func (builder *Builder) WithFile(file string, doc []byte) *Builder {
	builder.file = file
	builder.doc = doc
	return builder
}

func (builder *Builder) WithPos(pos int) *Builder {
	builder.pos = pos
	return builder
}

func (builder *Builder) Build(w io.Writer) error {
	switch builder.mode {
	case ModeApi:
		return builder.buildApi(w)
	case ModeSqlx:
		return builder.buildSqlx(w)
	}
	return nil
}

func assert(expr bool, msg string) {
	if !expr {
		panic(msg)
	}
}
