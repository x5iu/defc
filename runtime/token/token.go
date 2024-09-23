package token

import (
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

const (
	Space = " "
)

const (
	Question  = "?"
	Comma     = ","
	Colon     = ":"
	Dollar    = "$"
	At        = "@"
	Dash      = "-"
	Div       = "/"
	Mul       = "*"
	Underline = "_"
)

type Lexer struct {
	Raw string
	idx int
	tok string
}

func (l *Lexer) Next() (next bool) {
	l.tok, next = l.parse()
	return next
}

func (l *Lexer) Token() string {
	return l.tok
}

func (l *Lexer) parse() (string, bool) {
	line := l.Raw

	var (
		singleQuoted bool
		doubleQuoted bool
		backQuoted   bool
		arg          []byte
	)

	for ; l.idx < len(line); l.idx++ {
		switch ch := line[l.idx]; ch {
		case ':', ';', ',', '(', ')', '[', ']', '{', '}', '.', '=', '?', '+', '-', '*', '/', '>', '<', '!', '~', '%', '@', '&', '|':
			if doubleQuoted || singleQuoted || backQuoted {
				arg = append(arg, ch)
			} else {
				if len(arg) > 0 {
					return string(arg), true
				}
				l.idx++
				return string(ch), true
			}
		case ' ', '\t', '\n', '\r':
			if doubleQuoted || singleQuoted || backQuoted {
				arg = append(arg, ch)
			} else if len(arg) > 0 {
				l.idx++
				return string(arg), true
			}
		case '"':
			if !(l.idx > 0 && line[l.idx-1] == '\\' || singleQuoted || backQuoted) {
				doubleQuoted = !doubleQuoted
			}
			arg = append(arg, ch)
			if !doubleQuoted {
				l.idx++
				return string(arg), true
			}
		case '\'':
			if !(l.idx > 0 && line[l.idx-1] == '\\' || doubleQuoted || backQuoted) {
				singleQuoted = !singleQuoted
			}
			arg = append(arg, ch)
			if !singleQuoted {
				l.idx++
				return string(arg), true
			}
		case '`':
			if !(l.idx > 0 && line[l.idx-1] == '\\' || singleQuoted || doubleQuoted) {
				backQuoted = !backQuoted
			}
			arg = append(arg, ch)
			if !backQuoted {
				l.idx++
				return string(arg), true
			}
		default:
			arg = append(arg, ch)
		}
	}

	if len(arg) > 0 {
		return string(arg), true
	}

	return "", false
}

func MergeSqlTokens(tokens []string) string {
	n := 0
	for _, token := range tokens {
		n += len(token) + 1
	}
	var merged strings.Builder
	if n > 0 {
		merged.Grow(n - 1)
	}
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		merged.WriteString(token)
		if i < len(tokens)-1 {
			func() {
				switch token {
				case Colon, At, Dollar:
					if i < len(tokens)-1 {
						if r, _ := utf8.DecodeRuneInString(tokens[i+1]); string(r) == Underline || unicode.IsLetter(r) {
							return
						}
					}
				case Dash:
					if i < len(tokens)-1 {
						switch next := tokens[i+1]; next {
						case Dash:
							return
						default:
						}
					}
				case Div:
					if i < len(tokens)-1 {
						switch next := tokens[i+1]; next {
						case Mul, Div:
							return
						default:
						}
					}
				case Mul:
					if i < len(tokens)-1 {
						switch next := tokens[i+1]; next {
						case Div:
							return
						default:
						}
					}
				default:
				}
				merged.WriteString(Space)
			}()
		}
	}
	return merged.String()
}

var (
	splitTokensCache = &sync.Map{}
	stringSlicePool  = &sync.Pool{
		New: func() any {
			return make([]string, 0, 16)
		},
	}
)

func getSplitTokensCache(line string) ([]string, bool) {
	value, exists := splitTokensCache.Load(line)
	if !exists {
		return nil, false
	}
	return value.([]string), true
}

func setSplitTokensCache(line string, tokens []string) {
	splitTokensCache.Store(line, tokens)
}

func getStringSlice() []string {
	return stringSlicePool.Get().([]string)
}

func putStringSlice(s []string) {
	s = s[:0]
	stringSlicePool.Put(s)
}

func SplitTokens(line string) (tokens []string) {
	tokens, exists := getSplitTokensCache(line)
	if exists {
		return tokens
	}
	tokens = getStringSlice()
	defer putStringSlice(tokens)
	l := Lexer{Raw: line}
	for l.Next() {
		tokens = append(tokens, l.Token())
	}
	setSplitTokensCache(line, tokens)
	return tokens
}
