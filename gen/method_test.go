package gen

import (
	"reflect"
	"testing"
)

func TestMethod(t *testing.T) {
	m := &Method{}
	if tmplUrl := m.TmplURL(); tmplUrl != "" {
		t.Errorf("method: %q != \"\"", tmplUrl)
		return
	}
	if sqlxOp := m.SqlxOperation(); sqlxOp != "" {
		t.Errorf("method: %q != \"\"", sqlxOp)
		return
	}
	if sqlxOpt := m.SqlxOptions(); sqlxOpt != nil {
		t.Errorf("method: %v != nil", sqlxOpt)
		return
	}
	if exScan := m.ExtraScan(); exScan != nil {
		t.Errorf("method: %v != nil", exScan)
		return
	}
	if wrapFn := m.WrapFunc(); wrapFn != "" {
		t.Errorf("method: %q != \"\"", wrapFn)
		return
	}
	if isoLv := m.IsolationLv(); isoLv != "" {
		t.Errorf("method: %q != \"\"", isoLv)
		return
	}
	if argVar := m.ArgumentsVar(); argVar != "" {
		t.Errorf("method: %q != \"\"", argVar)
		return
	}
	m = &Method{Meta: "Test Query One Scan(obj) wrap=fn isolation=sql.LevelDefault arguments=sqlArguments retry=3 options(reqOpts)"}
	if exScan := m.ExtraScan(); !reflect.DeepEqual(exScan, []string{"obj"}) {
		t.Errorf("method: %v != [obj]", exScan)
		return
	}
	if wrapFn := m.WrapFunc(); wrapFn != "fn" {
		t.Errorf("method: %q != \"fn\"", wrapFn)
		return
	}
	if isoLv := m.IsolationLv(); isoLv != "sql.LevelDefault" {
		t.Errorf("method: %q != \"sql.LevelDefault\"", isoLv)
		return
	}
	if rtnSlice := m.ReturnSlice(); rtnSlice != false {
		t.Errorf("method: %v != false", rtnSlice)
		return
	}
	if argVar := m.ArgumentsVar(); argVar != "sqlArguments" {
		t.Errorf("method: %q != \"sqlArguments\"", argVar)
		return
	}
	if maxRetry := m.MaxRetry(); maxRetry != "3" {
		t.Errorf("method: %q != \"3\"", maxRetry)
		return
	}
	if options := m.RequestOptions(); options != "reqOpts" {
		t.Errorf("method: %q != \"reqOpts\"", options)
		return
	}
}

func TestParseConstBindExpressions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantSQL  string
		wantArgs []string
		wantErr  bool
	}{
		{
			name:     "simple expression",
			input:    "SELECT * FROM user WHERE username = ${user.Name}",
			wantSQL:  "SELECT * FROM user WHERE username = ?",
			wantArgs: []string{"user.Name"},
		},
		{
			name:     "multiple expressions",
			input:    "SELECT * FROM user WHERE username = ${user.Name} AND age > ${user.Age}",
			wantSQL:  "SELECT * FROM user WHERE username = ? AND age > ?",
			wantArgs: []string{"user.Name", "user.Age"},
		},
		{
			name:     "simple variable expression",
			input:    "SELECT * FROM user WHERE name = ${name}",
			wantSQL:  "SELECT * FROM user WHERE name = ?",
			wantArgs: []string{"name"},
		},
		{
			name:     "expression inside sql string literal should not be replaced",
			input:    "SELECT * FROM user WHERE status = '${literal}'",
			wantSQL:  "SELECT * FROM user WHERE status = '${literal}'",
			wantArgs: []string{},
		},
		{
			name:     "expression inside double quoted string should not be replaced",
			input:    `SELECT * FROM user WHERE status = "${literal}"`,
			wantSQL:  `SELECT * FROM user WHERE status = "${literal}"`,
			wantArgs: []string{},
		},
		{
			name:     "nested braces in expression",
			input:    "SELECT * FROM user WHERE data = ${map[string]int{}}",
			wantSQL:  "SELECT * FROM user WHERE data = ?",
			wantArgs: []string{"map[string]int{}"},
		},
		{
			name:     "no expressions",
			input:    "SELECT * FROM user",
			wantSQL:  "SELECT * FROM user",
			wantArgs: []string{},
		},
		{
			name:     "unclosed expression",
			input:    "SELECT * FROM user WHERE id = ${name",
			wantSQL:  "",
			wantArgs: nil,
			wantErr:  true,
		},
		{
			name:     "empty expression",
			input:    "SELECT * FROM user WHERE id = ${}",
			wantSQL:  "",
			wantArgs: nil,
			wantErr:  true,
		},
		{
			name:     "expression with spaces",
			input:    "SELECT * FROM user WHERE id = ${ user.ID }",
			wantSQL:  "SELECT * FROM user WHERE id = ?",
			wantArgs: []string{"user.ID"},
		},
		{
			name:     "expression with function call",
			input:    "SELECT * FROM user WHERE created_at > ${time.Now().Add(-24 * time.Hour)}",
			wantSQL:  "SELECT * FROM user WHERE created_at > ?",
			wantArgs: []string{"time.Now().Add(-24 * time.Hour)"},
		},
		{
			name:     "mixed quoted and unquoted",
			input:    "SELECT * FROM user WHERE name = ${name} AND status = 'active' AND age = ${age}",
			wantSQL:  "SELECT * FROM user WHERE name = ? AND status = 'active' AND age = ?",
			wantArgs: []string{"name", "age"},
		},
		{
			name:     "literal expression",
			input:    "SELECT * FROM user WHERE name = ${\"X\"} AND status = 'active' AND age = ${18}",
			wantSQL:  "SELECT * FROM user WHERE name = ? AND status = 'active' AND age = ?",
			wantArgs: []string{"\"X\"", "18"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseConstBindExpressions(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if result.SQL != tt.wantSQL {
				t.Errorf("SQL = %q, want %q", result.SQL, tt.wantSQL)
			}
			if len(result.Args) == 0 && len(tt.wantArgs) == 0 {
				return
			}
			if !reflect.DeepEqual(result.Args, tt.wantArgs) {
				t.Errorf("Args = %v, want %v", result.Args, tt.wantArgs)
			}
		})
	}
}
