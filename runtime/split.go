package defc

import (
	"strings"

	tok "github.com/x5iu/defc/runtime/token"
)

func Count(sql string, ch string) (n int) {
	tokens := tok.SplitTokens(sql)
	for _, token := range tokens {
		if token == ch {
			n++
		}
	}
	return n
}

func Split(sql string, sep string) (group []string) {
	tokens := tok.SplitTokens(sql)
	group = make([]string, 0, len(tokens))
	last := 0
	for i, token := range tokens {
		if token == sep || i+1 == len(tokens) {
			if joint := tok.MergeSqlTokens(tokens[last : i+1]); len(strings.Trim(joint, sep)) > 0 {
				group = append(group, joint)
			}
			last = i + 1
		}
	}
	return group
}
