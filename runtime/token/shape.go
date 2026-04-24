package token

import "strings"

type ShapeTokenKind int

const (
	ShapeWhitespace ShapeTokenKind = iota
	ShapeLineComment
	ShapeBlockComment
	ShapeSingleQuoted
	ShapeDoubleQuoted
	ShapeNumericLiteral
	ShapeUnquoted
)

type ShapeToken struct {
	Kind ShapeTokenKind
	Raw  string
}

func TokenizeShapeSQL(sql string) []ShapeToken {
	var out []ShapeToken
	type stKind int
	const (
		stNormal stKind = iota
		stSQuote
		stDQuote
		stLineCom
		stBlockCom
	)
	st := stNormal
	dquoteStart := -1
	squoteStart := -1
	sqlIdentByte := func(c byte) bool {
		return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
	}
	prevNotIdent := func(i int) bool {
		if i == 0 {
			return true
		}
		return !sqlIdentByte(sql[i-1])
	}
	nextNotIdent := func(i int) bool {
		if i >= len(sql) {
			return true
		}
		return !sqlIdentByte(sql[i])
	}
	isNumStart := func(i int) bool {
		c := sql[i]
		if c >= '0' && c <= '9' {
			return true
		}
		return c == '.' && i+1 < len(sql) && sql[i+1] >= '0' && sql[i+1] <= '9'
	}
	dquoteCloses := func(i int) bool {
		bs := 0
		for j := i - 1; j >= 0 && sql[j] == '\\'; j-- {
			bs++
		}
		return bs%2 == 0
	}
	emitUnquoted := func(c byte) {
		out = append(out, ShapeToken{Kind: ShapeUnquoted, Raw: string(c)})
	}
	for i := 0; i < len(sql); {
		c := sql[i]
		switch st {
		case stLineCom:
			start := i
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
			out = append(out, ShapeToken{Kind: ShapeLineComment, Raw: sql[start:i]})
			if i < len(sql) {
				i++
			}
			st = stNormal
			continue
		case stBlockCom:
			start := i
			for i < len(sql) {
				if sql[i] == '*' && i+1 < len(sql) && sql[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			out = append(out, ShapeToken{Kind: ShapeBlockComment, Raw: sql[start:i]})
			st = stNormal
			continue
		case stSQuote:
			if c == '\'' {
				if i+1 < len(sql) && sql[i+1] == '\'' {
					i += 2
					continue
				}
				out = append(out, ShapeToken{Kind: ShapeSingleQuoted, Raw: sql[squoteStart : i+1]})
				i++
				st = stNormal
				squoteStart = -1
				continue
			}
			if c == '\\' && i+1 < len(sql) {
				i += 2
				continue
			}
			i++
			continue
		case stDQuote:
			if c == '\\' && i+1 < len(sql) {
				i += 2
				continue
			}
			if c == '"' {
				if !dquoteCloses(i) {
					if i+1 < len(sql) {
						i += 2
					} else {
						i++
					}
					continue
				}
				if i+1 < len(sql) && sql[i+1] == '"' {
					i += 2
					continue
				}
				out = append(out, ShapeToken{Kind: ShapeDoubleQuoted, Raw: sql[dquoteStart : i+1]})
				i++
				st = stNormal
				dquoteStart = -1
				continue
			}
			i++
			continue
		case stNormal:
			if c == '-' && i+1 < len(sql) && sql[i+1] == '-' {
				i += 2
				st = stLineCom
				continue
			}
			if c == '/' && i+1 < len(sql) && sql[i+1] == '*' {
				i += 2
				st = stBlockCom
				continue
			}
			if c == '\'' {
				squoteStart = i
				i++
				st = stSQuote
				continue
			}
			if c == '"' {
				dquoteStart = i
				i++
				st = stDQuote
				continue
			}
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
				start := i
				i++
				for i < len(sql) {
					cc := sql[i]
					if cc != ' ' && cc != '\t' && cc != '\n' && cc != '\r' {
						break
					}
					i++
				}
				out = append(out, ShapeToken{Kind: ShapeWhitespace, Raw: sql[start:i]})
				continue
			}
			if isNumStart(i) && prevNotIdent(i) {
				j := i
				j++
				for j < len(sql) {
					cc := sql[j]
					if (cc >= '0' && cc <= '9') || cc == '.' {
						j++
						continue
					}
					if cc == 'e' || cc == 'E' {
						j++
						if j < len(sql) && (sql[j] == '+' || sql[j] == '-') {
							j++
						}
						continue
					}
					break
				}
				if nextNotIdent(j) {
					out = append(out, ShapeToken{Kind: ShapeNumericLiteral, Raw: sql[i:j]})
					i = j
					continue
				}
			}
			emitUnquoted(c)
			i++
		}
	}
	if st == stSQuote {
		if squoteStart >= 0 {
			out = append(out, ShapeToken{Kind: ShapeSingleQuoted, Raw: sql[squoteStart:]})
		} else {
			out = append(out, ShapeToken{Kind: ShapeSingleQuoted, Raw: "'"})
		}
	} else if st == stDQuote && dquoteStart >= 0 {
		out = append(out, ShapeToken{Kind: ShapeDoubleQuoted, Raw: sql[dquoteStart:]})
	}
	return out
}

func NormalizeShape(sql string) string {
	toks := TokenizeShapeSQL(sql)
	var b strings.Builder
	pendingSpace := false
	emitSpace := func() {
		if b.Len() > 0 {
			pendingSpace = true
		}
	}
	flushSpace := func() {
		if pendingSpace && b.Len() > 0 {
			b.WriteByte(' ')
		}
		pendingSpace = false
	}
	writeByte := func(c byte) {
		flushSpace()
		b.WriteByte(c)
	}
	writeStr := func(x string) {
		flushSpace()
		b.WriteString(x)
	}
	for _, t := range toks {
		switch t.Kind {
		case ShapeWhitespace:
			emitSpace()
		case ShapeLineComment, ShapeBlockComment:
			emitSpace()
		case ShapeSingleQuoted:
			writeStr("?")
		case ShapeDoubleQuoted:
			writeStr(t.Raw)
		case ShapeNumericLiteral:
			writeStr("?")
		case ShapeUnquoted:
			if len(t.Raw) != 1 {
				continue
			}
			c := t.Raw[0]
			if c >= 'A' && c <= 'Z' {
				writeByte(c + 32)
			} else {
				writeByte(c)
			}
		}
	}
	return strings.TrimSpace(b.String())
}
