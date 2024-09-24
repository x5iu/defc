package defc

import (
	"database/sql"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

type implNotAnArg struct{}

func (implNotAnArg) NotAnArg() {}

type implToArgs struct{}

func (implToArgs) ToArgs() []any {
	return []any{
		"test",
		0314,
		true,
	}
}

type implToNamedArgs struct{}

func (implToNamedArgs) ToNamedArgs() map[string]any {
	return map[string]any{
		"s": "test",
		"i": 0315,
		"b": true,
	}
}

type nested struct {
	Nested string `db:"nested; charset=utf-8"`
}

func TestMergeArgs(t *testing.T) {
	type TestCase struct {
		Name   string
		Data   []any
		Expect []any
	}
	var testcases = []*TestCase{
		{
			Name:   "ints",
			Data:   []any{1, 2, 3},
			Expect: []any{1, 2, 3},
		},
		{
			Name:   "valuer",
			Data:   []any{sql.NullString{String: "test", Valid: true}},
			Expect: []any{sql.NullString{String: "test", Valid: true}},
		},
		{
			Name:   "list",
			Data:   []any{1, 2, []int{3, 4, 5}, [2]int{6, 7}, &implNotAnArg{}},
			Expect: []any{1, 2, 3, 4, 5, 6, 7},
		},
		{
			Name:   "naa",
			Data:   []any{&implNotAnArg{}},
			Expect: []any{},
		},
		{
			Name:   "nested",
			Data:   []any{1, 2, []any{3, []any{4, 5, &implToArgs{}}}},
			Expect: []any{1, 2, 3, 4, 5, "test", 0314, true},
		},
		{
			Name:   "bytes",
			Data:   []any{"test", []byte("test")},
			Expect: []any{"test", []byte("test")},
		},
	}
	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			merged := MergeArgs(testcase.Data...)
			if len(merged) != len(testcase.Expect) {
				t.Errorf("merge: %d != %d", len(merged), len(testcase.Expect))
				return
			}
			for i, arg := range merged {
				if !reflect.DeepEqual(arg, testcase.Expect[i]) {
					t.Errorf("merge: %v != %v", arg, testcase.Expect[i])
					return
				}
			}
		})
	}
}

func TestMergeNamedArgs(t *testing.T) {
	type TestCase struct {
		Name   string
		Data   map[string]any
		Expect map[string]any
	}
	var testcases = []*TestCase{
		{
			Name: "ints",
			Data: map[string]any{
				"one":   1,
				"two":   2,
				"three": 3,
			},
			Expect: map[string]any{
				"one":   1,
				"two":   2,
				"three": 3,
			},
		},
		{
			Name: "naa",
			Data: map[string]any{
				"naa": &implNotAnArg{},
			},
			Expect: map[string]any{},
		},
		{
			Name: "tna",
			Data: map[string]any{
				"tna": &implToNamedArgs{},
			},
			Expect: map[string]any{
				"s": "test",
				"i": 0315,
				"b": true,
			},
		},
		{
			Name: "args",
			Data: map[string]any{
				"args": &implToArgs{},
			},
			Expect: map[string]any{
				"args": &implToArgs{},
			},
		},
		{
			Name: "map",
			Data: map[string]any{
				"map": map[string]any{
					"one":   1,
					"two":   2,
					"three": 3,
				},
			},
			Expect: map[string]any{
				"one":   1,
				"two":   2,
				"three": 3,
			},
		},
		{
			Name: "valuer",
			Data: map[string]any{
				"valuer": sql.NullInt64{
					Int64: 0315,
					Valid: true,
				},
			},
			Expect: map[string]any{
				"valuer": sql.NullInt64{
					Int64: 0315,
					Valid: true,
				},
			},
		},
		{
			Name: "struct",
			Data: map[string]any{
				"struct": struct {
					One, Two, Three int
					Name            string `db:"name; charset=utf-8"`
					*nested
				}{
					One:   1,
					Two:   2,
					Three: 3,
					Name:  "test",
					nested: &nested{
						Nested: "nested",
					},
				},
			},
			Expect: map[string]any{
				"name":   "test",
				"nested": "nested",
			},
		},
	}
	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			merged := MergeNamedArgs(testcase.Data)
			if len(merged) != len(testcase.Expect) {
				t.Errorf("merge: %d != %d", len(merged), len(testcase.Expect))
				return
			}
			for k, v := range merged {
				if !reflect.DeepEqual(v, testcase.Expect[k]) {
					t.Errorf("merge: %v != %v", v, testcase.Expect[k])
					return
				}
			}
		})
	}
}

func TestBindVars(t *testing.T) {
	type TestCase struct {
		Name  string
		Input any
		N     int
	}
	output := func(testcase *TestCase) string {
		var bf strings.Builder
		for i := 0; i < testcase.N; i++ {
			if i > 0 {
				bf.WriteString(",")
			}
			bf.WriteString("?")
		}
		return bf.String()
	}
	test := func(testcases []*TestCase) func(*testing.T) {
		return func(t *testing.T) {
			for _, testcase := range testcases {
				t.Run(testcase.Name, func(t *testing.T) {
					if bindvars, expect := BindVars(testcase.Input), output(testcase); bindvars != expect {
						t.Errorf("%T: %q != %q", testcase.Input, bindvars, expect)
						return
					}
				})
			}
		}
	}
	t.Run("int", test([]*TestCase{
		{Name: "zero", Input: 0, N: 0},
		{Name: "int", Input: int(1), N: 1},
		{Name: "int8", Input: int8(2), N: 2},
		{Name: "int16", Input: int16(3), N: 3},
		{Name: "int32", Input: int32(4), N: 4},
		{Name: "int64", Input: int64(5), N: 5},
		{Name: "uint", Input: uint(1), N: 1},
		{Name: "uint8", Input: uint8(2), N: 2},
		{Name: "uint16", Input: uint16(3), N: 3},
		{Name: "uint32", Input: uint32(4), N: 4},
		{Name: "uint64", Input: uint64(5), N: 5},
	}))
	t.Run("slice", test([]*TestCase{
		{Name: "three", Input: []int{1, 2, 3}, N: 3},
		{Name: "zero", Input: []int{}, N: 0},
	}))
	t.Run("bytes", test([]*TestCase{
		{Name: "one", Input: []byte("test"), N: 1},
		{Name: "empty", Input: []byte{}, N: 1},
		{Name: "other", Input: json.RawMessage{}, N: 1},
	}))
	t.Run("nil", test([]*TestCase{
		{Name: "one", Input: nil, N: 1},
	}))
	t.Run("other", test([]*TestCase{
		{Name: "valuer", Input: sql.NullInt64{Int64: 0314, Valid: true}, N: 1},
	}))
}

func TestIn(t *testing.T) {
	type TestCase struct {
		Name   string
		Query  string
		Args   []any
		Expect string
		N      int
	}
	var testcases = []*TestCase{
		{
			Name:  "mixin",
			Query: "(?) (?) (?)",
			Args: []any{
				"test",
				[2]bool{true, false},
				&implToArgs{},
			},
			Expect: "(?) (?,?) (?,?,?)",
			N:      6,
		},
	}
	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			query, args, err := In(testcase.Query, testcase.Args)
			if err != nil {
				t.Errorf("in: %s", err)
				return
			}
			if query != testcase.Expect {
				t.Errorf("in: %q != %q", query, testcase.Expect)
				return
			}
			if len(args) != testcase.N {
				t.Errorf("in: %d != %d", len(args), testcase.N)
				return
			}
		})
	}
	t.Run("error", func(t *testing.T) {
		t.Run("more", func(t *testing.T) {
			_, _, err := In("?, ?", []any{1})
			if err == nil {
				t.Errorf("errors: expects errors, got nil")
				return
			}
			if err.Error() != "number of bind-vars exceeds arguments" {
				t.Errorf("errors: unexpected error message => %q", err.Error())
				return
			}
		})
		t.Run("less", func(t *testing.T) {
			_, _, err := In("?", []any{1, 2})
			if err == nil {
				t.Errorf("errors: expects errors, got nil")
				return
			}
			if err.Error() != "number of bind-vars less than number arguments" {
				t.Errorf("errors: unexpected error message => %q", err.Error())
				return
			}
		})
		t.Run("empty", func(t *testing.T) {
			_, _, err := In("?", []any{[]any{}})
			if err == nil {
				t.Errorf("errors: expects errors, got nil")
				return
			}
			if err.Error() != "empty slice passed to 'in' query" {
				t.Errorf("errors: unexpected error message => %q", err.Error())
				return
			}
		})
	})
}

func TestArguments(t *testing.T) {
	var arguments = make(Arguments, 0, 2)
	bindvars := arguments.Add([]int{1, 2, 3})
	if bindvars != "?,?,?" {
		t.Errorf("arguments: %q != \"?,?,?\"", bindvars)
		return
	}
	if l := len(arguments); l != 3 {
		t.Errorf("arguments: len(arguments) != 3, got %d", l)
		return
	}
	if !reflect.DeepEqual(arguments, Arguments{1, 2, 3}) {
		t.Errorf("arguments: %v != [1, 2, 3]", arguments)
		return
	}
}
