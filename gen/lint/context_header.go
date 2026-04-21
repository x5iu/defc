package lint

import (
	"go/token"
	"strings"
	"text/template/parse"
)

// lintHeader walks an HTTP header block (one "Name: value" per line)
// and flags any ActionNode in a value position that is not wrapped
// by the safe header helper. Actions in header *names* are always
// BLOCKER: dynamic header names defeat allow-listing.
func lintHeader(file string, base token.Position, src string) []Finding {
	if strings.TrimSpace(src) == "" {
		return nil
	}
	tree := parseSafe("header", src)
	if tree == nil {
		return nil
	}
	var out []Finding
	walkActions(tree.Root, func(act *parse.ActionNode) {
		off := int(act.Pos) - 1
		if off < 0 {
			off = 0
		}
		if off > len(src) {
			off = len(src)
		}
		kind := classifyHeader(src, off)
		if kind == "header.value" && actionIsSafe(act, safeHeader) {
			return
		}
		line, col := posForOffset(base, src, off)
		out = append(out, Finding{
			File:         file,
			Line:         line,
			Column:       col,
			Mode:         "api",
			Kind:         kind,
			Action:       actionText(act),
			Severity:     SevBlocker,
			SuggestedFix: suggestHeader(kind, act),
			Why:          "bare interpolation into HTTP headers permits CR/LF injection and header smuggling.",
		})
	})
	return out
}

// classifyHeader decides whether off falls in a header name or
// header value region. Rule: scan backwards to the start of the
// current line. If we encounter a ':' before hitting the line start,
// we are in the value; otherwise in the name.
func classifyHeader(src string, off int) string {
	i := off - 1
	for i >= 0 && src[i] != '\n' {
		if src[i] == ':' {
			return "header.value"
		}
		i--
	}
	return "header.name"
}

func suggestHeader(kind string, act *parse.ActionNode) string {
	name := exprName(act)
	switch kind {
	case "header.value":
		return "{{ header " + name + " }}"
	case "header.name":
		return "use a constant header name"
	}
	return "{{ header " + name + " }}"
}
