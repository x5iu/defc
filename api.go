package main

import (
	"bytes"
	"fmt"
	"github.com/spf13/cobra"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"net/http"
	"sort"
	"strings"
	"text/template"

	_ "embed"
)

const (
	ApiMethodInner    = "Inner"
	ApiMethodResponse = "Response"

	FeatureApiCache  = "api/cache"
	FeatureApiLog    = "api/log"
	FeatureApiClient = "api/client"
)

func genApi(_ *cobra.Command, _ []string) error {
	inspectCtx, err := inspectApi(join(CurrentDir, CurrentFile), LineNum+1)
	if err != nil {
		return fmt.Errorf("inspectApi(%s, %d): %w", quote(join(CurrentDir, CurrentFile)), LineNum, err)
	}

	if !checkResponse(inspectCtx.Methods) {
		return fmt.Errorf("checkResponse: no '%s() T' method found in Interface", ApiMethodResponse)
	}

	for _, method := range inspectCtx.Methods {
		if method.Ident != ApiMethodResponse && method.Ident != ApiMethodInner {
			if l := len(method.Out); l == 0 || !checkErrorType(method.Out[l-1]) {
				return fmt.Errorf("checkErrorType: no 'error' found in method %s returned value",
					quote(method.Ident))
			}
		}

		if (method.Ident == ApiMethodResponse || method.Ident == ApiMethodInner) &&
			(len(method.In) != 0 || len(method.Out) != 1) {
			return fmt.Errorf(
				"%s method can only have no income params "+
					"and 1 returned value", quote(method.Ident))
		}

		if method.Ident == ApiMethodResponse {
			if !checkResponseType(method) {
				return fmt.Errorf(
					"checkResponseType: returned type of %s "+
						"should be kind of *ast.Ident or *ast.StarExpr",
					quote(ApiMethodResponse))
			}
		}

		if len(method.Out) > 2 {
			return fmt.Errorf("%s method expects 2 returned value at most, got %d",
				quote(method.Ident),
				len(method.Out))
		}
	}

	code, err := genApiCode(inspectCtx)
	if err != nil {
		return fmt.Errorf("genApiCode: \n\n%#v\n\n%w", inspectCtx, err)
	}

	fmtCode, err := format.Source(code)
	if err != nil {
		return fmt.Errorf("format.Source: \n\n%s\n\n%w", code, err)
	}

	if output == "" {
		output = "api.go"
	}

	if err = write(join(CurrentDir, output), fmtCode, FileMode); err != nil {
		return fmt.Errorf("os.WriteFile(%s, %04x): %w", join(CurrentDir, output), FileMode, err)
	}

	return nil
}

type ApiContext struct {
	Package  string
	Ident    string
	Generics map[string]ast.Expr
	Methods  []*Method
	Features []string
}

func (ctx *ApiContext) SortGenerics() []string {
	idents := make([]string, 0, len(ctx.Generics))
	for k := range ctx.Generics {
		idents = append(idents, k)
	}
	sort.Slice(idents, func(i, j int) bool {
		return ctx.Generics[idents[i]].Pos() < ctx.Generics[idents[j]].Pos()
	})
	return idents
}

func (ctx *ApiContext) GenericsRepr(withType bool) string {
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
			dst.WriteString(getRepr(expr, FileContent))
		}
		if index < len(ctx.Generics)-1 {
			dst.WriteString(", ")
		}
	}
	dst.WriteByte(']')

	return dst.String()
}

func (ctx *ApiContext) HasFeature(feature string) bool {
	for _, current := range ctx.Features {
		if current == feature {
			return true
		}
	}
	return false
}

func (ctx *ApiContext) HasHeader() bool {
	for _, method := range ctx.Methods {
		if method.Header != "" {
			return true
		}
	}
	return false
}

func (ctx *ApiContext) HasInner() bool {
	return hasInner(ctx.Methods)
}

func (ctx *ApiContext) InnerType() ast.Node {
	for _, method := range ctx.Methods {
		if method.Ident == ApiMethodInner {
			return method.Out[0]
		}
	}
	return nil
}

func inspectApi(file string, line int) (*ApiContext, error) {
	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, file, FileContent, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var (
		genDecl   *ast.GenDecl
		typeSpec  *ast.TypeSpec
		ifaceType *ast.InterfaceType
	)

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

	apiFeatures := make([]string, 0, len(features))
	for _, feature := range features {
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

	return &ApiContext{
		Package:  PackageName,
		Ident:    typeSpec.Name.Name,
		Generics: generics,
		Methods: nodeMap(ifaceType.Methods.List, func(node ast.Node) *Method {
			return inspectMethod(node, FileContent)
		}),
		Features: apiFeatures,
	}, nil
}

func checkResponse(methods []*Method) bool {
	for _, method := range methods {
		if method.Ident == ApiMethodResponse {
			return true
		}
	}
	return false
}

func checkResponseType(method *Method) bool {
	switch method.Out[0].(type) {
	case *ast.Ident, *ast.StarExpr:
		return true
	default:
		return false
	}
}

func hasInner(methods []*Method) bool {
	for _, method := range methods {
		if method.Ident == ApiMethodInner {
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

//go:embed templates/api.tmpl
var ApiTemplate string

func genApiCode(ctx *ApiContext) ([]byte, error) {
	tmpl, err := template.
		New("defc(api)").
		Funcs(template.FuncMap{
			"quote":         quote,
			"isPointer":     isPointer,
			"indirect":      indirect,
			"importContext": importContext,
			"sub": func(x, y int) int {
				return x - y
			},
			"getRepr": func(node ast.Node) string {
				return getRepr(node, FileContent)
			},
			"methodResp": func() string {
				return ApiMethodResponse
			},
			"isResponse": func(ident string) bool {
				return ident == ApiMethodResponse
			},
			"isInner": func(ident string) bool {
				return ident == ApiMethodInner
			},
			"newType": func(expr ast.Expr) string {
				return newType(expr, FileContent)
			},
			"httpMethodHasBody": func(method string) bool {
				switch method {
				case http.MethodGet:
					return false
				case http.MethodPost, http.MethodPut:
					return true
				default:
					return false
				}
			},
			"headerHasBody": func(header string) bool {
				if index := strings.Index(header, "\r\n\r\n"); index != -1 {
					return len(header[index+4:]) > 0
				}
				return false
			},
		}).
		Parse(ApiTemplate)

	if err != nil {
		return nil, err
	}

	var dst bytes.Buffer
	if err = tmpl.Execute(&dst, ctx); err != nil {
		return nil, err
	}

	return dst.Bytes(), nil
}
