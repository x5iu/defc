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
	if argVar := m.ArgumentsVar(); argVar != "" {
		t.Errorf("method: %q != \"\"", argVar)
		return
	}
	m = &Method{Meta: "Test Query One Scan(obj) wrap=fn arguments=sqlArguments"}
	if exScan := m.ExtraScan(); !reflect.DeepEqual(exScan, []string{"obj"}) {
		t.Errorf("method: %v != [obj]", exScan)
		return
	}
	if wrapFn := m.WrapFunc(); wrapFn != "fn" {
		t.Errorf("method: %q != \"fn\"", wrapFn)
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
}
