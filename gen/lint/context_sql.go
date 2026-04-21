package lint

import (
	"go/token"
	"strings"
	"text/template/parse"
)

// lintSQL walks a SQL template and flags any ActionNode that is not
// one of the approved safe helpers (bind/bindvars/identifier).
func lintSQL(file string, base token.Position, src string) []Finding {
	tree := parseSafe("sql", src)
	if tree == nil {
		return nil
	}
	var out []Finding
	walkActions(tree.Root, func(act *parse.ActionNode) {
		if actionIsSafe(act, safeSQL) {
			return
		}
		// Classify by preceding bytes to assign a sub-kind.
		off := int(act.Pos) - 1 // positions are 1-based
		if off < 0 {
			off = 0
		}
		if off > len(src) {
			off = len(src)
		}
		kind := classifySQL(src, off)
		line, col := posForOffset(base, src, off)
		out = append(out, Finding{
			File:         file,
			Line:         line,
			Column:       col,
			Mode:         "sqlx",
			Kind:         kind,
			Action:       actionText(act),
			Severity:     SevBlocker,
			SuggestedFix: suggestSQL(kind, act),
			Why:          "bare interpolation into SQL context permits injection; use bind/bindvars for values or identifier for object names.",
		})
	})
	return out
}

// classifySQL returns "sql.value" or "sql.identifier" depending on
// the surrounding tokens at off within src. It is a lightweight
// heuristic: inside single-quotes => literal (sql.literal); directly
// after FROM/JOIN/UPDATE/INTO/TABLE keywords => identifier; inside
// /* */ or -- run => comment; otherwise value.
func classifySQL(src string, off int) string {
	// scan from start of src to off to track quote/comment state
	inSingle := false
	inDouble := false
	inLineCmt := false
	inBlockCmt := false
	for i := 0; i < off; i++ {
		c := src[i]
		if inLineCmt {
			if c == '\n' {
				inLineCmt = false
			}
			continue
		}
		if inBlockCmt {
			if c == '*' && i+1 < off && src[i+1] == '/' {
				inBlockCmt = false
				i++
			}
			continue
		}
		if inSingle {
			if c == '\'' {
				inSingle = false
			}
			continue
		}
		if inDouble {
			if c == '"' {
				inDouble = false
			}
			continue
		}
		switch c {
		case '\'':
			inSingle = true
		case '"':
			inDouble = true
		case '-':
			if i+1 < off && src[i+1] == '-' {
				inLineCmt = true
				i++
			}
		case '/':
			if i+1 < off && src[i+1] == '*' {
				inBlockCmt = true
				i++
			}
		}
	}
	if inLineCmt || inBlockCmt {
		return "sql.comment"
	}
	if inSingle || inDouble {
		return "sql.literal"
	}
	// Look backwards for a keyword that implies identifier context.
	// Find the last word before off.
	j := off - 1
	for j >= 0 && (src[j] == ' ' || src[j] == '\t' || src[j] == '\n' || src[j] == '\r') {
		j--
	}
	end := j + 1
	for j >= 0 && isWordByte(src[j]) {
		j--
	}
	word := strings.ToUpper(src[j+1 : end])
	switch word {
	case "FROM", "JOIN", "UPDATE", "INTO", "TABLE":
		return "sql.identifier"
	}
	return "sql.value"
}

func isWordByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

func suggestSQL(kind string, act *parse.ActionNode) string {
	name := exprName(act)
	switch kind {
	case "sql.identifier":
		return "{{ identifier " + name + " }}"
	case "sql.value", "sql.literal":
		return "{{ bind " + name + " }}"
	case "sql.comment":
		return "remove interpolation from SQL comment or move elsewhere"
	}
	return "{{ bind " + name + " }}"
}

// exprName returns the source text of the first command of act, used
// when suggesting a helper wrap.
func exprName(act *parse.ActionNode) string {
	if act == nil || act.Pipe == nil || len(act.Pipe.Cmds) == 0 {
		return "."
	}
	return act.Pipe.Cmds[0].String()
}
