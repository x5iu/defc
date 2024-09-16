package defc

import "strings"

func Count(sql string, ch string) (n int) {
	tokens := SplitTokens(sql)
	for _, token := range tokens {
		if token == ch {
			n++
		}
	}
	return n
}

func Split(sql string, sep string) (group []string) {
	tokens := SplitTokens(sql)
	group = make([]string, 0, len(tokens))
	last := 0
	for i, token := range tokens {
		if token == sep || i+1 == len(tokens) {
			if joint := mergeTokens(tokens[last : i+1]); len(strings.Trim(joint, sep)) > 0 {
				group = append(group, joint)
			}
			last = i + 1
		}
	}
	return group
}

func mergeTokens(tokens []string) string {
	var merged strings.Builder
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		merged.WriteString(token)
		if i < len(tokens)-1 {
			func() {
				switch token {
				case ":":
					return
				case "-":
					if i < len(tokens)-1 {
						switch next := tokens[i+1]; next {
						case "-":
							return
						default:
						}
					}
				case "/":
					if i < len(tokens)-1 {
						switch next := tokens[i+1]; next {
						case "*", "/":
							return
						default:
						}
					}
				case "*":
					if i < len(tokens)-1 {
						switch next := tokens[i+1]; next {
						case "/":
							return
						default:
						}
					}
				default:
				}
				merged.WriteString(" ")
			}()
		}
	}
	return merged.String()
}

func SplitTokens(line string) (tokens []string) {
	l := lexer{raw: line}
	for {
		if token, ok := l.next(); ok {
			tokens = append(tokens, token)
		} else {
			break
		}
	}
	return tokens
}

type lexer struct {
	raw string
	idx int
}

func (l *lexer) next() (string, bool) {
	line := l.raw

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
