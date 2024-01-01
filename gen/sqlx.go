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

	FeatureSqlxIn          = "sqlx/in"
	FeatureSqlxLog         = "sqlx/log"
	FeatureSqlxRebind      = "sqlx/rebind"
	FeatureSqlxNoRt        = "sqlx/nort"
	FeatureSqlxFuture      = "sqlx/future"
	FeatureSqlxCallback    = "sqlx/callback"
	FeatureSqlxAnyCallback = "sqlx/any-callback"
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
	Embeds        []ast.Expr
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
	var fixedMethods []*Method = nil
	for i, method := range ctx.Methods {
		if l := len(method.Out); l == 0 || !checkErrorType(method.Out[l-1]) {
			return fmt.Errorf("checkErrorType: no 'error' found in method %s returned values",
				quote(method.Ident))
		}

		if method.SingleScan() != "" {
			if len(method.Out) != 1 {
				return fmt.Errorf("%s method expects only error returned value when `scan(expr)` option has been specified",
					quote(method.Ident))
			}
		} else {
			if len(method.Out) > 2 {
				return fmt.Errorf("%s method expects 2 returned value at most, got %d",
					quote(method.Ident),
					len(method.Out))
			}
		}

		if method.Ident == sqlxMethodWithTx {
			ctx.WithTx = true
			ctx.WithTxContext = method.HasContext()
			fixedMethods = make([]*Method, 0, len(ctx.Methods)-1)
			fixedMethods = append(fixedMethods, ctx.Methods[:i]...)
			fixedMethods = append(fixedMethods, ctx.Methods[i+1:]...)
		}
	}

	// Modifying the value of Methods within the loop can cause the loop to skip the check for one of the methods.
	// To avoid this issue, we assign the modified Methods value to fixedMethods and then assign it back to the
	// original Methods after the loop ends.
	if fixedMethods != nil {
		ctx.Methods = fixedMethods
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
	}

	if ctx.HasFeature(FeatureSqlxFuture) {
		imports = append(imports, quote("github.com/x5iu/sqlx"))
	} else {
		imports = append(imports, quote("github.com/jmoiron/sqlx"))
	}

	if ctx.HasFeature(FeatureSqlxLog) {
		imports = append(imports, quote("time"))
	}

	if ctx.HasFeature(FeatureSqlxNoRt) {
		imports = append(imports,
			quote("errors"),
			quote("strings"),
			quote("reflect"),
			quote("sync"),
			quote("bytes"),
			quote("database/sql/driver"))
	} else {
		if len(ctx.Methods) > 0 {
			imports = append(imports, quote("github.com/x5iu/defc/__rt"))
		}
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

	if !builder.disableAutoImport {
		imports, err := getImports(builder.pkg, builder.pwd, builder.file, func(node ast.Node) bool {
			switch x := node.(type) {
			case *ast.TypeSpec:
				return x.Name.Name == typeSpec.Name.Name
			}
			return false
		})

		if err != nil {
			return nil, err
		}

		for _, spec := range f.Imports {
			path := spec.Path.Value[1 : len(spec.Path.Value)-1]
			for _, imported := range imports {
				if path == imported.Path {
					var name string
					if spec.Name != nil {
						name = spec.Name.Name
					}
					if importRepr := strings.TrimSpace(name + " " + path); !in(builder.imports, importRepr) {
						builder.imports = append(builder.imports, importRepr)
					}
				}
			}
		}
	}

	var (
		methods = make([]*ast.Field, 0, len(ifaceType.Methods.List))
		embeds  = make([]ast.Expr, 0, len(ifaceType.Methods.List))
	)

	for _, method := range ifaceType.Methods.List {
		if _, ok := method.Type.(*ast.FuncType); ok {
			methods = append(methods, method)
		} else if method.Names == nil {
			embeds = append(embeds, method.Type)
		}
	}

	for _, method := range methods {
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
		Methods:   nodeMap(methods, builder.doc.InspectMethod),
		Embeds:    embeds,
		Features:  sqlxFeatures,
		Imports:   builder.imports,
		Funcs:     builder.funcs,
		Doc:       builder.doc,
	}, nil
}

func readHeader(header string, pwd string) (string, error) {
	var buf bytes.Buffer
	scanner := bufio.NewScanner(strings.NewReader(header))
	var text string
	for {
		if text == "" {
			if !scanner.Scan() {
				break
			}
			text = scanner.Text()
		}

		var next string
		for {
			if !scanner.Scan() {
				break
			}
			next = scanner.Text()
			if len(next) > 0 && (next[0] == ' ' || next[0] == '\t') {
				text += " " + trimSpace(next)
				next = "" // next is consumed here
			} else {
				break
			}
		}

		text = trimSpace(text)
		args := splitArgs(text)

		// parse #include/#script command which should be placed in a new line
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

		// now next becomes the current line
		text = next
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
			"deselect":      deselect,
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
