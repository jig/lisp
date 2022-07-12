package lisp

import (
	"encoding/json"

	"github.com/jig/lisp/lib/call-helper"
	"github.com/jig/lisp/types"
)

var NSMarshalExample = map[string]types.MalType{
	"new-marshalexample": call.Call0e(newLispMarshalExample),
}

type MarshalExample struct {
	A int    `json:"a"`
	B string `json:"b"`
}

type LispMarshalExample struct {
	Val MarshalExample
}

func (lec LispMarshalExample) MarshalHashMap() (types.MalType, error) {
	return types.HashMap{
		Val: map[string]types.MalType{
			"ʞa": lec.Val.A,
			"ʞb": lec.Val.B,
		},
	}, nil
}

type LispMarshalExampleFactory struct {
	Type MarshalExample
}

func newLispMarshalExample(a []types.MalType) (types.MalType, error) {
	return LispMarshalExampleFactory{}, nil
}

func (lec LispMarshalExampleFactory) FromHashMap(_hm types.MalType) (interface{}, error) {
	hm := _hm.(types.HashMap)
	ex := MarshalExample{
		A: hm.Val["ʞa"].(int),
		B: hm.Val["ʞb"].(string),
	}
	return LispMarshalExample{ex}, nil
}

func (lec LispMarshalExampleFactory) FromJSON(b []byte) (interface{}, error) {
	if err := json.Unmarshal(b, &lec.Type); err != nil {
		return nil, err
	}
	return LispMarshalExample{
		Val: lec.Type,
	}, nil
}
