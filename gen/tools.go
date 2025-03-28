package gen

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	errorf     = fmt.Errorf
	quote      = strconv.Quote
	trimPrefix = strings.TrimPrefix
	trimSuffix = strings.TrimSuffix
	trimSpace  = strings.TrimSpace
	hasPrefix  = strings.HasPrefix
	hasSuffix  = strings.HasSuffix
	split      = strings.Split
	concat     = strings.Join
	toUpper    = strings.ToUpper
	index      = strings.Index
	cut        = strings.Cut
	contains   = strings.Contains
	join       = filepath.Join
	isAbs      = filepath.IsAbs
	glob       = filepath.Glob
	base       = filepath.Base
	stat       = os.Stat
	read       = os.ReadFile
	list       = os.ReadDir
)

func getPosRepr(src []byte, pos, end token.Pos) string {
	return string(src[pos-1 : end-1])
}

func getRepr(node ast.Node, src []byte) string {
	return getPosRepr(src, node.Pos(), node.End())
}

func surroundLine(fset *token.FileSet, node ast.Node, line int) bool {
	pos, end := fset.Position(node.Pos()), fset.Position(node.End())
	return pos.Line <= line && end.Line >= line
}

func afterLine(fset *token.FileSet, node ast.Node, line int) bool {
	_, end := fset.Position(node.Pos()), fset.Position(node.End())
	return end.Line >= line
}

func indirect(node ast.Node) ast.Node {
	if ptr, ok := getNode(node).(*ast.StarExpr); ok {
		// hack for compatibility
		return &Expr{
			Expr: ptr.X,
			// `ptr` may come from parser.ParseExpr, so we calculate offset by field offsets
			Offset: int(node.Pos() + (ptr.X.Pos() - ptr.Star) - 1),
		}
	}
	return node
}

func deselect(node ast.Node) ast.Node {
	if sel, ok := getNode(node).(*ast.SelectorExpr); ok {
		// hack for compatibility
		return &Expr{
			Expr: sel.Sel,
			// `sel` may come from parser.ParseExpr, so we calculate offset by field offsets
			Offset: int(node.Pos() + (sel.Sel.Pos() - sel.X.Pos()) - 1),
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
	if !ok {
		return false
	}

	// []byte is a special slice, equivalent to type string
	eltIsByte := false
	if elt, ok := typ.Elt.(*ast.Ident); ok {
		eltIsByte = elt.Name == "byte"
	}

	return typ.Len == nil && !eltIsByte
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

func typeMap[T any, U any](src []T, f func(T) U) []U {
	dst := make([]U, len(src))
	for i := 0; i < len(dst); i++ {
		dst[i] = f(src[i])
	}
	return dst
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
	line = trimSpace(line)
	if len(line) == 0 {
		return nil
	}

	var (
		parenthesisStack int
		curlyBraceStack  int
		doubleQuoted     bool
		singleQuoted     bool
		backQuoted       bool
		arg              []byte
	)

	for i := 0; i < len(line); i++ {
		switch ch := line[i]; ch {
		case ' ', '\t', '\n', '\r':
			if doubleQuoted || singleQuoted || backQuoted ||
				parenthesisStack > 0 || curlyBraceStack > 0 {
				arg = append(arg, ch)
			} else if len(arg) > 0 {
				args = append(args, string(arg))
				arg = arg[:0]
			}
		case '"':
			if (i > 0 && line[i-1] == '\\') || singleQuoted || backQuoted {
				arg = append(arg, ch)
			} else {
				doubleQuoted = !doubleQuoted
				arg = append(arg, ch)
			}
		case '\'':
			if (i > 0 && line[i-1] == '\\') || doubleQuoted || backQuoted {
				arg = append(arg, ch)
			} else {
				singleQuoted = !singleQuoted
				arg = append(arg, ch)
			}
		case '`':
			if (i > 0 && line[i-1] == '\\') || doubleQuoted || singleQuoted {
				arg = append(arg, ch)
			} else {
				backQuoted = !backQuoted
				arg = append(arg, ch)
			}
		case '(':
			if !(doubleQuoted || singleQuoted || backQuoted) {
				parenthesisStack++
			}
			arg = append(arg, ch)
		case ')':
			if !(doubleQuoted || singleQuoted || backQuoted) {
				parenthesisStack--
			}
			arg = append(arg, ch)
		case '{':
			if !(doubleQuoted || singleQuoted || backQuoted) {
				curlyBraceStack++
			}
			arg = append(arg, ch)
		case '}':
			if !(doubleQuoted || singleQuoted || backQuoted) {
				curlyBraceStack--
			}
			arg = append(arg, ch)
		default:
			arg = append(arg, ch)
		}
	}

	if len(arg) > 0 {
		args = append(args, string(arg))
	}

	return args
}

func trimSlash(comment string) string {
	if hasPrefix(comment, "//") {
		comment = trimPrefix(comment, "//")
	} else if hasPrefix(comment, "/*") {
		comment = trimPrefix(comment, "/*")
		if hasSuffix(comment, "*/") {
			comment = trimSuffix(comment, "*/")
		}
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

// parseBuildTags uses source []byte instead of ast.CommentGroup to parse build tags,
// since parser.ParseFile removes commands like "//go:build" or "//go:generate", we
// can't get build tags from ast.CommentGroup.
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

func unquote(quoted string) (unquoted string) {
	if len(quoted) == 0 {
		return ""
	}
	if (hasPrefix(quoted, "\"") && hasSuffix(quoted, "\"")) ||
		(hasPrefix(quoted, "'") && hasSuffix(quoted, "'")) ||
		isBackQuoted(quoted) {
		return quoted[1 : len(quoted)-1]
	}
	return quoted
}

func isBackQuoted(s string) bool {
	return hasPrefix(s, "`") && hasSuffix(s, "`")
}

func runCommand(args []string) (string, error) {
	assert(len(args) > 0, "empty command")
	repl := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if isBackQuoted(arg) {
			if innerArgs := splitArgs(unquote(arg)); len(innerArgs) > 0 {
				innerOutput, err := runCommand(innerArgs)
				if err != nil {
					return "", err
				}
				repl = append(repl, innerOutput)
			}
		} else if (hasPrefix(arg, "$(") && hasSuffix(arg, ")")) ||
			(hasPrefix(arg, "${") && hasSuffix(arg, "}")) {
			if innerArgs := splitArgs(arg[2 : len(arg)-1]); len(innerArgs) > 0 {
				innerOutput, err := runCommand(innerArgs)
				if err != nil {
					return "", err
				}
				repl = append(repl, innerOutput)
			}
		} else {
			repl = append(repl, unquote(arg))
		}
	}
	if len(repl) == 0 {
		return "", nil
	}
	var output bytes.Buffer
	command := exec.Command(repl[0], repl[1:]...)
	command.Stdout = &output
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return "", err
	}
	return trimSpace(output.String()), nil
}

type Import struct {
	Name string
	Path string
}

func getImports(pkg string, dir string, name string, isIt func(ast.Node) bool) (imports []*Import, err error) {
	imports = make([]*Import, 0, 8)

	var (
		fset   = token.NewFileSet()
		files  = make([]*ast.File, 0, 8)
		target *ast.File
	)

	filenames, err := glob(join(dir, "*.go"))
	if err != nil {
		return nil, err
	}

	for _, filename := range filenames {
		file, err := parser.ParseFile(fset, filename, nil, 0)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
		if base(name) == base(filename) {
			target = file
		}
	}

	conf := types.Config{
		IgnoreFuncBodies: true,
		FakeImportC:      true,
		Importer: &Importer{
			imported:      map[string]*types.Package{},
			tokenFileSet:  fset,
			defaultImport: importer.Default(),
		},
	}

	info := &types.Info{
		Types:      map[ast.Expr]types.TypeAndValue{},
		Instances:  map[*ast.Ident]types.Instance{},
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Implicits:  map[ast.Node]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
		Scopes:     map[ast.Node]*types.Scope{},
		InitOrder:  []*types.Initializer{},
	}

	_, err = conf.Check(pkg, fset, files, info)
	if err != nil {
		return nil, err
	}

	if target != nil {
		ast.Inspect(target, func(x ast.Node) bool {
			if x != nil && isIt(x) {
				ast.Inspect(x, func(n ast.Node) bool {
					switch node := n.(type) {
					case ast.Expr:
						if named, ok := info.TypeOf(node).(*types.Named); ok {
							if objPkg := named.Obj().Pkg(); objPkg != nil {
								imports = append(imports, &Import{
									Name: objPkg.Name(),
									Path: objPkg.Path(),
								})
							}
						}
					}
					return true
				})
			}
			return true
		})
	}

	return imports, nil
}

type Importer struct {
	imported      map[string]*types.Package
	tokenFileSet  *token.FileSet
	defaultImport types.Importer
}

var importing types.Package

func (importer *Importer) ImportFrom(path, dir string, _ types.ImportMode) (*types.Package, error) {
	if path == "unsafe" {
		return types.Unsafe, nil
	}
	if path == "C" {
		return nil, errorf("unreachable: %s", "import \"C\"")
	}
	goroot := join(build.Default.GOROOT, "src")
	if _, err := stat(join(goroot, path)); err != nil {
		if os.IsNotExist(err) {
			target := importer.imported[path]
			if target != nil {
				if target == &importing {
					return nil, errors.New("cycle importing " + path)
				}
				return target, nil
			}
			importer.imported[path] = &importing
			pkg, err := build.Import(path, dir, 0)
			if err != nil {
				return nil, err
			}
			var files []*ast.File
			for _, name := range append(pkg.GoFiles, pkg.CgoFiles...) {
				name = join(pkg.Dir, name)
				file, err := parser.ParseFile(importer.tokenFileSet, name, nil, 0)
				if err != nil {
					return nil, err
				}
				files = append(files, file)
			}
			conf := types.Config{
				Importer:         importer,
				FakeImportC:      true,
				IgnoreFuncBodies: true,
			}
			target, err = conf.Check(path, importer.tokenFileSet, files, nil)
			if err != nil {
				return nil, err
			}
			importer.imported[path] = target
			return target, nil
		}
	}
	if importerFrom, ok := importer.defaultImport.(types.ImporterFrom); ok {
		return importerFrom.ImportFrom(path, dir, 0)
	}
	return importer.defaultImport.Import(path)
}

func (importer *Importer) Import(path string) (*types.Package, error) {
	return importer.ImportFrom(path, "", 0)
}

var ErrNoTargetDeclFound = errors.New("no target decl found")

func DetectTargetDecl(file string, src []byte, target string) (string, Mode, int, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, parser.ParseComments)
	if err != nil {
		return "", 0, 0, err
	}
	for _, decl := range f.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if target != "" && typeSpec.Name.String() != target {
						continue
					}
					if ifaceType, ok := typeSpec.Type.(*ast.InterfaceType); ok && ifaceType.Methods != nil {
						for _, field := range ifaceType.Methods.List {
							if _, ok := field.Type.(*ast.FuncType); ok {
								if field.Doc != nil && len(field.Doc.List) > 0 {
									firstLine := field.Doc.List[0]
									firstLineArgs := splitArgs(trimSlash(firstLine.Text))
									if len(firstLineArgs) > 1 {
										switch opArg := firstLineArgs[1]; toUpper(opArg) {
										case sqlxOpExec, sqlxOpQuery:
											return f.Name.String(), ModeSqlx, fset.Position(typeSpec.Pos()).Line - 1, nil
										case http.MethodGet,
											http.MethodHead,
											http.MethodPost,
											http.MethodPut,
											http.MethodPatch,
											http.MethodDelete,
											http.MethodConnect,
											http.MethodOptions,
											http.MethodTrace:
											return f.Name.String(), ModeApi, fset.Position(typeSpec.Pos()).Line - 1, nil
										}
									}
								}
								if len(field.Names) > 0 {
									if funcName := field.Names[0]; funcName.String() == sqlxMethodWithTx {
										return f.Name.String(), ModeSqlx, fset.Position(typeSpec.Pos()).Line - 1, nil
									} else if funcNameString := funcName.String(); isInner(funcNameString) || isResponse(funcNameString) {
										return f.Name.String(), ModeApi, fset.Position(typeSpec.Pos()).Line - 1, nil
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return "", 0, 0, ErrNoTargetDeclFound
}
