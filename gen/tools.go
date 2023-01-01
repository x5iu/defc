package gen

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"strconv"
	"strings"
)

func assert(expr bool, msg string) {
	if !expr {
		panic(msg)
	}
}

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
	index      = strings.Index
	cut        = strings.Cut
	contains   = strings.Contains
	join       = path.Join
	isAbs      = path.IsAbs
	read       = os.ReadFile
	list       = os.ReadDir
)

func getPosRepr(src []byte, pos, end token.Pos) string {
	return string(src[pos-1 : end-1])
}

func getRepr(node ast.Node, src []byte) string {
	return getPosRepr(src, node.Pos(), node.End())
}

func hit(fset *token.FileSet, node ast.Node, line int) bool {
	pos, end := fset.Position(node.Pos()), fset.Position(node.End())
	return pos.Line <= line && end.Line >= line
}

func indirect(node ast.Node) ast.Node {
	if ptr, ok := getNode(node).(*ast.StarExpr); ok {
		// hack for compatibility
		return &Expr{
			Expr:   ptr.X,
			Offset: int(node.Pos()),
		}
	}
	return node
}

func isPointer(node ast.Node) bool {
	node = getNode(node)
	_, ok := node.(*ast.StarExpr)
	return ok
}

func isSlice(node ast.Node) bool {
	node = getNode(node)
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

func checkErrorType(node ast.Node) bool {
	node = getNode(node)
	ident, ok := node.(*ast.Ident)
	return ok && ident.Name == ExprErrorIdent
}

func isContextType(ident string, expr ast.Expr, src []byte) bool {
	return ident == "ctx" || contains(getRepr(expr, src), ExprContextIdent)
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

func in[T comparable](list []T, item T) bool {
	for _, ele := range list {
		if ele == item {
			return true
		}
	}
	return false
}

func parseImport(imp string) string {
	elements := splitArgs(imp)
	if len(elements) == 1 {
		pkg := elements[0]
		if hasPrefix(pkg, "\"") && hasSuffix(pkg, "\"") {
			return pkg
		}
		return quote(pkg)
	} else {
		alias, pkg := elements[0], elements[1]
		if hasPrefix(pkg, "\"") && hasSuffix(pkg, "\"") {
			return alias + " " + pkg
		}
		return alias + " " + quote(pkg)
	}
}

var seps = []rune{
	'=',
	':',
}

func cutkv(kv string) (string, string, bool) {
	for _, ch := range kv {
		if in(seps, ch) {
			k, v, ok := cut(kv, string(ch))
			if !ok {
				return kv, "", false
			}
			return trimSpace(k), trimSpace(v), true
		}
	}
	return kv, "", false
}

func getIdent(s string) string {
	if i := index(s, " "); i >= 0 {
		return s[:i]
	}
	return s
}

func parseExpr(input string) (expr ast.Expr, err error) {
	return parser.ParseExpr(input)
}

func getNode(node ast.Node) ast.Node {
	// NOTE: compatible with `defc generate` command
	for {
		if wrapper, ok := node.(interface{ Unwrap() ast.Node }); ok {
			node = wrapper.Unwrap()
			continue
		}
		break
	}
	return node
}

const (
	addBuild = "+build"
	goBuild  = "go:build"
)

func parseBuildTags(src []byte) (tags []string) {
	scanner := bufio.NewScanner(bytes.NewReader(src))
	for scanner.Scan() {
		text := trimSlash(scanner.Text())
		if hasPrefix(text, addBuild) || hasPrefix(text, goBuild) {
			tags = append(tags, text)
		}
	}
	return tags
}
