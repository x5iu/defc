package main

import (
	"bytes"
	"go/ast"
	"net/http"
	"sort"
)

// Method represents a method declaration in an interface
type Method struct {
	// Meta represents first-line comment of this method, who
	// looks like a command in cli, the first argument should
	// always be the name of this method, which is 'Ident'
	// field below
	Meta string

	// Header represents contents after first-line comment,
	// who is HTTP header message with '--mode=api' arg or
	// literal sql string with '--mode=sqlx' arg
	Header string

	Ident     string
	In        map[string]ast.Expr
	UnnamedIn []ast.Expr
	Out       []ast.Expr

	// Source represents the raw file content
	Source []byte
}

func (method *Method) SortIn() []string {
	idents := make([]string, 0, len(method.In))
	for k := range method.In {
		idents = append(idents, k)
	}
	sort.Slice(idents, func(i, j int) bool {
		return method.In[idents[i]].Pos() < method.In[idents[j]].Pos()
	})
	return idents
}

func (method *Method) MetaArgs() []string {
	rawArgs := splitArgs(method.Meta)
	args := make([]string, 0, len(rawArgs))
	for i := 0; i < len(rawArgs); i++ {
		if rawArgs[i] != "" && rawArgs[i] != " " {
			args = append(args, rawArgs[i])
		}
	}
	return args
}

// TmplURL should only be used with '--mode=api' arg
func (method *Method) TmplURL() string {
	args := method.MetaArgs()
	if len(args) >= 1 {
		return args[len(args)-1]
	}
	return ""
}

var availableMethods = []string{
	http.MethodGet,
	http.MethodPost,
	http.MethodPut,
}

// MethodHTTP should only be used with '--mode=api' arg
func (method *Method) MethodHTTP() string {
	args := method.MetaArgs()
	if len(args) >= 2 {
		for _, httpMethod := range availableMethods {
			if toUpper(args[len(args)-2]) == httpMethod {
				return httpMethod
			}
		}
	}
	return ""
}

var availableOperations = []string{
	SqlxOpExec,
	SqlxOpQuery,
}

// SqlOperation should only be used with '--mode=sqlx' arg
func (method *Method) SqlOperation() string {
	args := method.MetaArgs()
	if len(args) >= 2 {
		for _, operation := range availableOperations {
			if toUpper(args[1]) == operation {
				return operation
			}
		}
	}
	return ""
}

// SqlFeatures should only be used with '--mode=sqlx' arg
func (method *Method) SqlFeatures() []string {
	args := method.MetaArgs()
	if len(args) >= 3 {
		feats := make([]string, 0, len(args[2:]))
		for _, feat := range args[2:] {
			feats = append(feats, toUpper(feat))
		}
		return feats
	}
	return nil
}

func (method *Method) HasContext() bool {
	for ident, ty := range method.In {
		if isContextType(ident, ty, method.Source) {
			return true
		}
	}

	// for sqlx WithTxContext, we should consider unnamed arguments
	for _, ty := range method.UnnamedIn {
		if isContextType("", ty, method.Source) {
			return true
		}
	}

	return false
}

func (method *Method) ReturnSlice() bool {
	return len(method.Out) > 1 && isSlice(method.Out[0])
}

func inspectMethod(node ast.Node, source []byte) (method *Method) {
	field := node.(*ast.Field)
	method = new(Method)
	method.Source = source
	if field.Doc != nil {
		method.Meta = trimSpace(
			trimPrefix(field.Doc.List[0].Text,
				"//"),
		)
		var buffer bytes.Buffer
		for _, header := range field.Doc.List[1:] {
			buffer.WriteString(trimSpace(trimPrefix(header.Text, "//")))
			buffer.WriteString("\r\n")
		}
		method.Header = buffer.String()
		switch len(method.Header) {
		default:
			if method.Header[len(method.Header)-4:] == "\r\n\r\n" {
				break
			}
			fallthrough
		case 2, 3:
			if method.Header[len(method.Header)-2:] == "\r\n" {
				method.Header += "\r\n"
			} else {
				method.Header += "\r\n\r\n"
			}
		case 1:
			method.Header += "\r\n\r\n"
		case 0:
		}
	}
	method.Ident = field.Names[0].Name
	funcType := field.Type.(*ast.FuncType)
	inParams := funcType.Params.List
	method.In = make(map[string]ast.Expr, len(inParams))
	for _, param := range inParams {
		if param.Names != nil {
			for _, name := range param.Names {
				method.In[name.Name] = param.Type
			}
		} else {
			method.UnnamedIn = append(method.UnnamedIn, param.Type)
		}
	}
	if funcType.Results != nil {
		outParams := funcType.Results.List
		method.Out = make([]ast.Expr, 0, len(outParams))
		for _, param := range outParams {
			method.Out = append(method.Out, param.Type)
		}
	}
	return method
}
