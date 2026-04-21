package gen

import (
	"go/ast"
	"io"
	"time"
)

type Mode int

const (
	ModeStart Mode = iota
	ModeApi
	ModeSqlx
	ModeRpc
	ModeEnd
)

func (mode Mode) String() string {
	switch mode {
	case ModeApi:
		return "api"
	case ModeSqlx:
		return "sqlx"
	case ModeRpc:
		return "rpc"
	default:
		return sprintf("Mode(%d)", mode)
	}
}

func (mode Mode) IsValid() bool {
	return ModeStart < mode && mode < ModeEnd
}

type Doc []byte

func (doc Doc) Bytes() []byte {
	return doc
}

func (doc Doc) Repr(node ast.Node) string {
	return getRepr(node, doc)
}

func (doc Doc) InspectMethod(node *ast.Field) *Method {
	return inspectMethod(node, doc)
}

func (doc Doc) IsContextType(ident string, expr ast.Expr) bool {
	return isContextType(ident, expr, doc)
}

func NewCliBuilder(mode Mode) *CliBuilder {
	assert(mode.IsValid(), "invalid mode")
	return &CliBuilder{
		mode: mode,
	}
}

type CliBuilder struct {
	// mode
	mode Mode

	// feats
	feats []string

	// imports
	imports []string

	// funcs
	funcs []string

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

	// template
	template string

	// allowScript gates the sqlx-mode `#SCRIPT` directive.
	allowScript bool

	// scriptTimeout caps each `#SCRIPT` invocation.
	scriptTimeout time.Duration

	// scriptEnv is an extra names-only env allow-list for `#SCRIPT`.
	scriptEnv []string

	// includeRoots is an optional list of extra filesystem roots under
	// which sqlx-mode `#INCLUDE` paths may resolve.
	includeRoots []string
}

func (builder *CliBuilder) WithFeats(feats []string) *CliBuilder {
	builder.feats = feats
	return builder
}

func (builder *CliBuilder) WithImports(imports []string) *CliBuilder {
	builder.imports = imports
	return builder
}

func (builder *CliBuilder) WithFuncs(funcs []string) *CliBuilder {
	builder.funcs = funcs
	return builder
}

func (builder *CliBuilder) WithPkg(pkg string) *CliBuilder {
	builder.pkg = pkg
	return builder
}

func (builder *CliBuilder) WithPwd(pwd string) *CliBuilder {
	builder.pwd = pwd
	return builder
}

func (builder *CliBuilder) WithFile(file string, doc []byte) *CliBuilder {
	builder.file = file
	builder.doc = doc
	return builder
}

func (builder *CliBuilder) WithPos(pos int) *CliBuilder {
	builder.pos = pos
	return builder
}

func (builder *CliBuilder) WithTemplate(template string) *CliBuilder {
	builder.template = template
	return builder
}

func (builder *CliBuilder) WithAllowScript(allow bool) *CliBuilder {
	builder.allowScript = allow
	return builder
}

func (builder *CliBuilder) WithScriptTimeout(timeout time.Duration) *CliBuilder {
	builder.scriptTimeout = timeout
	return builder
}

func (builder *CliBuilder) WithScriptEnv(env []string) *CliBuilder {
	builder.scriptEnv = env
	return builder
}

func (builder *CliBuilder) WithIncludeRoots(roots []string) *CliBuilder {
	builder.includeRoots = roots
	return builder
}

func (builder *CliBuilder) Build(w io.Writer) error {
	switch builder.mode {
	case ModeApi:
		return builder.buildApi(w)
	case ModeSqlx:
		return builder.buildSqlx(w)
	case ModeRpc:
		return builder.buildRpc(w)
	default:
	}
	return nil
}
