package gen

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	_ "embed"
)

const (
	sqlxOpExec  = "EXEC"
	sqlxOpQuery = "QUERY"

	sqlxMethodWithTx = "WithTx"

	// sqlxCmdInclude expands to the contents of one or more files under the
	// schema directory (or a configured --include-root). Symlinks are
	// rejected and per-file / aggregate size caps apply. See SECURITY.md.
	//
	// sqlxCmdScript executes an external command at generate time. It is
	// gated behind --allow-script and is DEPRECATED for removal in the next
	// minor release; see SECURITY.md for the migration path.
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
	Package         string
	BuildTags       []string
	Ident           string
	Methods         []*Method
	Embeds          []ast.Expr
	WithTx          bool
	WithTxType      ast.Expr
	WithTxContext   bool
	WithTxIsolation string
	Features        []string
	Imports         []string
	Funcs           []string
	Pwd             string
	HeaderCfg       *readHeaderConfig
	Doc             Doc
	Template        string
}

func (ctx *sqlxContext) Build(w io.Writer) error {
	const (
		constbindOption = "CONSTBIND"
		bindOption      = "BIND"
	)

	var fixedMethods []*Method = nil
	for i, method := range ctx.Methods {
		if l := len(method.Out); l == 0 || !checkErrorType(method.Out[l-1]) {
			return fmt.Errorf("checkErrorType: no 'error' found in method %s returned values",
				quote(method.Ident))
		}

		// Check for conflicting CONSTBIND and BIND options
		opts := method.SqlxOptions()
		if hasOption(opts, constbindOption) && hasOption(opts, bindOption) {
			return fmt.Errorf("method %s: CONSTBIND and BIND options are mutually exclusive, please use only one of them",
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
			txType, err := method.TxType()
			if err != nil {
				return err
			}
			ctx.WithTx = true
			ctx.WithTxType = txType
			ctx.WithTxContext = method.HasContext()
			ctx.WithTxIsolation = method.TxIsolationLv()
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

	var bindInvoked bool
	// Since the text/template standard library does not provide a specific error type, we can only determine whether
	// the bind function has been invoked in the template through this rudimentary way.
	if _, err := template.New("detect_bind_function").Parse(ctx.Template); err != nil {
		bindInvoked = contains(err.Error(), `function "bind" not defined`)
	}

	// Small hack: When the --template/-t option is enabled, and "bind" function has been invoked, the Bind option
	// is enabled by default for all methods.
	if ctx.Template != "" && bindInvoked {
		const (
			bindOption  = "BIND"
			namedOption = "NAMED"
		)
		// [2024-05-07]
		// Eventually, it was realized that arbitrarily adding a Bind option to each method was a foolish act.
		// Bind would require parsing the template content every time the method is called, which is very slow.
		// In some scenarios, there is simply a need for some common templates without wanting this heavy burden.
		// Therefore, today we will disable this unwise behavior.
		//
		// [2024-05-11]
		// When the situation becomes that one method includes a Bind option, but other methods do not include a
		// Bind option, the best strategy should be to add a Bind option to all methods. This is because the
		// template may contain calls to bind, and if you do not add a Bind option for the method, it will cause
		// an error in rendering the template.
		var useBind bool
		for _, method := range ctx.Methods {
			if hasOption(method.SqlxOptions(), bindOption) {
				useBind = true
				break
			}
		}
		if useBind {
			for _, method := range ctx.Methods {
				if !hasOption(method.SqlxOptions(), bindOption) && !hasOption(method.SqlxOptions(), namedOption) {
					method.Meta += " " + bindOption
				}
			}
		}
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
		imports = append(imports, quote("github.com/x5iu/defc/sqlx"))
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
			imports = append(imports, parseImport("__rt github.com/x5iu/defc/runtime"))
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
		if name := method.Names[0].Name; name != sqlxMethodWithTx {
			if funcType, ok := method.Type.(*ast.FuncType); ok && !checkInput(funcType) {
				return nil, fmt.Errorf(""+
					"input params for method %s should "+
					"contain 'Name' and 'Type' both",
					quote(name))
			}
		}
	}

	sqlxFeatures := make([]string, 0, len(builder.feats))
	for _, feature := range builder.feats {
		if hasPrefix(feature, "sqlx") {
			sqlxFeatures = append(sqlxFeatures, feature)
		}
	}

	schemaDir := builder.pwd
	if builder.file != "" {
		if abs := builder.file; filepath.IsAbs(abs) {
			schemaDir = filepath.Dir(abs)
		} else {
			schemaDir = filepath.Dir(filepath.Join(builder.pwd, abs))
		}
	}
	headerCfg := &readHeaderConfig{
		schemaDir:     schemaDir,
		includeRoots:  append([]string(nil), builder.includeRoots...),
		allowScript:   builder.allowScript,
		scriptTimeout: builder.scriptTimeout,
		scriptEnv:     append([]string(nil), builder.scriptEnv...),
		directiveFile: builder.file,
		lineOffset:    builder.pos,
	}

	return &sqlxContext{
		Package:   builder.pkg,
		BuildTags: parseBuildTags(builder.doc),
		Ident:     typeSpec.Name.Name,
		Methods:   typeMap(methods, builder.doc.InspectMethod),
		Embeds:    embeds,
		Features:  sqlxFeatures,
		Imports:   builder.imports,
		Funcs:     builder.funcs,
		Pwd:       builder.pwd,
		HeaderCfg: headerCfg,
		Doc:       builder.doc,
		Template:  builder.template,
	}, nil
}

// readHeaderConfig carries the policy knobs readHeader needs to evaluate
// `#INCLUDE` / `#SCRIPT` directives safely. Anchors `#INCLUDE` to the schema
// file's own directory plus any caller-supplied includeRoots; gates `#SCRIPT`
// behind allowScript; bounds `#SCRIPT` execution via scriptTimeout; scrubs
// the child env down to scriptEnv plus the runCommand baseline allow-list;
// and carries directiveFile / lineOffset so diagnostics can report `file:line`
// pointing at the offending directive.
type readHeaderConfig struct {
	schemaDir     string
	includeRoots  []string
	allowScript   bool
	scriptTimeout time.Duration
	scriptEnv     []string
	directiveFile string
	lineOffset    int
}

const (
	includePerFileCap   = 1 * 1024 * 1024 // 1 MiB
	includeAggregateCap = 4 * 1024 * 1024 // 4 MiB
)

// isPathUnder reports whether child, after cleaning, is lexically equal to or
// a descendant of parent (also cleaned). Both arguments must be absolute.
func isPathUnder(child, parent string) bool {
	c := filepath.Clean(child)
	p := filepath.Clean(parent)
	if c == p {
		return true
	}
	rel, err := filepath.Rel(p, c)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false
	}
	return !filepath.IsAbs(rel)
}

// rejectSymlinkInPath walks every component from `anchor` (inclusive) down to
// `target` (inclusive) and reports an error if any is a symlink. Both inputs
// must be clean absolute paths; `target` must lie under `anchor`.
func rejectSymlinkInPath(anchor, target string) error {
	anchor = filepath.Clean(anchor)
	target = filepath.Clean(target)
	rel, err := filepath.Rel(anchor, target)
	if err != nil {
		return err
	}
	current := anchor
	// Check the anchor itself.
	if fi, err := lstat(current); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink not allowed at %q", current)
		}
	}
	if rel == "." {
		return nil
	}
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		fi, err := lstat(current)
		if err != nil {
			return err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink not allowed at %q", current)
		}
	}
	return nil
}

// resolveIncludePath anchors a user-supplied #INCLUDE path (which may be a
// glob pattern) to the schema directory or one of the explicit include roots,
// rejecting anything that escapes. The returned path is a clean absolute
// pattern suitable for filepath.Glob; the returned anchor is the root the
// pattern resolved under (used for per-component symlink rejection).
func resolveIncludePath(rawPath string, cfg *readHeaderConfig) (pattern, anchor string, err error) {
	var resolved string
	if filepath.IsAbs(rawPath) {
		resolved = filepath.Clean(rawPath)
		for _, root := range cfg.includeRoots {
			if isPathUnder(resolved, root) {
				return resolved, filepath.Clean(root), nil
			}
		}
		return "", "", fmt.Errorf("uses an absolute path that does not lie under any --include-root")
	}
	resolved = filepath.Clean(filepath.Join(cfg.schemaDir, rawPath))
	if isPathUnder(resolved, cfg.schemaDir) {
		return resolved, filepath.Clean(cfg.schemaDir), nil
	}
	for _, root := range cfg.includeRoots {
		if isPathUnder(resolved, root) {
			return resolved, filepath.Clean(root), nil
		}
	}
	return "", "", fmt.Errorf("resolves to %q which is outside the schema directory and all --include-root entries", resolved)
}

func readHeader(header string, cfg *readHeaderConfig) (string, error) {
	if cfg == nil {
		cfg = &readHeaderConfig{}
	}
	var (
		buf           bytes.Buffer
		scanner       = bufio.NewScanner(strings.NewReader(header))
		text          string
		lineNo        int // 1-based line within `header`
		currentLineNo int // line on which the current directive began
		includedTotal int64
	)

	for {
		if text == "" {
			if !scanner.Scan() {
				break
			}
			lineNo++
			text = scanner.Text()
			currentLineNo = lineNo
		}

		var (
			next       string
			nextLineNo int
			haveNext   bool
		)
		for {
			if !scanner.Scan() {
				break
			}
			lineNo++
			candidate := scanner.Text()
			if len(candidate) > 0 && (candidate[0] == ' ' || candidate[0] == '\t') {
				text += " " + trimSpace(candidate)
			} else {
				next = candidate
				nextLineNo = lineNo
				haveNext = true
				break
			}
		}

		text = trimSpace(text)
		args := splitArgs(text)
		directiveLine := cfg.lineOffset + currentLineNo

		if len(args) == 2 && toUpper(args[0]) == sqlxCmdInclude {
			rawPath := unquote(args[1])
			pattern, anchor, resolveErr := resolveIncludePath(rawPath, cfg)
			if resolveErr != nil {
				return "", fmt.Errorf("#INCLUDE %q at %s:%d %s",
					rawPath, cfg.directiveFile, directiveLine, resolveErr.Error())
			}
			matches, err := glob(pattern)
			if err != nil {
				return "", fmt.Errorf("#INCLUDE %q at %s:%d: filepath.Glob(%q): %w",
					rawPath, cfg.directiveFile, directiveLine, pattern, err)
			}
			if len(matches) == 0 {
				return "", fmt.Errorf("#INCLUDE %q at %s:%d matched no files",
					rawPath, cfg.directiveFile, directiveLine)
			}
			sort.Strings(matches)
			for _, match := range matches {
				if !filepath.IsAbs(match) {
					match = filepath.Clean(filepath.Join(cfg.schemaDir, match))
				} else {
					match = filepath.Clean(match)
				}
				if err := rejectSymlinkInPath(anchor, match); err != nil {
					return "", fmt.Errorf("#INCLUDE %q at %s:%d: %s",
						rawPath, cfg.directiveFile, directiveLine, err.Error())
				}
				fi, err := stat(match)
				if err != nil {
					return "", fmt.Errorf("#INCLUDE %q at %s:%d: os.Stat(%q): %w",
						rawPath, cfg.directiveFile, directiveLine, match, err)
				}
				if fi.IsDir() {
					continue
				}
				if fi.Size() > includePerFileCap {
					return "", fmt.Errorf("#INCLUDE %q at %s:%d: file %q exceeds 1 MiB per-file limit (%d bytes)",
						rawPath, cfg.directiveFile, directiveLine, match, fi.Size())
				}
				if includedTotal+fi.Size() > includeAggregateCap {
					return "", fmt.Errorf("#INCLUDE %q at %s:%d: total included bytes exceed 4 MiB aggregate limit",
						rawPath, cfg.directiveFile, directiveLine)
				}
				content, err := read(match)
				if err != nil {
					return "", fmt.Errorf("#INCLUDE %q at %s:%d: os.ReadFile(%q): %w",
						rawPath, cfg.directiveFile, directiveLine, match, err)
				}
				includedTotal += int64(len(content))
				buf.Write(content)
			}
		} else if len(args) > 1 && toUpper(args[0]) == sqlxCmdScript {
			if !cfg.allowScript {
				return "", fmt.Errorf("#SCRIPT directive at %s:%d is disabled by default; re-run with --allow-script if you understand the security implications (see SECURITY.md)",
					cfg.directiveFile, directiveLine)
			}
			timeout := cfg.scriptTimeout
			if timeout <= 0 {
				timeout = 10 * time.Second
			}
			output, err := runCommand(context.Background(), args[1:], timeout, cfg.scriptEnv)
			if err != nil {
				return "", fmt.Errorf("#SCRIPT at %s:%d: %w",
					cfg.directiveFile, directiveLine, err)
			}
			fmt.Fprintf(os.Stderr, "defc: warning: #SCRIPT at %s:%d is deprecated and will be removed in a future release; see SECURITY.md\n",
				cfg.directiveFile, directiveLine)
			buf.WriteString(output)
		} else {
			buf.WriteString(text)
		}
		buf.WriteString("\r\n")

		if haveNext {
			text = next
			currentLineNo = nextLineNo
		} else {
			text = ""
			break
		}
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

//go:embed template/sqlx.tmpl
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
			"readHeader":    func(header string) (string, error) { return readHeader(header, ctx.HeaderCfg) },
			"isContextType": func(ident string, expr ast.Expr) bool { return ctx.Doc.IsContextType(ident, expr) },
			"sub":           func(x, y int) int { return x - y },
			"getRepr":       func(node ast.Node) string { return ctx.Doc.Repr(node) },
			"isQuery":       func(op string) bool { return op == sqlxOpQuery },
			"isExec":        func(op string) bool { return op == sqlxOpExec },
			"constBindSQL": func(header string) (string, error) {
				processed, err := readHeader(header, ctx.HeaderCfg)
				if err != nil {
					return "", err
				}
				result, err := parseConstBindExpressions(processed)
				if err != nil {
					return "", err
				}
				return result.SQL, nil
			},
			"constBindArgs": func(header string) ([]string, error) {
				processed, err := readHeader(header, ctx.HeaderCfg)
				if err != nil {
					return nil, err
				}
				result, err := parseConstBindExpressions(processed)
				if err != nil {
					return nil, err
				}
				return result.Args, nil
			},
		}).
		Parse(sqlxTemplate)

	if err != nil {
		return err
	}

	return tmpl.Execute(w, ctx)
}
