package __rt

import (
	"errors"
	"reflect"
	"testing"
)

func TestNew(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		var boolVal bool
		newDefault := newType(reflect.TypeOf(boolVal))
		if yes, ok := newDefault.Interface().(bool); !ok || yes {
			t.Errorf("default: bool = %v", newDefault)
			return
		}
		reflect.ValueOf(&boolVal).Elem().Set(newDefault)
	})
	t.Run("pointer", func(t *testing.T) {
		newPointer := New[*string]()
		if newPointer == nil {
			t.Errorf("pointer: expects non-nil value, got nil")
			return
		}
		if *newPointer != "" {
			t.Errorf("pointer: *string != \"\"")
			return
		}
		*newPointer = "test"
	})
	t.Run("slice", func(t *testing.T) {
		newSlice := New[[]int]()
		if newSlice == nil || len(newSlice) != 0 || cap(newSlice) != 0 {
			t.Errorf("slice: []int = %v", newSlice)
			return
		}
		newSlice = append(newSlice, 1, 2, 3, 4)
	})
	t.Run("map", func(t *testing.T) {
		newMap := New[map[string]int]()
		if newMap == nil || len(newMap) != 0 {
			t.Errorf("map: map[string]int = %v", newMap)
			return
		}
		newMap["test"] = 0313
	})
	t.Run("chan", func(t *testing.T) {
		newChan := New[chan struct{}]()
		if newChan == nil {
			t.Errorf("chan: chan struct{} = %v", newChan)
			return
		}
		go func() { <-newChan }()
		newChan <- struct{}{}
		close(newChan)
	})
	t.Run("func", func(t *testing.T) {
		newFunc := New[func(int) float64]()
		if newFunc == nil {
			t.Errorf("func: func(int) float64 = nil")
			return
		}
		ret := newFunc(0313)
		if ret != 0.0 {
			t.Errorf("func: return %v != 0.0", ret)
			return
		}
	})
	t.Run("interface", func(t *testing.T) {
		newInterface := New[error]()
		if newInterface != nil {
			t.Errorf("interface: error = %v", newInterface)
			return
		}
		newInterface = errors.New("error")
	})
}
