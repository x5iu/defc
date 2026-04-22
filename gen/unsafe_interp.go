package gen

import (
	"fmt"
	"os"
	"strings"
	"text/template/parse"
)

type UnsafeInterpFinding struct {
	MethodArg  string
	Line       int
	NodeSource string
}

func scanUnsafeRawInterpolation(tree *parse.Tree, sharedTrees map[string]*parse.Tree, methodArgs map[string]struct{}, allowedSinks map[string]struct{}) []UnsafeInterpFinding {
	if tree == nil || tree.Root == nil {
		return nil
	}
	return walkTemplateList(tree.Root, true, methodArgs, allowedSinks, sharedTrees, "", nil)
}

func walkTemplateList(list *parse.ListNode, emitSQL bool, methodArgs map[string]struct{}, allowed map[string]struct{}, shared map[string]*parse.Tree, taint string, varTaint map[string]string) []UnsafeInterpFinding {
	if list == nil {
		return nil
	}
	if varTaint == nil {
		varTaint = map[string]string{}
	}
	var out []UnsafeInterpFinding
	for _, n := range list.Nodes {
		if an, ok := n.(*parse.ActionNode); ok && an.Pipe != nil && len(an.Pipe.Decl) > 0 {
			addDeclTaints(an.Pipe, methodArgs, varTaint)
		}
		out = append(out, walkTemplateNode(n, emitSQL, methodArgs, allowed, shared, taint, varTaint)...)
	}
	return out
}

func walkTemplateNode(n parse.Node, emitSQL bool, methodArgs map[string]struct{}, allowed map[string]struct{}, shared map[string]*parse.Tree, taint string, varTaint map[string]string) []UnsafeInterpFinding {
	switch n := n.(type) {
	case *parse.TextNode, *parse.CommentNode:
		return nil
	case *parse.IfNode:
		locals := copyVarTaint(varTaint)
		var out []UnsafeInterpFinding
		out = append(out, findPipeFindings(n.Pipe, false, methodArgs, allowed, shared, lineOf(n), taint, locals)...)
		out = append(out, walkTemplateList(n.List, emitSQL, methodArgs, allowed, shared, taint, locals)...)
		out = append(out, walkTemplateList(n.ElseList, emitSQL, methodArgs, allowed, shared, taint, locals)...)
		return out
	case *parse.RangeNode:
		var out []UnsafeInterpFinding
		out = append(out, findPipeFindings(n.Pipe, false, methodArgs, allowed, shared, lineOf(n), taint, varTaint)...)
		locals := copyVarTaint(varTaint)
		prov := branchTaint(n.Pipe, methodArgs)
		innerTaint := prov
		applyRangeDeclTaints(n.Pipe, prov, methodArgs, locals)
		out = append(out, walkTemplateList(n.List, emitSQL, methodArgs, allowed, shared, innerTaint, locals)...)
		out = append(out, walkTemplateList(n.ElseList, emitSQL, methodArgs, allowed, shared, taint, varTaint)...)
		return out
	case *parse.WithNode:
		var out []UnsafeInterpFinding
		out = append(out, findPipeFindings(n.Pipe, false, methodArgs, allowed, shared, lineOf(n), taint, varTaint)...)
		locals := copyVarTaint(varTaint)
		innerTaint := branchTaint(n.Pipe, methodArgs)
		out = append(out, walkTemplateList(n.List, emitSQL, methodArgs, allowed, shared, innerTaint, locals)...)
		out = append(out, walkTemplateList(n.ElseList, emitSQL, methodArgs, allowed, shared, taint, varTaint)...)
		return out
	case *parse.ActionNode:
		if n.Pipe == nil {
			return nil
		}
		if len(n.Pipe.Decl) > 0 {
			return findPipeFindings(n.Pipe, false, methodArgs, allowed, shared, lineOfPipe(n.Pipe), taint, varTaint)
		}
		return findPipeFindings(n.Pipe, emitSQL, methodArgs, allowed, shared, lineOfPipe(n.Pipe), taint, varTaint)
	case *parse.TemplateNode:
		var out []UnsafeInterpFinding
		locals := copyVarTaint(varTaint)
		out = append(out, findPipeFindings(n.Pipe, false, methodArgs, allowed, shared, n.Line, taint, locals)...)
		if st := shared[n.Name]; st != nil {
			out = append(out, walkTemplateList(st.Root, emitSQL, methodArgs, allowed, shared, taint, locals)...)
		}
		return out
	case *parse.BreakNode, *parse.ContinueNode:
		return nil
	default:
		return nil
	}
}

func copyVarTaint(m map[string]string) map[string]string {
	if len(m) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func addDeclTaints(p *parse.PipeNode, methodArgs map[string]struct{}, varTaint map[string]string) {
	prov := branchTaint(p, methodArgs)
	if prov == "" {
		return
	}
	for _, vn := range p.Decl {
		if name, ok := varNameFromIdent(vn.Ident); ok {
			varTaint[name] = prov
		}
	}
}

func applyRangeDeclTaints(p *parse.PipeNode, prov string, methodArgs map[string]struct{}, varTaint map[string]string) {
	if p == nil || prov == "" || len(p.Decl) == 0 {
		return
	}
	decls := p.Decl
	if len(decls) == 1 {
		if name, ok := varNameFromIdent(decls[0].Ident); ok {
			varTaint[name] = prov
		}
		return
	}
	if len(decls) == 2 {
		if name, ok := varNameFromIdent(decls[1].Ident); ok {
			varTaint[name] = prov
		}
	}
}

func vnameFromVariable(n *parse.VariableNode) (string, bool) {
	if n == nil {
		return "", false
	}
	if len(n.Ident) >= 2 && n.Ident[0] == "$" && n.Ident[1] != "" {
		return n.Ident[1], true
	}
	if len(n.Ident) == 1 {
		s := n.Ident[0]
		if len(s) > 1 && s[0] == '$' {
			return s[1:], true
		}
	}
	return "", false
}

func varNameFromIdent(id []string) (string, bool) {
	if len(id) < 2 || id[0] != "$" {
		if len(id) == 1 && len(id[0]) > 0 {
			s := id[0]
			if s[0] == '$' && len(s) > 1 {
				return s[1:], true
			}
		}
		return "", false
	}
	if id[1] == "" {
		return "", false
	}
	return id[1], true
}

func branchTaint(p *parse.PipeNode, methodArgs map[string]struct{}) string {
	if p == nil {
		return ""
	}
	for _, c := range p.Cmds {
		for _, a := range c.Args {
			if s := firstMethodArgRefInExpr(a, methodArgs); s != "" {
				return s
			}
		}
	}
	return ""
}

func firstMethodArgRefInExpr(n parse.Node, methodArgs map[string]struct{}) string {
	switch n := n.(type) {
	case *parse.FieldNode:
		if len(n.Ident) > 0 {
			if _, ok := methodArgs[n.Ident[0]]; ok {
				return n.Ident[0]
			}
		}
	case *parse.VariableNode:
		if v, ok := vnameFromVariable(n); ok {
			if _, in := methodArgs[v]; in {
				return v
			}
		}
	case *parse.ChainNode:
		return firstMethodArgRefInExpr(n.Node, methodArgs)
	case *parse.PipeNode:
		return branchTaint(n, methodArgs)
	}
	return ""
}

func lineOf(n parse.Node) int {
	switch n := n.(type) {
	case *parse.IfNode:
		return n.Line
	case *parse.RangeNode:
		return n.Line
	case *parse.WithNode:
		return n.Line
	default:
		return 0
	}
}

func lineOfPipe(p *parse.PipeNode) int {
	if p == nil {
		return 0
	}
	if p.Line > 0 {
		return p.Line
	}
	return 0
}

func findPipeFindings(pipe *parse.PipeNode, emitSQL bool, methodArgs map[string]struct{}, allowed map[string]struct{}, shared map[string]*parse.Tree, line int, taint string, varTaint map[string]string) []UnsafeInterpFinding {
	if pipe == nil {
		return nil
	}
	if len(pipe.Decl) > 0 || pipe.IsAssign {
		var out []UnsafeInterpFinding
		for _, cmd := range pipe.Cmds {
			for _, arg := range cmd.Args {
				out = append(out, walkTemplateNode(arg, false, methodArgs, allowed, shared, taint, varTaint)...)
			}
		}
		return out
	}
	if !emitSQL {
		var out []UnsafeInterpFinding
		for _, cmd := range pipe.Cmds {
			for _, arg := range cmd.Args {
				out = append(out, walkTemplateNode(arg, false, methodArgs, allowed, shared, taint, varTaint)...)
			}
		}
		return out
	}
	if pipeAllowedSink(pipe, allowed) {
		return nil
	}
	ln := line
	if pipe.Line > 0 {
		ln = pipe.Line
	}
	var out []UnsafeInterpFinding
	for _, cmd := range pipe.Cmds {
		for _, arg := range cmd.Args {
			out = append(out, scanArgForRawInterpolation(arg, methodArgs, allowed, shared, ln, taint, varTaint)...)
		}
	}
	return out
}

func pipeAllowedSink(pipe *parse.PipeNode, allowed map[string]struct{}) bool {
	if pipe == nil || len(pipe.Cmds) == 0 {
		return false
	}
	last := pipe.Cmds[len(pipe.Cmds)-1]
	if len(last.Args) == 0 {
		return false
	}
	id, ok := last.Args[0].(*parse.IdentifierNode)
	if !ok {
		return false
	}
	_, ok = allowed[id.Ident]
	return ok
}

func scanArgForRawInterpolation(n parse.Node, methodArgs map[string]struct{}, allowed map[string]struct{}, shared map[string]*parse.Tree, line int, taint string, varTaint map[string]string) []UnsafeInterpFinding {
	switch n := n.(type) {
	case *parse.FieldNode:
		if len(n.Ident) == 0 {
			return nil
		}
		if _, ok := methodArgs[n.Ident[0]]; ok {
			return []UnsafeInterpFinding{{MethodArg: n.Ident[0], Line: line, NodeSource: strings.TrimSpace(n.String())}}
		}
		if taint != "" {
			return []UnsafeInterpFinding{{MethodArg: taint, Line: line, NodeSource: strings.TrimSpace(n.String())}}
		}
		return nil
	case *parse.VariableNode:
		if vname, ok := vnameFromVariable(n); ok {
			if prov, ok := varTaint[vname]; ok && prov != "" {
				return []UnsafeInterpFinding{{MethodArg: prov, Line: line, NodeSource: strings.TrimSpace(n.String())}}
			}
			if _, ok := methodArgs[vname]; ok {
				return []UnsafeInterpFinding{{MethodArg: vname, Line: line, NodeSource: strings.TrimSpace(n.String())}}
			}
		}
		return nil
	case *parse.DotNode:
		if taint != "" {
			return []UnsafeInterpFinding{{MethodArg: taint, Line: line, NodeSource: n.String()}}
		}
		return []UnsafeInterpFinding{{MethodArg: ".", Line: line, NodeSource: n.String()}}
	case *parse.ChainNode:
		return scanChainForRaw(n, methodArgs, line, taint, varTaint)
	case *parse.PipeNode:
		return findPipeFindings(n, true, methodArgs, allowed, shared, line, taint, varTaint)
	default:
		return nil
	}
}

func scanChainForRaw(c *parse.ChainNode, methodArgs map[string]struct{}, line int, taint string, varTaint map[string]string) []UnsafeInterpFinding {
	switch r := c.Node.(type) {
	case *parse.DotNode:
		if len(c.Field) > 0 {
			if _, ok := methodArgs[c.Field[0]]; ok {
				return []UnsafeInterpFinding{{MethodArg: c.Field[0], Line: line, NodeSource: strings.TrimSpace(c.String())}}
			}
			if taint != "" {
				return []UnsafeInterpFinding{{MethodArg: taint, Line: line, NodeSource: strings.TrimSpace(c.String())}}
			}
		}
	case *parse.VariableNode:
		vname, vok := vnameFromVariable(r)
		if !vok {
			return nil
		}
		if prov, ok := varTaint[vname]; ok && prov != "" {
			return []UnsafeInterpFinding{{MethodArg: prov, Line: line, NodeSource: strings.TrimSpace(c.String())}}
		}
		if _, ok := methodArgs[vname]; ok {
			return []UnsafeInterpFinding{{MethodArg: vname, Line: line, NodeSource: strings.TrimSpace(c.String())}}
		}
	}
	return nil
}

func sqlxParseStubFuncs(ctx *sqlxContext) map[string]any {
	fm := map[string]any{
		"bind":     func(...any) any { return "" },
		"bindvars": func(...any) any { return "" },
	}
	for name := range ctx.AdditionalFuncs() {
		if _, ok := fm[name]; ok {
			continue
		}
		fm[name] = func(...any) any { return "" }
	}
	return fm
}

func templateForestReferencesBind(trees map[string]*parse.Tree) bool {
	if trees == nil {
		return false
	}
	for _, tree := range trees {
		if tree == nil || tree.Root == nil {
			continue
		}
		if walkForestBind(tree.Root, trees) {
			return true
		}
	}
	return false
}

func walkForestBind(list *parse.ListNode, shared map[string]*parse.Tree) bool {
	if list == nil {
		return false
	}
	for _, n := range list.Nodes {
		if walkNodeBind(n, shared) {
			return true
		}
	}
	return false
}

func walkNodeBind(n parse.Node, shared map[string]*parse.Tree) bool {
	switch n := n.(type) {
	case *parse.ActionNode:
		if n.Pipe == nil {
			return false
		}
		for _, cmd := range n.Pipe.Cmds {
			if len(cmd.Args) == 0 {
				continue
			}
			if id, ok := cmd.Args[0].(*parse.IdentifierNode); ok && id.Ident == "bind" {
				return true
			}
		}
		for _, cmd := range n.Pipe.Cmds {
			for _, arg := range cmd.Args {
				if walkNodeBind(arg, shared) {
					return true
				}
			}
		}
		return false
	case *parse.IfNode:
		return walkForestBind(n.List, shared) || walkForestBind(n.ElseList, shared)
	case *parse.RangeNode:
		return walkForestBind(n.List, shared) || walkForestBind(n.ElseList, shared)
	case *parse.WithNode:
		return walkForestBind(n.List, shared) || walkForestBind(n.ElseList, shared)
	case *parse.TemplateNode:
		if walkPipeBindArgs(n.Pipe, shared) {
			return true
		}
		if sub := shared[n.Name]; sub != nil && sub.Root != nil {
			return walkForestBind(sub.Root, shared)
		}
		return false
	case *parse.ListNode:
		return walkForestBind(n, shared)
	default:
		return false
	}
}

func walkPipeBindArgs(pipe *parse.PipeNode, shared map[string]*parse.Tree) bool {
	if pipe == nil {
		return false
	}
	for _, cmd := range pipe.Cmds {
		for _, arg := range cmd.Args {
			if walkNodeBind(arg, shared) {
				return true
			}
		}
	}
	return false
}

func (ctx *sqlxContext) methodSQLArgNames(method *Method) map[string]struct{} {
	out := make(map[string]struct{})
	for _, id := range method.SortIn() {
		if ctx.Doc.IsContextType(id, method.In[id]) {
			continue
		}
		out[id] = struct{}{}
	}
	return out
}

func (ctx *sqlxContext) emitSqlxUnsafeInterpolationWarnings() {
	allowed := map[string]struct{}{"bind": {}, "bindvars": {}}
	stub := sqlxParseStubFuncs(ctx)
	var sharedTrees map[string]*parse.Tree
	if ctx.Template != "" {
		var err error
		sharedTrees, err = parse.Parse("defc-shared", ctx.Template, "{{", "}}", stub)
		if err != nil {
			sharedTrees = nil
		}
	}
	if sharedTrees == nil {
		sharedTrees = map[string]*parse.Tree{}
	}
	for _, method := range ctx.Methods {
		if method.Ident == sqlxMethodWithTx {
			continue
		}
		opts := method.SqlxOptions()
		if hasOption(opts, "CONST") || hasOption(opts, "CONSTBIND") {
			continue
		}
		body, err := readHeader(method.Header, ctx.Pwd)
		if err != nil {
			continue
		}
		trees, err := parse.Parse(method.Ident, body, "{{", "}}", stub)
		if err != nil {
			continue
		}
		tree := trees[method.Ident]
		if tree == nil {
			continue
		}
		findings := scanUnsafeRawInterpolation(tree, sharedTrees, ctx.methodSQLArgNames(method), allowed)
		for _, f := range findings {
			arg := f.MethodArg
			atLoc := fmt.Sprintf("%s:%d", method.Ident, f.Line)
			if ctx.File != "" {
				atLoc = fmt.Sprintf("%s:%d", ctx.File, f.Line)
			}
			fmt.Fprintf(os.Stderr, "defc: warning: method %s at %s interpolates argument %q as raw SQL text outside bind/bindvars.\n", method.Ident, atLoc, arg)
			if arg == "." {
				fmt.Fprintf(os.Stderr, "  This pattern is a SQL-injection foot-gun at runtime. Use {{ bind . }} if the intent is to bind the dot value, CONSTBIND where applicable, or positional ? placeholders instead.\n")
			} else {
				fmt.Fprintf(os.Stderr, "  This pattern is a SQL-injection foot-gun at runtime. Use {{ bind $.%s }} (requires the `bind` option), CONSTBIND with ${%s}, or positional ? placeholders instead.\n", arg, arg)
			}
			fmt.Fprintf(os.Stderr, "  Raw interpolation: %s\n", f.NodeSource)
			fmt.Fprintf(os.Stderr, "  See https://github.com/x5iu/defc/issues/17 for guidance.\n")
		}
	}
}
