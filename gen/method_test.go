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
	m = &Method{Meta: "Test Query One Scan(obj)"}
	if exScan := m.ExtraScan(); !reflect.DeepEqual(exScan, []string{"obj"}) {
		t.Errorf("method: %v != [obj]", exScan)
		return
	}
	if rtnSlice := m.ReturnSlice(); rtnSlice != false {
		t.Errorf("method: %v != false", rtnSlice)
		return
	}
}
