package gen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"sort"
	"strings"
	"text/template"

	_ "embed"
)

const (
	apiMethodInner    = "INNER"
	apiMethodResponse = "RESPONSE"

	FeatureApiCache  = "api/cache"
	FeatureApiLog    = "api/log"
	FeatureApiClient = "api/client"
	FeatureApiPage   = "api/page"
	FeatureApiError  = "api/error"
	FeatureApiNoRt   = "api/nort"
)

func (builder *CliBuilder) buildApi(w io.Writer) error {
	inspectCtx, err := builder.inspectApi()
	if err != nil {
		return fmt.Errorf("inspectApi(%s, %d): %w", quote(join(builder.pwd, builder.file)), builder.pos, err)
	}
	return inspectCtx.Build(w)
}

type apiContext struct {
	Package   string
	BuildTags []string
	Ident     string
	Generics  map[string]ast.Expr
	Methods   []*Method
	Features  []string
	Imports   []string
	Funcs     []string
	Doc       Doc
	Schema    string
}

func (ctx *apiContext) Build(w io.Writer) error {
	if !checkResponse(ctx.Methods) {
		return fmt.Errorf("checkResponse: no '%s() T' method found in Interface", apiMethodResponse)
	}

	for _, method := range ctx.Methods {
		if isResponse(method.Ident) && isInner(method.Ident) {
			if l := len(method.Out); l == 0 || !checkErrorType(method.Out[l-1]) {
				return fmt.Errorf("checkErrorType: no 'error' found in method %s returned value",
					quote(method.Ident))
			}
		}

		if (isResponse(method.Ident) || isInner(method.Ident)) &&
			(len(method.In) != 0 || len(method.Out) != 1) {
			return fmt.Errorf(
				"%s method can only have no income params "+
					"and 1 returned value", quote(method.Ident))
		}

		if isResponse(method.Ident) {
			if !checkResponseType(method) {
				return fmt.Errorf(
					"checkResponseType: returned type of %s "+
						"should be kind of *ast.Ident or *ast.StarExpr",
					quote(apiMethodResponse))
			}
		}

		if len(method.Out) > 2 {
			return fmt.Errorf("%s method expects 2 returned value at most, got %d",
				quote(method.Ident),
				len(method.Out))
		}
	}

	if err := ctx.genApiCode(w); err != nil {
		return fmt.Errorf("genApiCode: %w", err)
	}

	return nil
}

func (ctx *apiContext) SortGenerics() []string {
	idents := make([]string, 0, len(ctx.Generics))
	for k := range ctx.Generics {
		idents = append(idents, k)
	}
	sort.Slice(idents, func(i, j int) bool {
		return ctx.Generics[idents[i]].Pos() < ctx.Generics[idents[j]].Pos()
	})
	return idents
}

func (ctx *apiContext) GenericsRepr(withType bool) string {
	if len(ctx.Generics) == 0 {
		return ""
	}

	var dst bytes.Buffer
	dst.WriteByte('[')
	for index, name := range ctx.SortGenerics() {
		expr := ctx.Generics[name]
		dst.WriteString(name)
		if withType {
			dst.WriteByte(' ')
			dst.WriteString(ctx.Doc.Repr(expr))
		}
		if index < len(ctx.Generics)-1 {
			dst.WriteString(", ")
		}
	}
	dst.WriteByte(']')

	return dst.String()
}

func (ctx *apiContext) HasFeature(feature string) bool {
	for _, current := range ctx.Features {
		if current == feature {
			return true
		}
	}
	return false
}

func (ctx *apiContext) HasHeader() bool {
	for _, method := range ctx.Methods {
		if method.Header != "" {
			return true
		}
	}
	return false
}

func (ctx *apiContext) HasInner() bool {
	return hasInner(ctx.Methods)
}

func (ctx *apiContext) InnerType() ast.Node {
	for _, method := range ctx.Methods {
		if isInner(method.Ident) {
			return method.Out[0]
		}
	}
	return nil
}

func (ctx *apiContext) MethodResponse() string {
	for _, method := range ctx.Methods {
		if isResponse(method.Ident) {
			return method.Ident
		}
	}
	return apiMethodResponse
}

func (ctx *apiContext) MethodInner() string {
	for _, method := range ctx.Methods {
		if isInner(method.Ident) {
			return method.Ident
		}
	}
	return apiMethodInner
}

func (ctx apiContext) MergedImports() (imports []string) {
	imports = []string{
		quote("fmt"),
		quote("io"),
		quote("net/http"),
		quote("text/template"),
	}

	if ctx.HasFeature(FeatureApiLog) {
		imports = append(imports, quote("time"))
	}

	if ctx.HasFeature(FeatureApiNoRt) {
		imports = append(imports,
			quote("bytes"),
			quote("sync"),
			quote("reflect"))
	} else {
		imports = append(imports, quote("github.com/x5iu/defc/__rt"))
	}

	if ctx.HasHeader() {
		imports = append(imports, quote("bufio"))
		imports = append(imports, quote("net/textproto"))
	}

	if importContext(ctx.Methods) {
		imports = append(imports, quote("context"))
	}

	for _, imp := range ctx.Imports {
		if !in(imports, imp) {
			imports = append(imports, parseImport(imp))
		}
	}

	return imports
}

func (ctx *apiContext) AdditionalFuncs() (funcMap map[string]string) {
	funcMap = make(map[string]string, len(ctx.Funcs))
	for _, fn := range ctx.Funcs {
		if key, value, ok := cutkv(fn); ok {
			funcMap[key] = value
		}
	}
	return funcMap
}

func (builder *CliBuilder) inspectApi() (*apiContext, error) {
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
		if !checkInput(method.Type.(*ast.FuncType)) {
			return nil, fmt.Errorf(""+
				"input params for method %s should "+
				"contain 'Name' and 'Type' both",
				quote(method.Names[0].Name))
		}
	}

	apiFeatures := make([]string, 0, len(builder.feats))
	for _, feature := range builder.feats {
		if hasPrefix(feature, "api") {
			apiFeatures = append(apiFeatures, feature)
		}
	}

	generics := make(map[string]ast.Expr, 10)
	if typeSpec.TypeParams != nil {
		for _, param := range typeSpec.TypeParams.List {
			for _, name := range param.Names {
				generics[name.Name] = param.Type
			}
		}
	}

	return &apiContext{
		Package:   builder.pkg,
		BuildTags: parseBuildTags(builder.doc),
		Ident:     typeSpec.Name.Name,
		Generics:  generics,
		Methods:   nodeMap(ifaceType.Methods.List, builder.doc.InspectMethod),
		Features:  apiFeatures,
		Imports:   builder.imports,
		Funcs:     builder.funcs,
		Doc:       builder.doc,
	}, nil
}

func checkResponse(methods []*Method) bool {
	for _, method := range methods {
		if isResponse(method.Ident) {
			return true
		}
	}
	return false
}

func checkResponseType(method *Method) bool {
	node := getNode(method.Out[0])
	switch node.(type) {
	case *ast.Ident, *ast.StarExpr:
		return true
	default:
		return false
	}
}

func hasInner(methods []*Method) bool {
	for _, method := range methods {
		if isInner(method.Ident) {
			return true
		}
	}
	return false
}

func importContext(methods []*Method) bool {
	for _, method := range methods {
		if method.HasContext() {
			return true
		}
	}
	return false
}

func isResponse(ident string) bool {
	return toUpper(ident) == apiMethodResponse
}

func isInner(ident string) bool {
	return toUpper(ident) == apiMethodInner
}

//go:embed templates/api.tmpl
var apiTemplate string

func (ctx *apiContext) genApiCode(w io.Writer) error {
	tmpl, err := template.
		New("defc(api)").
		Funcs(template.FuncMap{
			"quote":         quote,
			"isPointer":     isPointer,
			"indirect":      indirect,
			"importContext": importContext,
			"sub":           func(x, y int) int { return x - y },
			"getRepr":       func(node ast.Node) string { return ctx.Doc.Repr(node) },
			"methodResp":    ctx.MethodResponse,
			"methodInner":   ctx.MethodInner,
			"isResponse":    isResponse,
			"isInner":       isInner,
			"httpMethodHasBody": func(method string) bool {
				switch method {
				case http.MethodGet:
					return false
				case http.MethodPost, http.MethodPut, http.MethodPatch:
					return true
				default:
					return false
				}
			},
			"headerHasBody": func(header string) bool {
				if index := strings.Index(header, "\r\n\r\n"); index != -1 {
					return len(trimSpace(header[index+4:])) > 0
				}
				if index := strings.Index(header, "\n\n"); index != -1 {
					return len(trimSpace(header[index+2:])) > 0
				}
				return false
			},
		}).
		Parse(apiTemplate)

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
