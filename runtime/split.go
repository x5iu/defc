package defc

import "strings"

func Count(sql string, ch string) (n int) {
	tokens := splitTokens(sql)
	for _, token := range tokens {
		if token == ch {
			n++
		}
	}
	return n
}

func Split(sql string, sep string) (group []string) {
	tokens := splitTokens(sql)
	group = make([]string, 0, len(tokens))
	last := 0
	for i, token := range tokens {
		if token == sep || i+1 == len(tokens) {
			if joint := strings.Join(tokens[last:i+1], " "); len(strings.Trim(joint, sep)) > 0 {
				group = append(group, joint)
			}
			last = i + 1
		}
	}
	return group
}

func splitTokens(line string) (tokens []string) {
	var (
		singleQuoted bool
		doubleQuoted bool
		arg          []byte
	)

	for i := 0; i < len(line); i++ {
		switch ch := line[i]; ch {
		case ';', '?':
			if doubleQuoted || singleQuoted {
				arg = append(arg, ch)
			} else {
				if len(arg) > 0 {
					tokens = append(tokens, string(arg))
				}
				tokens = append(tokens, string(ch))
				arg = arg[:0]
			}
		case ' ', '\t', '\n', '\r':
			if doubleQuoted || singleQuoted {
				arg = append(arg, ch)
			} else if len(arg) > 0 {
				tokens = append(tokens, string(arg))
				arg = arg[:0]
			}
		case '"':
			if !(i > 0 && line[i-1] == '\\' || singleQuoted) {
				doubleQuoted = !doubleQuoted
			}
			arg = append(arg, ch)
		case '\'':
			if !(i > 0 && line[i-1] == '\\' || doubleQuoted) {
				singleQuoted = !singleQuoted
			}
			arg = append(arg, ch)
		default:
			arg = append(arg, ch)
		}
	}

	if len(arg) > 0 {
		tokens = append(tokens, string(arg))
	}

	return tokens
}
