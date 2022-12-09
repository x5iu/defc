package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/spf13/cobra"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
	"text/template"

	_ "embed"
)

const (
	SqlxOpExec  = "EXEC"
	SqlxOpQuery = "QUERY"

	SqlxMethodWithTx = "WithTx"

	SqlxCmdInclude = "#INCLUDE"

	FeatureSqlxLog    = "sqlx/log"
	FeatureSqlxRebind = "sqlx/rebind"
)

func genSqlx(_ *cobra.Command, _ []string) error {
	inspectCtx, err := inspectSqlx(join(CurrentDir, CurrentFile), LineNum+1)
	if err != nil {
		return fmt.Errorf("inspectSqlx(%s, %d): %w", quote(join(CurrentDir, CurrentFile)), LineNum, err)
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

		if method.Ident == SqlxMethodWithTx {
			inspectCtx.WithTx = true
			inspectCtx.WithTxContext = method.HasContext()
			inspectCtx.Methods = append(inspectCtx.Methods[:i], inspectCtx.Methods[i+1:]...)
		}
	}

	code, err := genSqlxCode(inspectCtx)
	if err != nil {
		return fmt.Errorf("genApiCode: \n\n%#v\n\n%w", inspectCtx, err)
	}

	fmtCode, err := format.Source(code)
	if err != nil {
		return fmt.Errorf("format.Source: \n\n%s\n\n%w", code, err)
	}

	if output == "" {
		output = "sqlx.go"
	}

	if err = write(join(CurrentDir, output), fmtCode, FileMode); err != nil {
		return fmt.Errorf("os.WriteFile(%s, %04x): %w", join(CurrentDir, output), FileMode, err)
	}

	return nil
}

type SqlxContext struct {
	Package       string
	Ident         string
	Methods       []*Method
	WithTx        bool
	WithTxContext bool
	Features      []string
}

func (ctx *SqlxContext) HasFeature(feature string) bool {
	for _, current := range ctx.Features {
		if current == feature {
			return true
		}
	}
	return false
}

func inspectSqlx(file string, line int) (*SqlxContext, error) {
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
		if name := method.Names[0].Name; name != SqlxMethodWithTx && !checkInput(method.Type.(*ast.FuncType)) {
			return nil, fmt.Errorf(""+
				"input params for method %s should "+
				"contain 'Name' and 'Type' both",
				quote(name))
		}
	}

	sqlxFeatures := make([]string, 0, len(features))
	for _, feature := range features {
		if hasPrefix(feature, "sqlx") {
			sqlxFeatures = append(sqlxFeatures, feature)
		}
	}

	return &SqlxContext{
		Package: PackageName,
		Ident:   typeSpec.Name.Name,
		Methods: nodeMap(ifaceType.Methods.List, func(node ast.Node) *Method {
			return inspectMethod(node, FileContent)
		}),
		Features: sqlxFeatures,
	}, nil
}

func readHeader(header string) (string, error) {
	var buf bytes.Buffer
	scanner := bufio.NewScanner(strings.NewReader(header))
	for scanner.Scan() {
		text := scanner.Text()
		args := splitArgs(text)
		if len(args) == 2 && toUpper(args[0]) == SqlxCmdInclude {
			path := args[1]
			if !isAbs(path) {
				path = join(CurrentDir, path)
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

func hasFeature(feats []string, feat string) bool {
	for _, f := range feats {
		if f == toUpper(feat) {
			return true
		}
	}
	return false
}

//go:embed templates/sqlx.tmpl
var SqlxTemplate string

func genSqlxCode(ctx *SqlxContext) ([]byte, error) {
	tmpl, err := template.
		New("defc(sqlx)").
		Funcs(template.FuncMap{
			"quote":         quote,
			"readHeader":    readHeader,
			"hasFeature":    hasFeature,
			"isSlice":       isSlice,
			"isPointer":     isPointer,
			"indirect":      indirect,
			"isContextType": func(ident string, expr ast.Expr) bool { return isContextType(ident, expr, FileContent) },
			"sub":           func(x, y int) int { return x - y },
			"getRepr":       func(node ast.Node) string { return getRepr(node, FileContent) },
			"isQuery":       func(op string) bool { return op == SqlxOpQuery },
			"isExec":        func(op string) bool { return op == SqlxOpExec },
		}).
		Parse(SqlxTemplate)

	if err != nil {
		return nil, err
	}

	var dst bytes.Buffer
	if err = tmpl.Execute(&dst, ctx); err != nil {
		return nil, err
	}

	return dst.Bytes(), nil
}
