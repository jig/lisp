package call2

import (
	"reflect"
)

func Call(fin interface{}, args ...interface{}) (interface{}, error) {
	finType := reflect.TypeOf(fin)
	finValue := reflect.ValueOf(fin)

	inParams := finType.NumIn()
	in := make([]reflect.Value, inParams)
	for k, param := range args {
		in[k] = reflect.ValueOf(param)
	}
	res := finValue.Call(in)
	return res[0].Interface(), nil
}
