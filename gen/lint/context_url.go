package lint

import (
	"go/token"
	"strings"
	"text/template/parse"
)

// lintURL walks a URL template (single line) and flags any
// ActionNode that is not one of the URL-safe helpers.
func lintURL(file string, base token.Position, src string) []Finding {
	if src == "" {
		return nil
	}
	tree := parseSafe("url", src)
	if tree == nil {
		return nil
	}
	var out []Finding
	walkActions(tree.Root, func(act *parse.ActionNode) {
		if actionIsSafe(act, safeURL) {
			return
		}
		off := int(act.Pos) - 1
		if off < 0 {
			off = 0
		}
		if off > len(src) {
			off = len(src)
		}
		kind := classifyURL(src, off)
		sev := SevBlocker
		if kind == "url.authority" || kind == "url.scheme" {
			// scheme/authority drift = BLOCKER; bare path/query
			// values are also BLOCKER in v1.45.
			sev = SevBlocker
		}
		line, col := posForOffset(base, src, off)
		out = append(out, Finding{
			File:         file,
			Line:         line,
			Column:       col,
			Mode:         "api",
			Kind:         kind,
			Action:       actionText(act),
			Severity:     sev,
			SuggestedFix: suggestURL(kind, act),
			Why:          "bare interpolation into URL permits SSRF or path/query smuggling; use pathseg/query or prefix with a constant scheme+host.",
		})
	})
	return out
}

func classifyURL(src string, off int) string {
	// everything before '://' = scheme; between '://' and first '/'
	// = authority; between first '/' and '?' = path; after '?' = query
	// (before '#'); after '#' = fragment.
	schemeEnd := strings.Index(src, "://")
	if schemeEnd >= 0 && off <= schemeEnd {
		return "url.scheme"
	}
	authStart := -1
	if schemeEnd >= 0 {
		authStart = schemeEnd + 3
	}
	firstSlash := -1
	if authStart >= 0 {
		if i := strings.Index(src[authStart:], "/"); i >= 0 {
			firstSlash = authStart + i
		}
	}
	if authStart >= 0 && off >= authStart && (firstSlash < 0 || off < firstSlash) {
		return "url.authority"
	}
	qMark := strings.Index(src, "?")
	hash := strings.Index(src, "#")
	if hash >= 0 && off >= hash {
		return "url.fragment"
	}
	if qMark >= 0 && off >= qMark {
		// inside query — distinguish key vs value by previous '=' or '&'
		j := off - 1
		for j >= qMark {
			if src[j] == '=' {
				return "url.query-value"
			}
			if src[j] == '&' || j == qMark {
				return "url.query-key"
			}
			j--
		}
		return "url.query-key"
	}
	return "url.path"
}

func suggestURL(kind string, act *parse.ActionNode) string {
	name := exprName(act)
	switch kind {
	case "url.path":
		return "{{ pathseg " + name + " }}"
	case "url.query-key", "url.query-value":
		return "{{ query " + name + " }}"
	case "url.scheme", "url.authority":
		return "use a constant scheme+host; pass variable parts via pathseg/query"
	}
	return "{{ pathseg " + name + " }}"
}
