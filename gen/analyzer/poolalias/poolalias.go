// Package poolalias implements a go/analysis Analyzer that detects pool
// aliasing bugs: flowing a *bytes.Buffer's Bytes() slice, backed by a
// sync.Pool, into a retaining sink (e.g. NewResponseError, Response.FromBytes)
// without first routing the bytes through a defensive copy (DetachBytes).
//
// Recognition rules (R4 §5):
//
//	(a) taint source:      runtime.GetBuffer(), __<Ident>GetBuffer()
//	(b) taint propagation: buf.Bytes() where buf is a tainted *bytes.Buffer
//	                       (buf.String() is explicitly NOT tainted)
//	(c) sink:              runtime.NewResponseError,
//	                       __<Ident>NewResponseError (local package),
//	                       methods named FromBytes(string, []byte) error
//	                       (proxy for runtime.Response satisfaction)
//	(d) launder:           runtime.DetachBytes / __<Ident>DetachBytes
//
// Silence with a "// lint:pool-alias-ok <reason>" comment on the sink line
// or the line immediately preceding it; an empty reason is itself a
// diagnostic.
package poolalias

import (
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const doc = `report *bytes.Buffer.Bytes() slices from sync.Pool that escape into retaining sinks without DetachBytes`

var Analyzer = &analysis.Analyzer{
	Name: "poolalias",
	Doc:  doc,
	Run:  run,
}

var (
	nortGetBufferRE   = regexp.MustCompile(`^__[A-Z]\w*GetBuffer$`)
	nortNewRespErrRE  = regexp.MustCompile(`^__[A-Z]\w*NewResponseError$`)
	nortDetachBytesRE = regexp.MustCompile(`^__[A-Z]\w*DetachBytes$`)
)

const runtimeImportPath = "github.com/x5iu/defc/runtime"

type taintKind int

const (
	taintNone taintKind = iota
	taintBuffer
	taintBytes
)

type state struct {
	pass    *analysis.Pass
	taints  map[types.Object]taintKind
	waivers map[int]string
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, f := range pass.Files {
		s := &state{
			pass:    pass,
			taints:  make(map[types.Object]taintKind),
			waivers: collectWaivers(pass.Fset, f),
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			s.walkFunc(fn)
		}
	}
	return nil, nil
}

func collectWaivers(fset *token.FileSet, f *ast.File) map[int]string {
	out := make(map[int]string)
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			text := strings.TrimPrefix(c.Text, "//")
			text = strings.TrimPrefix(text, "/*")
			text = strings.TrimSuffix(text, "*/")
			text = strings.TrimSpace(text)
			if !strings.HasPrefix(text, "lint:pool-alias-ok") {
				continue
			}
			reason := strings.TrimSpace(strings.TrimPrefix(text, "lint:pool-alias-ok"))
			line := fset.Position(c.Pos()).Line
			if _, exists := out[line]; !exists {
				out[line] = reason
			}
			if _, exists := out[line+1]; !exists {
				out[line+1] = reason
			}
		}
	}
	return out
}

func (s *state) walkFunc(fn *ast.FuncDecl) {
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, rhs := range as.Rhs {
			if i >= len(as.Lhs) {
				break
			}
			lhsIdent, ok := as.Lhs[i].(*ast.Ident)
			if !ok {
				continue
			}
			obj := s.pass.TypesInfo.Defs[lhsIdent]
			if obj == nil {
				obj = s.pass.TypesInfo.Uses[lhsIdent]
			}
			if obj == nil {
				continue
			}
			if kind := s.classifyExpr(rhs); kind != taintNone {
				s.taints[obj] = kind
			}
		}
		return true
	})

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		s.checkSink(call)
		return true
	})
}

func (s *state) classifyExpr(e ast.Expr) taintKind {
	switch x := e.(type) {
	case *ast.CallExpr:
		if s.isGetBufferCall(x) {
			return taintBuffer
		}
		if s.isDetachBytesCall(x) {
			return taintNone
		}
		if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel != nil {
			if sel.Sel.Name == "Bytes" && len(x.Args) == 0 {
				if s.exprTaint(sel.X) == taintBuffer {
					return taintBytes
				}
			}
		}
	case *ast.Ident:
		if obj := s.pass.TypesInfo.Uses[x]; obj != nil {
			return s.taints[obj]
		}
	}
	return taintNone
}

func (s *state) exprTaint(e ast.Expr) taintKind {
	id, ok := e.(*ast.Ident)
	if !ok {
		return taintNone
	}
	if obj := s.pass.TypesInfo.Uses[id]; obj != nil {
		return s.taints[obj]
	}
	return taintNone
}

func (s *state) isGetBufferCall(call *ast.CallExpr) bool {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		if fun.Sel == nil || fun.Sel.Name != "GetBuffer" {
			return false
		}
		return s.selectorFromRuntime(fun)
	case *ast.Ident:
		return nortGetBufferRE.MatchString(fun.Name)
	}
	return false
}

func (s *state) isDetachBytesCall(call *ast.CallExpr) bool {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		if fun.Sel == nil || fun.Sel.Name != "DetachBytes" {
			return false
		}
		return s.selectorFromRuntime(fun)
	case *ast.Ident:
		return nortDetachBytesRE.MatchString(fun.Name)
	}
	return false
}

func (s *state) selectorFromRuntime(sel *ast.SelectorExpr) bool {
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	obj := s.pass.TypesInfo.Uses[id]
	if obj == nil {
		return false
	}
	pkgName, ok := obj.(*types.PkgName)
	if !ok {
		return false
	}
	return pkgName.Imported().Path() == runtimeImportPath
}

func (s *state) checkSink(call *ast.CallExpr) {
	sinkName, ok := s.sinkName(call)
	if !ok {
		return
	}
	for _, arg := range call.Args {
		if s.classifyExpr(arg) == taintBytes {
			s.report(call, sinkName)
			return
		}
	}
}

func (s *state) sinkName(call *ast.CallExpr) (string, bool) {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		if fun.Sel == nil {
			return "", false
		}
		switch fun.Sel.Name {
		case "NewResponseError":
			if s.selectorFromRuntime(fun) {
				return "runtime.NewResponseError", true
			}
		case "FromBytes":
			if s.isResponseFromBytes(fun) {
				return "Response.FromBytes", true
			}
		}
	case *ast.Ident:
		if nortNewRespErrRE.MatchString(fun.Name) {
			return fun.Name, true
		}
	}
	return "", false
}

func (s *state) isResponseFromBytes(sel *ast.SelectorExpr) bool {
	t := s.pass.TypesInfo.TypeOf(sel.X)
	if t == nil {
		return false
	}
	obj, _, _ := types.LookupFieldOrMethod(t, true, s.pass.Pkg, "FromBytes")
	if obj == nil {
		obj, _, _ = types.LookupFieldOrMethod(types.NewPointer(t), true, s.pass.Pkg, "FromBytes")
	}
	if obj == nil {
		return false
	}
	sig, ok := obj.Type().(*types.Signature)
	if !ok {
		return false
	}
	if sig.Params().Len() != 2 || sig.Results().Len() != 1 {
		return false
	}
	if !isStringType(sig.Params().At(0).Type()) {
		return false
	}
	if !isByteSlice(sig.Params().At(1).Type()) {
		return false
	}
	return isErrorType(sig.Results().At(0).Type())
}

func isStringType(t types.Type) bool {
	basic, ok := t.Underlying().(*types.Basic)
	return ok && basic.Kind() == types.String
}

func isByteSlice(t types.Type) bool {
	sl, ok := t.Underlying().(*types.Slice)
	if !ok {
		return false
	}
	basic, ok := sl.Elem().Underlying().(*types.Basic)
	return ok && basic.Kind() == types.Uint8
}

func isErrorType(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	return named.Obj().Name() == "error" && named.Obj().Pkg() == nil
}

func (s *state) report(call *ast.CallExpr, sink string) {
	line := s.pass.Fset.Position(call.Pos()).Line
	if reason, ok := s.waivers[line]; ok {
		if reason == "" {
			s.pass.Reportf(call.Pos(), "poolalias: empty lint:pool-alias-ok reason")
		}
		return
	}
	s.pass.Reportf(call.Pos(),
		"poolalias: %s receives pool-backed []byte without DetachBytes; escape may outlive PutBuffer", sink)
}
