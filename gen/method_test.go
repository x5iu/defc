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
