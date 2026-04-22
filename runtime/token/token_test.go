package token

import "testing"

func TestCountPlaceholders(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"SELECT ? WHERE a = 'a\\' AND x = ?", 2},
		{"SELECT ?, ? WHERE a = 'it''s' AND x = ?", 3},
	}
	for _, c := range cases {
		if got := CountPlaceholders(c.in); got != c.want {
			t.Errorf("CountPlaceholders(%q)=%d want %d", c.in, got, c.want)
		}
	}
}
