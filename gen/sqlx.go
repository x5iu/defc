package gen

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"strings"
	"text/template"

	_ "embed"
)

const (
	sqlxOpExec  = "EXEC"
	sqlxOpQuery = "QUERY"

	sqlxMethodWithTx = "WithTx"

	sqlxCmdInclude = "#INCLUDE"

	FeatureSqlxLog    = "sqlx/log"
	FeatureSqlxRebind = "sqlx/rebind"
)

func (builder *Builder) buildSqlx(w io.Writer) error {
	inspectCtx, err := builder.inspectSqlx()
	if err != nil {
		return fmt.Errorf("inspectSqlx(%s, %d): %w", quote(join(builder.pwd, builder.file)), builder.pos, err)
	}

	for i, method := range inspectCtx.Methods {
		if l := len(method.Out); l == 0 || !checkErrorType(method.Out[l-1]) {
			return fmt.Errorf("checkErrorType: no 'error' found in method %s returned value",
				quote(method.Ident))
		}

		if len(method.Out) > 2 {
			return fmt.Errorf("%s method expects 2 returned value at most, got %d",
				quote(method.Ident),
				len(method.Out))
		}

		if method.Ident == sqlxMethodWithTx {
			inspectCtx.WithTx = true
			inspectCtx.WithTxContext = method.HasContext()
			inspectCtx.Methods = append(inspectCtx.Methods[:i], inspectCtx.Methods[i+1:]...)
		}
	}

	if err = genSqlxCode(inspectCtx, builder.pwd, builder.doc, w); err != nil {
		return fmt.Errorf("genApiCode: \n\n%#v\n\n%w", inspectCtx, err)
	}

	return nil
}

type sqlxContext struct {
	Package       string
	Ident         string
	Methods       []*Method
	WithTx        bool
	WithTxContext bool
	Features      []string
	Imports       []string
	Funcs         []string
	Doc           Doc
}

func (ctx *sqlxContext) HasFeature(feature string) bool {
	for _, current := range ctx.Features {
		if current == feature {
			return true
		}
	}
	return false
}

func (ctx *sqlxContext) MergedImports() (imports []string) {
	imports = []string{
		quote("fmt"),
		quote("strconv"),
		quote("database/sql"),
		quote("strings"),
		quote("context"),
		quote("text/template"),
		quote("github.com/jmoiron/sqlx"),
		quote("github.com/x5iu/defc/__rt"),
	}

	if ctx.HasFeature(FeatureSqlxLog) {
		imports = append(imports, quote("time"))
	}

	for _, imp := range ctx.Imports {
		if !in(imports, imp) {
			imports = append(imports, parseImport(imp))
		}
	}

	return imports
}

func (ctx *sqlxContext) AdditionalFuncs() (funcMap map[string]string) {
	funcMap = make(map[string]string, len(ctx.Funcs))
	for _, fn := range ctx.Funcs {
		if key, value, ok := cutkv(fn); ok {
			funcMap[key] = value
		}
	}
	return funcMap
}

func (builder *Builder) inspectSqlx() (*sqlxContext, error) {
	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, builder.file, builder.doc.Bytes(), parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var (
		genDecl   *ast.GenDecl
		typeSpec  *ast.TypeSpec
		ifaceType *ast.InterfaceType
	)

	line := builder.pos + 1
inspectDecl:
	for _, declIface := range f.Decls {
		if hit(fset, declIface, line) {
			if decl := declIface.(*ast.GenDecl); decl.Tok == token.TYPE {
				genDecl = decl
				break inspectDecl
			}
		}
	}

	if genDecl == nil {
		return nil, fmt.Errorf(
			"no available 'Interface' type declaration (*ast.GenDecl) found, "+
				"available *ast.GenDecl are: \n\n"+
				"%s\n\n", concat(nodeMap(f.Decls, fmtNode), "\n"))
	}

inspectType:
	for _, specIface := range genDecl.Specs {
		if hit(fset, specIface, line) {
			spec := specIface.(*ast.TypeSpec)
			if iface, ok := spec.Type.(*ast.InterfaceType); ok && hit(fset, iface, line) {
				typeSpec = spec
				ifaceType = iface
				break inspectType
			}
		}
	}

	if ifaceType == nil {
		return nil, fmt.Errorf(
			"no available 'Interface' type declaration (*ast.InterfaceType) found, "+
				"available *ast.GenDecl are: \n\n"+
				"%s\n\n", concat(nodeMap(f.Decls, fmtNode), "\n"))
	}

	for _, method := range ifaceType.Methods.List {
		if name := method.Names[0].Name; name != sqlxMethodWithTx && !checkInput(method.Type.(*ast.FuncType)) {
			return nil, fmt.Errorf(""+
				"input params for method %s should "+
				"contain 'Name' and 'Type' both",
				quote(name))
		}
	}

	sqlxFeatures := make([]string, 0, len(builder.feats))
	for _, feature := range builder.feats {
		if hasPrefix(feature, "sqlx") {
			sqlxFeatures = append(sqlxFeatures, feature)
		}
	}

	return &sqlxContext{
		Package:  builder.pkg,
		Ident:    typeSpec.Name.Name,
		Methods:  nodeMap(ifaceType.Methods.List, builder.doc.InspectMethod),
		Features: sqlxFeatures,
		Imports:  builder.imports,
		Funcs:    builder.funcs,
		Doc:      builder.doc,
	}, nil
}

func readHeader(header string, pwd string) (string, error) {
	var buf bytes.Buffer
	scanner := bufio.NewScanner(strings.NewReader(header))
	for scanner.Scan() {
		text := scanner.Text()
		args := splitArgs(text)
		if len(args) == 2 && toUpper(args[0]) == sqlxCmdInclude {
			path := args[1]
			if !isAbs(path) {
				path = join(pwd, path)
			}
			content, err := read(path)
			if err != nil {
				return "", fmt.Errorf("os.ReadFile(%s): %w", quote(path), err)
			}
			buf.WriteString(string(content))
		} else {
			buf.WriteString(text)
		}
		buf.WriteString("\r\n")
	}
	return buf.String(), nil
}

func hasOption(opts []string, opt string) bool {
	for _, o := range opts {
		if o == toUpper(opt) {
			return true
		}
	}
	return false
}

//go:embed templates/sqlx.tmpl
var sqlxTemplate string

func genSqlxCode(ctx *sqlxContext, pwd string, doc Doc, w io.Writer) error {
	tmpl, err := template.
		New("defc(sqlx)").
		Funcs(template.FuncMap{
			"quote":         quote,
			"hasOption":     hasOption,
			"isSlice":       isSlice,
			"isPointer":     isPointer,
			"indirect":      indirect,
			"readHeader":    func(header string) (string, error) { return readHeader(header, pwd) },
			"isContextType": func(ident string, expr ast.Expr) bool { return doc.IsContextType(ident, expr) },
			"sub":           func(x, y int) int { return x - y },
			"getRepr":       func(node ast.Node) string { return doc.Repr(node) },
			"isQuery":       func(op string) bool { return op == sqlxOpQuery },
			"isExec":        func(op string) bool { return op == sqlxOpExec },
		}).
		Parse(sqlxTemplate)

	if err != nil {
		return err
	}

	return tmpl.Execute(w, ctx)
}
