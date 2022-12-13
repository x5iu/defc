package gen

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path"
	"strconv"
	"strings"
)

const (
	ExprErrorIdent   = "error"
	ExprContextIdent = "Context"
)

var (
	sprintf    = fmt.Sprintf
	quote      = strconv.Quote
	trimPrefix = strings.TrimPrefix
	trimSuffix = strings.TrimSuffix
	trimSpace  = strings.TrimSpace
	hasPrefix  = strings.HasPrefix
	hasSuffix  = strings.HasSuffix
	concat     = strings.Join
	toUpper    = strings.ToUpper
	join       = path.Join
	isAbs      = path.IsAbs
	read       = os.ReadFile
	list       = os.ReadDir
)

func getRepr(node ast.Node, src []byte) string {
	return string(src[node.Pos()-1 : node.End()-1])
}

func newType(expr ast.Expr, src []byte) string {
	switch typ := expr.(type) {
	case *ast.Ident:
		return sprintf("%s{}", typ.Name)
	case *ast.StarExpr:
		return sprintf("new(%s)", getRepr(typ.X, src))
	case *ast.ArrayType:
		return sprintf("make(%s, 0)", getRepr(typ, src))
	case *ast.MapType:
		return sprintf("make(map[%s]%s)", getRepr(typ.Key, src), getRepr(typ.Value, src))
	default:
		return "_"
	}
}

func hit(fset *token.FileSet, node ast.Node, line int) bool {
	pos, end := fset.Position(node.Pos()), fset.Position(node.End())
	return pos.Line <= line && end.Line >= line
}

func indirect(node ast.Node) ast.Node {
	if ptr, ok := node.(*ast.StarExpr); ok {
		return ptr.X
	}
	return node
}

func isPointer(node ast.Node) bool {
	_, ok := node.(*ast.StarExpr)
	return ok
}

func isSlice(node ast.Node) bool {
	typ, ok := node.(*ast.ArrayType)
	return ok && typ.Len == nil
}

func checkInput(method *ast.FuncType) bool {
	for _, param := range method.Params.List {
		if len(param.Names) == 0 {
			return false
		}
	}
	return true
}

func checkErrorType(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == ExprErrorIdent
}

func isContextType(ident string, expr ast.Expr, src []byte) bool {
	return ident == "ctx" || strings.Contains(getRepr(expr, src), ExprContextIdent)
}

func nodeMap[T ast.Node, U any](src []T, f func(ast.Node) U) []U {
	dst := make([]U, len(src))
	for i := 0; i < len(dst); i++ {
		dst[i] = f(src[i])
	}
	return dst
}

func fmtNode(node ast.Node) string {
	if stringer, ok := node.(fmt.Stringer); ok {
		return stringer.String()
	}
	return fmt.Sprintf("%#v", node)
}

func splitArgs(line string) (args []string) {
	var (
		CurlyBraceStack int
		Quoted          bool
		Arg             []byte
	)

	for i := 0; i < len(line); i++ {
		switch ch := line[i]; ch {
		case ' ':
			if Quoted || CurlyBraceStack > 0 {
				Arg = append(Arg, ch)
			} else if len(Arg) > 0 {
				args = append(args, string(Arg))
				Arg = Arg[:0]
			}
		case '"':
			if i > 0 && line[i-1] == '\\' {
				Arg = append(Arg, ch)
			} else {
				Quoted = !Quoted
				Arg = append(Arg, ch)
			}
		case '{':
			if !Quoted {
				CurlyBraceStack++
			}
			Arg = append(Arg, ch)
		case '}':
			if !Quoted {
				CurlyBraceStack--
			}
			Arg = append(Arg, ch)
		default:
			Arg = append(Arg, ch)
		}
	}

	if len(Arg) > 0 {
		args = append(args, string(Arg))
	}

	return args
}

func trimSlash(comment string) string {
	if hasPrefix(comment, "//") {
		comment = trimPrefix(comment, "//")
	} else if hasPrefix(comment, "/*") {
		comment = trimPrefix(comment, "/*")
	}

	if hasSuffix(comment, "*/") {
		comment = trimSuffix(comment, "*/")
	}

	return trimSpace(comment)
}
