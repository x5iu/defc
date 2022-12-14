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
	sqlxCmdScript  = "#SCRIPT"

	FeatureSqlxLog    = "sqlx/log"
	FeatureSqlxRebind = "sqlx/rebind"
	FeatureSqlxNoRt   = "sqlx/nort"
)

func (builder *CliBuilder) buildSqlx(w io.Writer) error {
	inspectCtx, err := builder.inspectSqlx()
	if err != nil {
		return fmt.Errorf("inspectSqlx(%s, %d): %w", quote(join(builder.pwd, builder.file)), builder.pos, err)
	}
	return inspectCtx.Build(w)
}

type sqlxContext struct {
	Package       string
	BuildTags     []string
	Ident         string
	Methods       []*Method
	WithTx        bool
	WithTxContext bool
	Features      []string
	Imports       []string
	Funcs         []string
	Pwd           string
	Doc           Doc
	Schema        string
}

func (ctx *sqlxContext) Build(w io.Writer) error {
	for i, method := range ctx.Methods {
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
			ctx.WithTx = true
			ctx.WithTxContext = method.HasContext()
			ctx.Methods = append(ctx.Methods[:i], ctx.Methods[i+1:]...)
		}
	}

	if err := ctx.genSqlxCode(w); err != nil {
		return fmt.Errorf("genSqlxCode: %w", err)
	}

	return nil
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
		quote("context"),
		quote("text/template"),
		quote("github.com/jmoiron/sqlx"),
	}

	if ctx.HasFeature(FeatureSqlxLog) {
		imports = append(imports, quote("time"))
	}

	if ctx.HasFeature(FeatureSqlxNoRt) {
		imports = append(imports,
			quote("strings"),
			quote("reflect"),
			quote("sync"),
			quote("bytes"),
			quote("database/sql/driver"))
	} else {
		imports = append(imports, quote("github.com/x5iu/defc/__rt"))
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

func (builder *CliBuilder) inspectSqlx() (*sqlxContext, error) {
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
		Package:   builder.pkg,
		BuildTags: parseBuildTags(builder.doc),
		Ident:     typeSpec.Name.Name,
		Methods:   nodeMap(ifaceType.Methods.List, builder.doc.InspectMethod),
		Features:  sqlxFeatures,
		Imports:   builder.imports,
		Funcs:     builder.funcs,
		Doc:       builder.doc,
	}, nil
}

func readHeader(header string, pwd string) (string, error) {
	var buf bytes.Buffer
	scanner := bufio.NewScanner(strings.NewReader(header))
	for scanner.Scan() {
		text := scanner.Text()
		args := splitArgs(text)
		// parse #include command which should be placed in a new line
		if len(args) == 2 && toUpper(args[0]) == sqlxCmdInclude {
			// unquote path pattern if it is quoted
			path := unquote(args[1])
			if !isAbs(path) {
				path = join(pwd, path)
			}
			// get filenames that match the pattern
			matches, err := glob(path)
			if err != nil {
				return "", err
			}
			// read each file into buffer
			for _, path = range matches {
				if !isAbs(path) {
					path = join(pwd, path)
				}
				content, err := read(path)
				if err != nil {
					return "", fmt.Errorf("os.ReadFile(%s): %w", quote(path), err)
				}
				buf.WriteString(string(content))
			}
		} else if len(args) > 1 && toUpper(args[0]) == sqlxCmdScript {
			output, err := runCommand(args[1:])
			if err != nil {
				return "", err
			}
			buf.WriteString(output)
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

func (ctx *sqlxContext) genSqlxCode(w io.Writer) error {
	tmpl, err := template.
		New("defc(sqlx)").
		Funcs(template.FuncMap{
			"quote":         quote,
			"hasOption":     hasOption,
			"isSlice":       isSlice,
			"isPointer":     isPointer,
			"indirect":      indirect,
			"readHeader":    func(header string) (string, error) { return readHeader(header, ctx.Pwd) },
			"isContextType": func(ident string, expr ast.Expr) bool { return ctx.Doc.IsContextType(ident, expr) },
			"sub":           func(x, y int) int { return x - y },
			"getRepr":       func(node ast.Node) string { return ctx.Doc.Repr(node) },
			"isQuery":       func(op string) bool { return op == sqlxOpQuery },
			"isExec":        func(op string) bool { return op == sqlxOpExec },
		}).
		Parse(sqlxTemplate)

	if err != nil {
		return err
	}

	if ctx.Schema != "" {
		if tmpl, err = tmpl.Parse(sprintf(`{{ define "schema" }} %s {{ end }}`, ctx.Schema)); err != nil {
			return err
		}
	}

	return tmpl.Execute(w, ctx)
}
