package gen

import (
	"bytes"
	"fmt"
	"go/ast"
	"net/http"
	"regexp"
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
	OrderedIn []string // to make In sorted
	In        map[string]ast.Expr
	UnnamedIn []ast.Expr
	Out       []ast.Expr

	// Source represents the raw file content
	Source []byte
}

func (method *Method) TxType() (ast.Expr, error) {
	var lastType ast.Expr
	if len(method.OrderedIn) > 0 {
		lastIn := method.OrderedIn[len(method.OrderedIn)-1]
		lastType = method.In[lastIn]
	} else if len(method.UnnamedIn) > 0 {
		lastType = method.UnnamedIn[len(method.UnnamedIn)-1]
	} else {
		return nil, fmt.Errorf("method %s expects at least one argument", method.Ident)
	}
	if funcType, ok := lastType.(*ast.FuncType); ok {
		if len(funcType.Params.List) != 1 {
			return nil, fmt.Errorf(
				"method %s expects an *ast.FuncType as arguments, who has and only has one argument",
				method.Ident,
			)
		}
		fnIn := funcType.Params.List[0]
		return fnIn.Type, nil
	} else {
		return nil, fmt.Errorf("method %s expects a function as the last argument", method.Ident)
	}
}

func (method *Method) SortIn() []string {
	return method.OrderedIn
}

var backslashRe = regexp.MustCompile(`\\[ \t\r]*?\n[ \t\r]*`)

func (method *Method) MetaArgs() []string {
	rawArgs := splitArgs(backslashRe.ReplaceAllString(method.Meta, ""))
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

var minusRe = regexp.MustCompile(`(?m)^[ \t]*?-[ \t]*`)

// TmplHeader should only be used with '--mode=api' arg
func (method *Method) TmplHeader() string {
	var (
		header = method.Header
		body   string
	)
	if idx := index(header, "\r\n\r\n"); idx != -1 {
		body = trimSpace(header[idx+4:])
		header = trimSpace(header[:idx])
	}
	if idx := index(header, "\n\n"); idx != -1 {
		body = trimSpace(header[idx+2:])
		header = trimSpace(header[:idx])
	}
	header = minusRe.ReplaceAllString(header, "") + "\r\n\r\n"
	if len(body) > 0 {
		header += body
	}
	return header
}

var availableMethods = []string{
	http.MethodGet,
	http.MethodHead,
	http.MethodPost,
	http.MethodPut,
	http.MethodPatch,
	http.MethodDelete,
	http.MethodConnect,
	http.MethodOptions,
	http.MethodTrace,
}

// MethodHTTP should only be used with '--mode=api' arg
func (method *Method) MethodHTTP() string {
	args := method.MetaArgs()
	if len(args) >= 2 {
		for _, httpMethod := range availableMethods {
			if toUpper(args[1]) == httpMethod {
				return httpMethod
			}
		}
	}
	return ""
}

var availableOperations = []string{
	sqlxOpExec,
	sqlxOpQuery,
}

// SqlxOperation should only be used with '--mode=sqlx' arg
func (method *Method) SqlxOperation() string {
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

// SqlxOptions should only be used with '--mode=sqlx' arg
func (method *Method) SqlxOptions() []string {
	args := method.MetaArgs()
	if len(args) >= 3 {
		opts := make([]string, 0, len(args[2:]))
		for _, opt := range args[2:] {
			opts = append(opts, toUpper(opt))
		}
		return opts
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

// ExtraScan should only be used with '--mode=api' arg
func (method *Method) ExtraScan() []string {
	if args := method.MetaArgs(); len(args) >= 3 {
		extra := make([]string, 0, 2)
		for _, arg := range args[2:] {
			if len(arg) > 6 && toUpper(arg[0:5]) == "SCAN(" && arg[len(arg)-1] == ')' {
				extra = append(extra, split(arg[5:len(arg)-1], ",")...)
			}
		}
		return extra
	}
	return nil
}

// SingleScan should only be used with '--mode=sqlx' arg
func (method *Method) SingleScan() string {
	if args := method.MetaArgs(); len(args) >= 3 {
		for _, opt := range args[2:] {
			if len(opt) > 6 && toUpper(opt[0:5]) == "SCAN(" && opt[len(opt)-1] == ')' {
				expressions := split(opt[5:len(opt)-1], ",")
				for _, expr := range expressions {
					return expr
				}
			}
		}
	}
	return ""
}

// WrapFunc should only be used with '--mode=sqlx' arg
func (method *Method) WrapFunc() string {
	const prefix = "WRAP="
	if args := method.MetaArgs(); len(args) >= 3 {
		for _, opt := range args[2:] {
			if len(opt) > len(prefix) && toUpper(opt[:len(prefix)]) == prefix {
				return opt[len(prefix):]
			}
		}
	}
	return ""
}

// IsolationLv should only be used with '--mode=sqlx' arg
func (method *Method) IsolationLv() string {
	const prefix = "ISOLATION="
	if args := method.MetaArgs(); len(args) >= 3 {
		for _, opt := range args[2:] {
			if len(opt) > len(prefix) && toUpper(opt[:len(prefix)]) == prefix {
				return opt[len(prefix):]
			}
		}
	}
	return ""
}

// TxIsolationLv should only be used with '--mode=sqlx' arg
func (method *Method) TxIsolationLv() string {
	const prefix = "ISOLATION="
	if args := method.MetaArgs(); len(args) >= 2 {
		for _, opt := range args[1:] {
			if len(opt) > len(prefix) && toUpper(opt[:len(prefix)]) == prefix {
				return opt[len(prefix):]
			}
		}
	}
	return ""
}

// ArgumentsVar should only be used with '--mode=sqlx' arg
func (method *Method) ArgumentsVar() string {
	const prefix = "ARGUMENTS="
	if args := method.MetaArgs(); len(args) >= 3 {
		for _, opt := range args[2:] {
			if len(opt) > len(prefix) && toUpper(opt[:len(prefix)]) == prefix {
				return opt[len(prefix):]
			}
		}
	}
	return ""
}

// ConstBindResult represents the result of parsing constbind expressions
type ConstBindResult struct {
	SQL  string   // SQL with ${...} replaced by ?
	Args []string // extracted expressions
}

// ParseConstBind parses the Header and extracts ${...} expressions,
// replacing them with ? placeholders. This is used with CONSTBIND option.
func (method *Method) ParseConstBind() (ConstBindResult, error) {
	return parseConstBindExpressions(method.Header)
}

// parseConstBindExpressions extracts ${...} expressions from the input string
// and replaces them with ? placeholders.
func parseConstBindExpressions(input string) (ConstBindResult, error) {
	var (
		result       []byte
		args         []string
		i            int
		singleQuoted bool
		doubleQuoted bool
	)

	for i < len(input) {
		ch := input[i]

		// Handle quote tracking (to avoid replacing ${...} inside string literals)
		if ch == '\'' && !doubleQuoted && (i == 0 || input[i-1] != '\\') {
			singleQuoted = !singleQuoted
			result = append(result, ch)
			i++
			continue
		}
		if ch == '"' && !singleQuoted && (i == 0 || input[i-1] != '\\') {
			doubleQuoted = !doubleQuoted
			result = append(result, ch)
			i++
			continue
		}

		// Look for ${ pattern outside of quotes
		if !singleQuoted && !doubleQuoted && ch == '$' && i+1 < len(input) && input[i+1] == '{' {
			startPos := i
			// Find the matching }
			depth := 1
			j := i + 2
			for j < len(input) && depth > 0 {
				switch input[j] {
				case '{':
					depth++
				case '}':
					depth--
				}
				j++
			}

			if depth != 0 {
				return ConstBindResult{}, fmt.Errorf("unclosed expression starting at position %d: %s", startPos, input[startPos:])
			}

			// Extract the expression (without ${ and })
			expr := trimSpace(input[i+2 : j-1])
			if expr == "" {
				return ConstBindResult{}, fmt.Errorf("empty expression at position %d", startPos)
			}
			args = append(args, expr)
			result = append(result, '?')
			i = j
			continue
		}

		result = append(result, ch)
		i++
	}

	return ConstBindResult{
		SQL:  string(result),
		Args: args,
	}, nil
}

// ReturnSlice should only be used with '--mode=api' arg
func (method *Method) ReturnSlice() bool {
	if args := method.MetaArgs(); len(args) >= 3 {
		for _, arg := range args[2:] {
			switch toUpper(arg) {
			case "ONE":
				return false
			case "MANY":
				return true
			}
		}
	}
	return len(method.Out) > 1 && isSlice(method.Out[0])
}

// MaxRetry should only be used with '--mode=api' arg
func (method *Method) MaxRetry() string {
	const prefix = "RETRY="
	if args := method.MetaArgs(); len(args) >= 3 {
		for _, arg := range args[2:] {
			if len(arg) > len(prefix) && toUpper(arg[:len(prefix)]) == prefix {
				return arg[len(prefix):]
			}
		}
	}
	// defaults to 2
	return "2"
}

// RequestOptions should only be used with '--mode=api' arg
func (method *Method) RequestOptions() string {
	if args := method.MetaArgs(); len(args) >= 3 {
		for _, arg := range args[2:] {
			if len(arg) > 9 && toUpper(arg[0:8]) == "OPTIONS(" && arg[len(arg)-1] == ')' {
				return arg[8 : len(arg)-1]
			}
		}
	}
	return ""
}

func inspectMethod(node *ast.Field, source []byte) (method *Method) {
	field := node
	method = new(Method)
	method.Source = source
	if field.Doc != nil {
		method.Meta = trimSlash(field.Doc.List[0].Text)
		var buffer bytes.Buffer
		for _, header := range field.Doc.List[1:] {
			buffer.WriteString(trimSlash(header.Text))
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
	if funcType, ok := field.Type.(*ast.FuncType); ok {
		inParams := funcType.Params.List
		method.In = make(map[string]ast.Expr, len(inParams))
		for _, param := range inParams {
			if param.Names != nil {
				for _, name := range param.Names {
					method.OrderedIn = append(method.OrderedIn, name.Name)
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
	}
	return method
}
