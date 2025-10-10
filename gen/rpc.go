package gen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"text/template"

	_ "embed"
)

const (
	FeatureRpcNoRt = "rpc/nort"
)

func (builder *CliBuilder) buildRpc(w io.Writer) error {
	inspectCtx, err := builder.inspectRpc()
	if err != nil {
		return fmt.Errorf("inspectRpc(%s, %d): %w", quote(join(builder.pwd, builder.file)), builder.pos, err)
	}
	return inspectCtx.Build(w)
}

type rpcContext struct {
	Package   string
	BuildTags []string
	Ident     string
	Methods   []*Method
	Features  []string
	Doc       Doc
}

func (ctx *rpcContext) Build(w io.Writer) error {
	for _, method := range ctx.Methods {
		if len(method.In) != 1 {
			return fmt.Errorf("rpc method %s should have exactly 1 input parameter", method.Ident)
		}
		if len(method.Out) != 2 {
			return fmt.Errorf("rpc method %s should have exactly 2 output parameters", method.Ident)
		}
		// if !isPointer(method.Out[0]) {
		// 	return fmt.Errorf("rpc method %s should have a pointer as the output parameter", method.Ident)
		// }
		if !checkErrorType(method.Out[1]) {
			return fmt.Errorf("rpc method %s should have an error as the second output parameter", method.Ident)
		}
	}
	if err := ctx.genRpcCode(w); err != nil {
		return fmt.Errorf("genRpcCode: %w", err)
	}
	return nil
}

func (builder *CliBuilder) inspectRpc() (*rpcContext, error) {
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

	rpcFeatures := make([]string, 0, len(builder.feats))
	for _, feature := range builder.feats {
		if hasPrefix(feature, "rpc") {
			rpcFeatures = append(rpcFeatures, feature)
		}
	}

	return &rpcContext{
		Package:   builder.pkg,
		BuildTags: parseBuildTags(builder.doc),
		Ident:     typeSpec.Name.Name,
		Methods:   typeMap(ifaceType.Methods.List, builder.doc.InspectMethod),
		Features:  rpcFeatures,
		Doc:       builder.doc,
	}, nil
}

func (ctx *rpcContext) HasFeature(feature string) bool {
	for _, current := range ctx.Features {
		if current == feature {
			return true
		}
	}
	return false
}

//go:embed template/rpc.tmpl
var rpcTemplate string

func (ctx *rpcContext) genRpcCode(w io.Writer) error {
	tmpl, err := template.
		New("defc(rpc)").
		Funcs(template.FuncMap{
			"isPointer": isPointer,
			"indirect":  indirect,
			"isChan":    isChan,
			"getRepr":   func(node ast.Node) string { return ctx.Doc.Repr(node) },
		}).
		Parse(rpcTemplate)

	if err != nil {
		return err
	}

	return tmpl.Execute(w, ctx)
}
