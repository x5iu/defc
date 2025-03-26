package token

import (
	"fmt"
	"strings"

	lru "github.com/hashicorp/golang-lru/v2"
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

	index int
	atsep bool
	token string
}

func (l *Lexer) Next() (next bool) {
	l.token, next = l.parse()
	return next
}

func (l *Lexer) Token() string {
	return l.token
}

func (l *Lexer) parse() (string, bool) {
	line := l.Raw

	var (
		singleQuoted bool
		doubleQuoted bool
		backQuoted   bool
		arg          []byte
	)

	for ; l.index < len(line); l.index++ {
		switch ch := line[l.index]; ch {
		case ':', ';', ',', '(', ')', '[', ']', '{', '}', '.', '=', '?', '+', '-', '*', '/', '>', '<', '!', '~', '%', '@', '&', '|':
			if doubleQuoted || singleQuoted || backQuoted {
				if l.atsep {
					panic("in various quotation marks, `atsep` should not be set")
				}
				arg = append(arg, ch)
			} else {
				if len(arg) > 0 {
					if l.atsep {
						panic("when the symbol is immediately adjacent to other tokens, `atsep` should not be set")
					}
					return string(arg), true
				}
				if l.atsep {
					l.atsep = false
					return Space, true
				}
				l.index++
				return string(ch), true
			}
		case ' ', '\t', '\n', '\r':
			if doubleQuoted || singleQuoted || backQuoted {
				if l.atsep {
					panic("in various quotation marks, `atsep` should not be set")
				}
				arg = append(arg, ch)
			} else if len(arg) > 0 {
				if l.atsep {
					panic("this is the first encounter with a space, `atsep` should not be set")
				}
				l.atsep = true
				return string(arg), true
			} else {
				l.atsep = true
			}
		case '"':
			if !(l.index > 0 && line[l.index-1] == '\\' || singleQuoted || backQuoted) {
				if !doubleQuoted {
					if l.atsep {
						l.atsep = false
						return Space, true
					}
				}
				doubleQuoted = !doubleQuoted
			}
			arg = append(arg, ch)
			if !doubleQuoted {
				l.index++
				return string(arg), true
			}
		case '\'':
			if !(l.index > 0 && line[l.index-1] == '\\' || doubleQuoted || backQuoted) {
				if !singleQuoted {
					if l.atsep {
						l.atsep = false
						return Space, true
					}
				}
				singleQuoted = !singleQuoted
			}
			arg = append(arg, ch)
			if !singleQuoted {
				l.index++
				return string(arg), true
			}
		case '`':
			if !(l.index > 0 && line[l.index-1] == '\\' || singleQuoted || doubleQuoted) {
				if !backQuoted {
					if l.atsep {
						l.atsep = false
						return Space, true
					}
				}
				backQuoted = !backQuoted
			}
			arg = append(arg, ch)
			if !backQuoted {
				l.index++
				return string(arg), true
			}
		default:
			if l.atsep {
				l.atsep = false
				return Space, true
			}
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
		n += len(token)
	}
	var merged strings.Builder
	merged.Grow(n)
	for _, token := range tokens {
		merged.WriteString(token)
	}
	return merged.String()
}

var splitTokensCache *lru.TwoQueueCache[string, []string]

func init() {
	var err error
	if splitTokensCache, err = lru.New2Q[string, []string](1024); err != nil {
		panic(fmt.Errorf("failed to init lru cache: %w", err))
	}
}

func SplitTokens(line string) (tokens []string) {
	tokens, exists := splitTokensCache.Get(line)
	if exists {
		return tokens
	}
	l := Lexer{Raw: line}
	for l.Next() {
		tokens = append(tokens, l.Token())
	}
	splitTokensCache.Add(line, tokens)
	return tokens
}
