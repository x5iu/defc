package defc

import (
	"reflect"
	"testing"
)

func TestCount(t *testing.T) {
	type TestCase struct {
		Name  string
		Input string
		Token string
		N     int
	}
	var testcases = []*TestCase{
		{
			Name:  "?",
			Input: "? '\t?' \"\n?\" \r\n? \t?",
			Token: "?",
			N:     3,
		},
	}
	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			if n := Count(testcase.Input, testcase.Token); n != testcase.N {
				t.Errorf("count: %d != %d", n, testcase.N)
				return
			}
		})
	}
}

func TestSplit(t *testing.T) {
	type TestCase struct {
		Name   string
		Input  string
		Sep    string
		Expect []string
	}
	var testcases = []*TestCase{
		{
			Name:  "escape_quotes",
			Input: "part1;\r\n \\'part2;\r\n \"\\\"part3\";",
			Sep:   ";",
			Expect: []string{
				"part1 ;",
				"\\' part2 ;",
				"\"\\\"part3\" ;",
			},
		},
		{
			Name:  "separate_comma",
			Input: "part1, part2, part3",
			Sep:   ",",
			Expect: []string{
				"part1 ,",
				"part2 ,",
				"part3",
			},
		},
		{
			Name:  "comma_in_paren",
			Input: "(autoincrement,\n\t\t\tname);insert",
			Sep:   ";",
			Expect: []string{
				"( autoincrement , name ) ;",
				"insert",
			},
		},
		{
			Name:  "comma_query",
			Input: "select id, name from user where name in (?, ?);",
			Sep:   ";",
			Expect: []string{
				"select id , name from user where name in ( ? , ? ) ;",
			},
		},
		{
			Name:  "named_query",
			Input: "select id, name from user where id = :id and name = :name;",
			Sep:   ";",
			Expect: []string{
				"select id , name from user where id = :id and name = :name ;",
			},
		},
		{
			Name:  "comment_query",
			Input: "/* sqlcomment */ select id, name from user where id = :id and name = :name;",
			Sep:   ";",
			Expect: []string{
				"/* sqlcomment */ select id , name from user where id = :id and name = :name ;",
			},
		},
	}
	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			if splitStrings := Split(testcase.Input, testcase.Sep); !reflect.DeepEqual(splitStrings, testcase.Expect) {
				t.Errorf("split: %v != %v", splitStrings, testcase.Expect)
				return
			}
		})
	}
}

func TestSplitTokens(t *testing.T) {
	type TestCase struct {
		Name   string
		Input  string
		Expect []string
	}
	var testcases = []*TestCase{
		{
			Name:  "comma_token",
			Input: "autoincrement,\n\t\t\tname",
			Expect: []string{
				"autoincrement",
				",",
				"name",
			},
		},
		{
			Name:  "question_token",
			Input: "in(?,?);",
			Expect: []string{
				"in", "(", "?", ",", "?", ")", ";",
			},
		},
		{
			Name:  "comment_token",
			Input: "# // -- /* */",
			Expect: []string{
				"#", "/", "/", "-", "-", "/", "*", "*", "/",
			},
		},
		{
			Name:  "colon_token",
			Input: ":id, :name",
			Expect: []string{
				":", "id",
				",",
				":", "name",
			},
		},
	}
	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			if tokens := SplitTokens(testcase.Input); !reflect.DeepEqual(tokens, testcase.Expect) {
				t.Errorf("tokens: %v != %v", tokens, testcase.Expect)
				return
			}
		})
	}
}
