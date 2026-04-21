// Package lint implements `defc lint`, a context-aware linter that
// flags unsafe template interpolations in defc interface schemas.
//
// The walker parses each .go file with go/parser, finds interface
// declarations whose methods carry defc-style doc comments, extracts
// the SQL / URL / header template source, parses it with
// text/template/parse, and emits findings for bare {{.x}} actions in
// contexts where an attacker-controlled value would be unsafe.
//
// Scope: sqlx and api modes. RPC is skipped — no templated
// interpolation reaches callers.
package lint

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template/parse"
)

// Severity categorises findings.
type Severity string

const (
	SevBlocker Severity = "BLOCKER"
	SevNote    Severity = "NOTE"
)

// Finding describes a single unsafe interpolation site.
type Finding struct {
	File         string   `json:"file"`
	Line         int      `json:"line"`
	Column       int      `json:"col"`
	Mode         string   `json:"mode"`
	Kind         string   `json:"kind"`
	Action       string   `json:"action"`
	Severity     Severity `json:"severity"`
	SuggestedFix string   `json:"suggested_fix,omitempty"`
	Why          string   `json:"why,omitempty"`
}

// Summary aggregates findings by severity.
type Summary struct {
	Blockers int `json:"blockers"`
	Notes    int `json:"notes"`
}

// Report is the root object emitted in JSON format.
type Report struct {
	Findings []Finding `json:"findings"`
	Summary  Summary   `json:"summary"`
}

// Options controls the walker.
type Options struct {
	// Only restricts findings to contexts whose Kind prefix is in the
	// set (e.g. {"sql"}, {"url"}, {"header"}). Empty = all.
	Only []string
	// Ignore is a list of file-path regexes; findings whose File
	// matches any are dropped.
	Ignore []*regexp.Regexp
	// Strict promotes NOTE findings to BLOCKER.
	Strict bool
}

// Run walks every *.go path and returns findings.
func Run(paths []string, opts Options) (*Report, error) {
	if len(paths) == 0 {
		paths = []string{"."}
	}
	var (
		files   []string
		visited = make(map[string]bool)
	)
	for _, p := range paths {
		if err := collectFiles(p, visited, &files); err != nil {
			return nil, err
		}
	}
	sort.Strings(files)

	rep := &Report{Findings: []Finding{}}
	for _, f := range files {
		fs, err := lintFile(f)
		if err != nil {
			return nil, fmt.Errorf("lint %s: %w", f, err)
		}
		rep.Findings = append(rep.Findings, fs...)
	}
	rep.Findings = filterFindings(rep.Findings, opts)
	if opts.Strict {
		for i := range rep.Findings {
			if rep.Findings[i].Severity == SevNote {
				rep.Findings[i].Severity = SevBlocker
			}
		}
	}
	for _, f := range rep.Findings {
		switch f.Severity {
		case SevBlocker:
			rep.Summary.Blockers++
		case SevNote:
			rep.Summary.Notes++
		}
	}
	return rep, nil
}

func collectFiles(root string, visited map[string]bool, out *[]string) error {
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		if strings.HasSuffix(root, ".go") && !visited[root] {
			visited[root] = true
			*out = append(*out, root)
		}
		return nil
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			name := d.Name()
			if path != root && (name == "vendor" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") && !visited[path] {
			visited[path] = true
			*out = append(*out, path)
		}
		return nil
	})
}

func filterFindings(list []Finding, opts Options) []Finding {
	if len(opts.Only) == 0 && len(opts.Ignore) == 0 {
		return list
	}
	out := list[:0]
NEXT:
	for _, f := range list {
		for _, re := range opts.Ignore {
			if re.MatchString(f.File) {
				continue NEXT
			}
		}
		if len(opts.Only) > 0 {
			ok := false
			for _, o := range opts.Only {
				if strings.HasPrefix(f.Kind, o) {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		out = append(out, f)
	}
	return out
}

// lintFile parses path and returns findings for every interface
// method whose doc-comment looks like a defc schema method.
func lintFile(path string) ([]Finding, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	abs, _ := filepath.Abs(path)

	var findings []Finding
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			iface, ok := ts.Type.(*ast.InterfaceType)
			if !ok {
				continue
			}
			for _, m := range iface.Methods.List {
				if m.Doc == nil || len(m.Names) == 0 {
					continue
				}
				findings = append(findings, lintMethod(fset, abs, m)...)
			}
		}
	}
	return findings, nil
}

func lintMethod(fset *token.FileSet, file string, m *ast.Field) []Finding {
	meta, body, basePos := extractTemplate(fset, m)
	if meta == "" {
		return nil
	}
	mode, op := detectMode(meta)
	if mode == "" {
		return nil
	}
	if strings.EqualFold(op, "constbind") {
		return nil
	}
	switch mode {
	case "sqlx":
		return lintSQL(file, basePos, body)
	case "api":
		url, hdr := splitURLHeader(meta, body)
		var out []Finding
		out = append(out, lintURL(file, basePos, url)...)
		out = append(out, lintHeader(file, basePos, hdr)...)
		return out
	}
	return nil
}

// extractTemplate returns (metaLine, templateBody, positionBase).
// metaLine is the first non-empty doc-comment line, stripped of its
// comment markers and trimmed. templateBody is the concatenation of
// all remaining doc-comment lines with markers stripped, joined with
// LF. basePos is the source position of the first doc-comment.
func extractTemplate(fset *token.FileSet, m *ast.Field) (string, string, token.Position) {
	if m.Doc == nil || len(m.Doc.List) == 0 {
		return "", "", token.Position{}
	}
	var (
		meta  string
		parts []string
	)
	for _, c := range m.Doc.List {
		line := stripCommentMarkers(c.Text)
		if meta == "" {
			trim := strings.TrimSpace(line)
			if trim != "" {
				meta = trim
				continue
			}
		}
		parts = append(parts, line)
	}
	body := strings.Join(parts, "\n")
	pos := fset.Position(m.Doc.List[0].Slash)
	return meta, body, pos
}

func stripCommentMarkers(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if strings.HasPrefix(s, "//") {
		return strings.TrimPrefix(s, "//")
	}
	if strings.HasPrefix(s, "/*") {
		s = strings.TrimPrefix(s, "/*")
		s = strings.TrimSuffix(s, "*/")
		return s
	}
	return s
}

var (
	sqlxOpRe       = regexp.MustCompile(`(?i)^(\S+)\s+(exec|query)\b`)
	apiOpRe        = regexp.MustCompile(`(?i)^(\S+)\s+(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS|CONNECT|TRACE)\b`)
	constbindOptRe = regexp.MustCompile(`(?i)\bconstbind\b`)
)

func detectMode(meta string) (mode, op string) {
	if m := sqlxOpRe.FindStringSubmatch(meta); m != nil {
		if constbindOptRe.MatchString(meta) {
			return "sqlx", "constbind"
		}
		return "sqlx", strings.ToLower(m[2])
	}
	if m := apiOpRe.FindStringSubmatch(meta); m != nil {
		return "api", strings.ToUpper(m[2])
	}
	return "", ""
}

// splitURLHeader extracts the URL template (last token of the meta
// line) and returns the body as the header template.
func splitURLHeader(meta, body string) (url, header string) {
	fields := strings.Fields(meta)
	if len(fields) > 0 {
		url = fields[len(fields)-1]
	}
	header = body
	return
}

// parseSafe runs text/template/parse with stub helpers so that any
// defc-registered helper name does not cause a parse error.
func parseSafe(name, src string) *parse.Tree {
	funcs := map[string]any{}
	for _, fn := range []string{
		"bind", "bindvars", "identifier", "pathseg", "query", "header",
		"page", "quote", "escape",
	} {
		funcs[fn] = func(...any) string { return "" }
	}
	trees, err := parse.Parse(name, src, "{{", "}}", funcs)
	if err != nil {
		return nil
	}
	return trees[name]
}

// safeHelpers are helper identifiers that, when appearing as the
// first command of an action, render the action safe in the given
// context.
var (
	safeSQL    = map[string]bool{"bind": true, "bindvars": true, "identifier": true}
	safeURL    = map[string]bool{"pathseg": true, "query": true}
	safeHeader = map[string]bool{"header": true}
)

// actionIsSafe reports whether the leading identifier of act is in
// safe.
func actionIsSafe(act *parse.ActionNode, safe map[string]bool) bool {
	if act == nil || act.Pipe == nil || len(act.Pipe.Cmds) == 0 {
		return false
	}
	first := act.Pipe.Cmds[0]
	if len(first.Args) == 0 {
		return false
	}
	if id, ok := first.Args[0].(*parse.IdentifierNode); ok {
		return safe[id.Ident]
	}
	return false
}

func actionText(act *parse.ActionNode) string {
	return "{{" + act.Pipe.String() + "}}"
}

// walkActions invokes visit on every ActionNode in the tree.
func walkActions(n parse.Node, visit func(*parse.ActionNode)) {
	if n == nil {
		return
	}
	switch x := n.(type) {
	case *parse.ListNode:
		if x == nil {
			return
		}
		for _, c := range x.Nodes {
			walkActions(c, visit)
		}
	case *parse.ActionNode:
		visit(x)
	case *parse.IfNode:
		walkActions(x.List, visit)
		walkActions(x.ElseList, visit)
	case *parse.RangeNode:
		walkActions(x.List, visit)
		walkActions(x.ElseList, visit)
	case *parse.WithNode:
		walkActions(x.List, visit)
		walkActions(x.ElseList, visit)
	}
}

// posForOffset translates a byte offset within the template source
// back to file line/col.
func posForOffset(base token.Position, src string, off int) (int, int) {
	if off < 0 {
		off = 0
	}
	if off > len(src) {
		off = len(src)
	}
	line := base.Line
	col := base.Column
	for i := 0; i < off; i++ {
		if src[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}

// EmitText writes findings in text format.
func EmitText(w io.Writer, rep *Report) {
	for _, f := range rep.Findings {
		fmt.Fprintf(w, "%s:%d:%d: [%s] kind=%s action=%s suggested=%s\n",
			f.File, f.Line, f.Column, f.Severity, f.Kind, f.Action, f.SuggestedFix)
	}
}

// EmitJSON writes the report as pretty JSON.
func EmitJSON(w io.Writer, rep *Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}
