package sqlx

import (
	"fmt"
	"testing"
)

func TestRebind(t *testing.T) {
	testCases := []struct {
		Name     string
		BindType int
		Query    string
		Want     string
	}{
		{
			Name:     "QuestionBindType",
			BindType: QUESTION,
			Query:    "SELECT * FROM users WHERE id = ?",
			Want:     "SELECT * FROM users WHERE id = ?",
		},
		{
			Name:     "UnknownBindType",
			BindType: UNKNOWN,
			Query:    "SELECT * FROM users WHERE id = ?",
			Want:     "SELECT * FROM users WHERE id = ?",
		},
		{
			Name:     "DollarBindType",
			BindType: DOLLAR,
			Query:    "SELECT * FROM users WHERE id = ? AND name = ?",
			Want:     "SELECT * FROM users WHERE id = $1 AND name = $2",
		},
		{
			Name:     "NamedBindType",
			BindType: NAMED,
			Query:    "SELECT * FROM users WHERE id = ? AND name = ?",
			Want:     "SELECT * FROM users WHERE id = :arg1 AND name = :arg2",
		},
		{
			Name:     "AtBindType",
			BindType: AT,
			Query:    "SELECT * FROM users WHERE id = ? AND name = ?",
			Want:     "SELECT * FROM users WHERE id = @p1 AND name = @p2",
		},
		{
			Name:     "ComplexQuery",
			BindType: DOLLAR,
			Query:    "SELECT * FROM users WHERE id IN (?, ?) AND name LIKE ? AND age > ?",
			Want:     "SELECT * FROM users WHERE id IN ($1, $2) AND name LIKE $3 AND age > $4",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			got := Rebind(tc.BindType, tc.Query)
			if got != tc.Want {
				t.Errorf("Rebind(%q, %q) = %q, want %q", bindTypeToString(tc.BindType), tc.Query, got, tc.Want)
			}
		})
	}
}

func bindTypeToString(bindType int) string {
	switch bindType {
	case QUESTION:
		return "?"
	case UNKNOWN:
		return "?"
	case DOLLAR:
		return "$"
	case NAMED:
		return ":"
	case AT:
		return "@"
	}
	panic(fmt.Sprintf("unknown bind type: %d", bindType))
}
