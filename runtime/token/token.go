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

// CountPlaceholders counts `?` positional placeholders in a SQL
// string, skipping those inside single-quoted literals, double-quoted
// identifiers, backtick-quoted identifiers, `--` line comments, and
// `/* ... */` block comments. Intended for [SQLArityCheck] equivalents
// outside this package; standard SQL escape (doubled quote inside a
// literal) is honored.
//
// String-literal scanning does NOT honor backslash escapes. This matches
// SQL:standard and Postgres default (standard_conforming_strings=on).
// Dialects that use backslash escaping (legacy MySQL) should quote-double
// the single quote instead.
//
// Placeholder counting is intentionally unaware of `:name` / `$N`
// styles — the defc sqlx pipeline emits `?` everywhere before rebind.
func CountPlaceholders(s string) int {
	var (
		count  int
		single bool
		double bool
		back   bool
		line   bool // -- ... \n
		block  bool // /* ... */
	)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case line:
			if c == '\n' {
				line = false
			}
		case block:
			if c == '*' && i+1 < len(s) && s[i+1] == '/' {
				block = false
				i++
			}
		case single:
			if c == '\'' {
				if i+1 < len(s) && s[i+1] == '\'' {
					i++
					continue
				}
				single = false
			}
		case double:
			if c == '"' {
				if i+1 < len(s) && s[i+1] == '"' {
					i++
					continue
				}
				double = false
			}
		case back:
			if c == '`' {
				back = false
			}
		default:
			switch c {
			case '\'':
				single = true
			case '"':
				double = true
			case '`':
				back = true
			case '-':
				if i+1 < len(s) && s[i+1] == '-' {
					line = true
					i++
				}
			case '/':
				if i+1 < len(s) && s[i+1] == '*' {
					block = true
					i++
				}
			case '?':
				count++
			}
		}
	}
	return count
}
