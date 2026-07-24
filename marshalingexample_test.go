package lisp

import (
	"encoding/json"

	"github.com/jig/lisp/lib/call"
	"github.com/jig/lisp/types"
)

func LoadMarshalExample(ns types.EnvType) {
	call.Call(ns, new_marshalexample)
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
		Items: map[types.MalType]types.MalType{
			types.KW("a"): lec.Val.A,
			types.KW("b"): lec.Val.B,
		},
	}, nil
}

type LispMarshalExampleFactory struct {
	Type MarshalExample
}

func new_marshalexample() (types.MalType, error) {
	return LispMarshalExampleFactory{}, nil
}

func (lec LispMarshalExampleFactory) FromHashMap(_hm types.MalType) (types.MalType, error) {
	hm := _hm.(types.HashMap)
	ex := MarshalExample{
		A: hm.Items[types.KW("a")].(int),
		B: hm.Items[types.KW("b")].(string),
	}
	return LispMarshalExample{ex}, nil
}

func (lec LispMarshalExampleFactory) FromJSON(b []byte) (types.MalType, error) {
	if err := json.Unmarshal(b, &lec.Type); err != nil {
		return nil, err
	}
	return LispMarshalExample{
		Val: lec.Type,
	}, nil
}
