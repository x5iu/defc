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
	"text/template"

	_ "embed"
)

const (
	apiMethodOptions         = "OPTIONS"
	apiMethodResponseHandler = "RESPONSEHANDLER"

	// Deprecated: use apiMethodOptions instead
	apiMethodInner = "INNER"
	// Deprecated: use apiMethodResponseHandler instead
	apiMethodResponse = "RESPONSE"

	FeatureApiCache        = "api/cache"
	FeatureApiLog          = "api/log"
	FeatureApiLogx         = "api/logx"
	FeatureApiClient       = "api/client"
	FeatureApiPage         = "api/page"
	FeatureApiError        = "api/error"
	FeatureApiNoRt         = "api/nort"
	FeatureApiFuture       = "api/future"
	FeatureApiIgnoreStatus = "api/ignore-status"
	FeatureApiGzip         = "api/gzip"
	FeatureApiRetry        = "api/retry"
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
}

func (ctx *apiContext) Build(w io.Writer) error {
	if !checkResponse(ctx.Methods) {
		return fmt.Errorf("checkResponse: no '%s() T' method found in Interface", apiMethodResponse)
	}

	for _, method := range ctx.Methods {
		if !isResponse(method.Ident) && !isInner(method.Ident) {
			if l := len(method.Out); l == 0 || !checkErrorType(method.Out[l-1]) {
				return fmt.Errorf("checkErrorType: no 'error' found in method %s returned values",
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
			methodOut0 := method.Out[0]
			if !checkResponseType(methodOut0) {
				return fmt.Errorf(
					"checkResponseType: returned type of %s "+
						"should be kind of "+
						"*ast.Ident/"+
						"*ast.StarExpr/"+
						"*ast.SelectorExpr/"+
						"*ast.IndexExpr/"+
						"*ast.IndexListExpr"+
						", got %T",
					quote(apiMethodResponse),
					methodOut0)
			}
		}

		// [2023-06-11] we limit 2 returned values on v1.0.0, now it is time to cancel this limitation
		/*
			if len(method.Out) > 2 {
				return fmt.Errorf("%s method expects 2 returned value at most, got %d",
					quote(method.Ident),
					len(method.Out))
			}
		*/
	}

	// When using the api/future feature without enabling the api/error feature, it may cause connections
	// to not be closed properly, potentially leading to memory leak risks. To prevent this from happening,
	// when the api/future feature is enabled, the api/error feature must also be enforced.
	if in(ctx.Features, FeatureApiFuture) && !in(ctx.Features, FeatureApiError) {
		ctx.Features = append(ctx.Features, FeatureApiError)
	}

	// We do not allow the use of api/ignore-status without enabling the api/future feature, as this would
	// cause callers to miss out on determining exceptional response codes.
	if in(ctx.Features, FeatureApiIgnoreStatus) && !in(ctx.Features, FeatureApiFuture) {
		return fmt.Errorf("api/ignore-status feature requires api/future feature to be enabled")
	}

	if in(ctx.Features, FeatureApiGzip) && in(ctx.Features, FeatureApiNoRt) {
		return fmt.Errorf("api/gzip feature requires api/nort feature to be disabled")
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

func (ctx *apiContext) HasBody() bool {
	for _, method := range ctx.Methods {
		if httpMethodHasBody(method.MethodHTTP()) && headerHasBody(method.TmplHeader()) {
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

func (ctx *apiContext) MergedImports() (imports []string) {
	imports = []string{
		quote("fmt"),
		quote("io"),
		quote("net/http"),
		quote("text/template"),
	}

	if ctx.HasFeature(FeatureApiLog) || ctx.HasFeature(FeatureApiLogx) {
		imports = append(imports, quote("time"))
		imports = append(imports, quote("context"))
	}

	if ctx.HasFeature(FeatureApiNoRt) {
		imports = append(imports,
			quote("bytes"),
			quote("sync"),
			quote("reflect"))
	} else {
		imports = append(imports, parseImport("__rt github.com/x5iu/defc/runtime"))
	}

	if ctx.HasHeader() {
		imports = append(imports, quote("bufio"))
		imports = append(imports, quote("net/textproto"))
		if ctx.HasBody() && ctx.HasFeature(FeatureApiLogx) {
			imports = append(imports, quote("bytes"))
		}
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
		if surroundLine(fset, declIface, line) {
			if decl, ok := declIface.(*ast.GenDecl); ok && decl.Tok == token.TYPE {
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
		if afterLine(fset, specIface, line) {
			if spec, ok := specIface.(*ast.TypeSpec); ok {
				if iface, ok := spec.Type.(*ast.InterfaceType); ok && afterLine(fset, iface, line) {
					typeSpec = spec
					ifaceType = iface
					break inspectType
				}
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
		if funcType, ok := method.Type.(*ast.FuncType); ok {
			if !checkInput(funcType) {
				return nil, fmt.Errorf(""+
					"input params for method %s should "+
					"contain 'Name' and 'Type' both",
					quote(method.Names[0].Name))
			}
		}
	}

	apiFeatures := make([]string, 0, len(builder.feats))
	for _, feature := range builder.feats {
		if hasPrefix(feature, "api") {
			apiFeatures = append(apiFeatures, feature)
		}
	}

	generics := make(map[string]ast.Expr, 16)
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
		Methods:   typeMap(ifaceType.Methods.List, builder.doc.InspectMethod),
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

func checkResponseType(node ast.Node) bool {
	switch node.(type) {
	case *ast.Ident, *ast.StarExpr, *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
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
	ident = toUpper(ident)
	return ident == apiMethodResponse || ident == apiMethodResponseHandler
}

func isInner(ident string) bool {
	ident = toUpper(ident)
	return ident == apiMethodInner || ident == apiMethodOptions
}

func httpMethodHasBody(method string) bool {
	switch method {
	case http.MethodGet:
		return false
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return true
	default:
		return false
	}
}

func headerHasBody(header string) bool {
	if idx := index(header, "\r\n\r\n"); idx != -1 {
		return len(trimSpace(header[idx+4:])) > 0
	}
	if idx := index(header, "\n\n"); idx != -1 {
		return len(trimSpace(header[idx+2:])) > 0
	}
	return false
}

//go:embed template/api.tmpl
var apiTemplate string

func (ctx *apiContext) genApiCode(w io.Writer) error {
	tmpl, err := template.
		New("defc(api)").
		Funcs(template.FuncMap{
			"quote":             quote,
			"isPointer":         isPointer,
			"indirect":          indirect,
			"importContext":     importContext,
			"sub":               func(x, y int) int { return x - y },
			"getRepr":           func(node ast.Node) string { return ctx.Doc.Repr(node) },
			"isEllipsis":        func(node ast.Node) bool { return hasPrefix(ctx.Doc.Repr(node), "...") },
			"methodResp":        ctx.MethodResponse,
			"methodInner":       ctx.MethodInner,
			"isResponse":        isResponse,
			"isInner":           isInner,
			"httpMethodHasBody": httpMethodHasBody,
			"headerHasBody":     headerHasBody,
		}).
		Parse(apiTemplate)

	if err != nil {
		return err
	}

	return tmpl.Execute(w, ctx)
}
