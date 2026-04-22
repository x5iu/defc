package gen

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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
	split      = strings.Split
	concat     = strings.Join
	toUpper    = strings.ToUpper
	index      = strings.Index
	cut        = strings.Cut
	contains   = strings.Contains
	join       = filepath.Join
	isAbs      = filepath.IsAbs
	glob       = filepath.Glob
	read       = os.ReadFile
	lstat      = os.Lstat
	stat       = os.Stat
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
	if ptr, ok := node.(*ast.StarExpr); ok {
		return ptr.X
	}
	return node
}

func deselect(node ast.Node) ast.Node {
	if sel, ok := node.(*ast.SelectorExpr); ok {
		return sel.Sel
	}
	return node
}

func isPointer(node ast.Node) bool {
	_, ok := node.(*ast.StarExpr)
	return ok
}

func isSlice(node ast.Node) bool {
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

func isChan(node ast.Node) bool {
	_, ok := node.(*ast.ChanType)
	return ok
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

// splitArgs tokenises a directive line using a small, custom state machine that
// understands double-quoted, single-quoted, backquoted, `(…)`-grouped and
// `{…}`-grouped tokens. The `${…}` form is treated as a grouping delimiter
// identical to `$(…)` for tokenisation purposes; this is NOT POSIX/bash
// `${VAR}` expansion. No shell is ever involved — tokens are ultimately
// handed to `exec.Command` argv directly (by `runCommand`). See SECURITY.md.
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
	kv = trimSpace(kv)
	for _, ch := range kv {
		if in(seps, ch) {
			k, v, ok := cut(kv, string(ch))
			if !ok {
				return "", "", false
			}
			return trimSpace(k), trimSpace(v), true
		}
	}
	return kv, kv, true
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

// scriptBaselineEnvNames is the always-on names-only allow-list for #SCRIPT
// invocations. Values are copied from the current process env at runtime.
var scriptBaselineEnvNames = []string{
	"PATH", "HOME", "USER",
	"LANG", "LC_ALL", "LC_CTYPE",
	"TMPDIR",
	"GOCACHE", "GOMODCACHE", "GOPATH",
}

// scriptStderrCap bounds the stderr captured from a #SCRIPT invocation so a
// runaway child process can't exhaust generator RAM. Anything beyond this is
// dropped with a trailing truncation marker in the returned error.
const scriptStderrCap = 64 * 1024

// buildScriptEnv constructs the scrubbed process env for a #SCRIPT invocation:
// only names listed in the baseline allow-list plus the caller-supplied extra
// names are passed through, and only when the current process actually has a
// value set for them.
func buildScriptEnv(extra []string) []string {
	seen := make(map[string]struct{}, len(scriptBaselineEnvNames)+len(extra))
	names := make([]string, 0, len(scriptBaselineEnvNames)+len(extra))
	for _, n := range scriptBaselineEnvNames {
		if _, dup := seen[n]; dup {
			continue
		}
		seen[n] = struct{}{}
		names = append(names, n)
	}
	for _, n := range extra {
		if _, dup := seen[n]; dup {
			continue
		}
		seen[n] = struct{}{}
		names = append(names, n)
	}
	env := make([]string, 0, len(names))
	for _, n := range names {
		if v, ok := os.LookupEnv(n); ok {
			env = append(env, n+"="+v)
		}
	}
	return env
}

// runCommand executes argv against a scrubbed environment under a bounded
// timeout. It still recognises `` `…` ``, `$(…)` and `${…}` within its own
// argv as recursive substitutions (preserved for backwards-compatibility with
// existing #SCRIPT users; see SECURITY.md for the deprecation timeline).
// argv[0] must be either a bare command name resolvable via `exec.LookPath`
// or an absolute path; relative paths are rejected.
func runCommand(ctx context.Context, args []string, timeout time.Duration, envAllow []string) (string, error) {
	assert(len(args) > 0, "empty command")
	repl := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if isBackQuoted(arg) {
			if innerArgs := splitArgs(unquote(arg)); len(innerArgs) > 0 {
				innerOutput, err := runCommand(ctx, innerArgs, timeout, envAllow)
				if err != nil {
					return "", err
				}
				repl = append(repl, innerOutput)
			}
		} else if (hasPrefix(arg, "$(") && hasSuffix(arg, ")")) ||
			(hasPrefix(arg, "${") && hasSuffix(arg, "}")) {
			if innerArgs := splitArgs(arg[2 : len(arg)-1]); len(innerArgs) > 0 {
				innerOutput, err := runCommand(ctx, innerArgs, timeout, envAllow)
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

	// Build scrubbed env first so PATH lookup uses only the allow-listed PATH.
	env := buildScriptEnv(envAllow)

	// Resolve argv[0]: absolute path accepted as-is; a path containing a '/'
	// but not absolute is rejected (prevents ./attacker smuggling); bare names
	// are resolved via exec.LookPath against the scrubbed PATH.
	argv0 := repl[0]
	if !filepath.IsAbs(argv0) {
		if strings.ContainsRune(argv0, os.PathSeparator) || strings.ContainsRune(argv0, '/') {
			return "", fmt.Errorf("runCommand: command %q is a relative path; use an absolute path or a bare binary name resolvable via PATH", argv0)
		}
		// Manually resolve argv[0] against the scrubbed PATH without mutating
		// the parent process env (tests and the generator itself run
		// concurrently with other goroutines that may read PATH).
		var scrubbedPath string
		for _, kv := range env {
			if strings.HasPrefix(kv, "PATH=") {
				scrubbedPath = kv[len("PATH="):]
				break
			}
		}
		resolved, lookErr := lookPathIn(argv0, scrubbedPath)
		if lookErr != nil {
			return "", fmt.Errorf("runCommand: command %q not found in PATH: %w", argv0, lookErr)
		}
		argv0 = resolved
	}

	// Apply timeout via a derived context. A zero or negative timeout means
	// "no runCommand-internal timeout" — parent ctx still governs.
	cmdCtx := ctx
	if cmdCtx == nil {
		cmdCtx = context.Background()
	}
	var cancel context.CancelFunc
	if timeout > 0 {
		cmdCtx, cancel = context.WithTimeout(cmdCtx, timeout)
		defer cancel()
	}

	var (
		output    bytes.Buffer
		stderrBuf bytes.Buffer
	)
	command := exec.CommandContext(cmdCtx, argv0, repl[1:]...)
	command.Env = env
	command.Stdin = nil
	command.Stdout = &output
	command.Stderr = &boundedWriter{buf: &stderrBuf, cap: scriptStderrCap}
	if err := command.Run(); err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("runCommand: command %q exceeded %s timeout", argv0, timeout)
		}
		stderrOut := stderrBuf.String()
		if stderrOut != "" {
			return "", fmt.Errorf("runCommand: %q exited non-zero: %w\nstderr: %s", argv0, err, stderrOut)
		}
		return "", fmt.Errorf("runCommand: %q exited non-zero: %w", argv0, err)
	}
	return trimSpace(output.String()), nil
}

// lookPathIn resolves `name` against the supplied PATH string (colon-separated
// on Unix, ';'-separated on Windows). Unlike exec.LookPath it does not consult
// process-wide env, which makes it safe to call concurrently without mutating
// os.Environ.
func lookPathIn(name, path string) (string, error) {
	if strings.ContainsRune(name, os.PathSeparator) {
		// Should have been caught by the absolute-path branch; defensive.
		if _, err := os.Stat(name); err != nil {
			return "", err
		}
		return name, nil
	}
	sep := byte(':')
	if os.PathSeparator == '\\' {
		sep = ';'
	}
	if path == "" {
		return "", fmt.Errorf("executable file %q not found in empty PATH", name)
	}
	for _, dir := range strings.Split(path, string(sep)) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, name)
		fi, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		if fi.Mode().IsRegular() && fi.Mode()&0111 != 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("executable file %q not found in PATH", name)
}

// boundedWriter is an io.Writer that captures at most `cap` bytes, discarding
// the tail and appending a truncation marker when the limit is exceeded.
type boundedWriter struct {
	buf       *bytes.Buffer
	cap       int
	truncated bool
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	if w.buf.Len() >= w.cap {
		if !w.truncated {
			w.buf.WriteString("\n… (truncated)")
			w.truncated = true
		}
		return len(p), nil
	}
	remaining := w.cap - w.buf.Len()
	if remaining >= len(p) {
		return w.buf.Write(p)
	}
	w.buf.Write(p[:remaining])
	if !w.truncated {
		w.buf.WriteString("\n… (truncated)")
		w.truncated = true
	}
	return len(p), nil
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
						if maybeRpcDecl(ifaceType) {
							return f.Name.String(), ModeRpc, fset.Position(typeSpec.Pos()).Line - 1, nil
						}
					}
				}
			}
		}
	}
	return "", 0, 0, ErrNoTargetDeclFound
}

func maybeRpcDecl(iface *ast.InterfaceType) bool {
	for _, field := range iface.Methods.List {
		if funcType, ok := field.Type.(*ast.FuncType); ok {
			if len(funcType.Params.List) != 1 {
				return false
			}
			if len(funcType.Results.List) != 2 {
				return false
			}
			if !checkErrorType(funcType.Results.List[1].Type) {
				return false
			}
		}
	}
	return true
}
