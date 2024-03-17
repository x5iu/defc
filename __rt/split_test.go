package __rt

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
			Name:  "1",
			Input: "part1;\r\n \\'part2;\r\n \"\\\"part3\";",
			Sep:   ";",
			Expect: []string{
				"part1 ;",
				"\\'part2 ;",
				"\"\\\"part3\" ;",
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
