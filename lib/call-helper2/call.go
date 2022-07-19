package call2

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/jig/lisp/types"
)

type externalCall func(context.Context, ...interface{}) (interface{}, error)

func Call(namespace map[string]types.MalType, fIn interface{}, contextRequired bool, args ...string) {
	if len(args) > 1 {
		panic("invalid arguments in environment setup")
	}
	n := strings.Split(strings.ToLower(runtime.FuncForPC(reflect.ValueOf(fIn).Pointer()).Name()), "/")
	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
	namespaceName, fName, ok := strings.Cut(fFullName, "/")
	if !ok {
		panic(fmt.Errorf("%s: invalid function full name (name is %s)", fFullName, fName))
	}

	fName = strings.ReplaceAll(fName, "_", "-")
	fFullName = namespaceName + "/" + fName

	finType := reflect.TypeOf(fIn)
	finValue := reflect.ValueOf(fIn)
	inParams := finType.NumIn()
	outParams := finType.NumOut()

	var extCall externalCall
	switch finType.NumOut() {
	case 0:
		if contextRequired {
			extCall = func(ctx context.Context, args ...interface{}) (result interface{}, err error) {
				_recover(fFullName, &err)
				return _nil_nil(finValue.Call(_args_ctx(ctx, inParams, args)))
			}
		} else {
			extCall = func(_ context.Context, args ...interface{}) (result interface{}, err error) {
				_recover(fFullName, &err)
				return _nil_nil(finValue.Call(_args(inParams, args)))
			}
		}
	case 1:
		if contextRequired {
			extCall = func(ctx context.Context, args ...interface{}) (result interface{}, err error) {
				_recover(fFullName, &err)
				return _nil_error(finValue.Call(_args_ctx(ctx, inParams, args)))
			}
		} else {
			extCall = func(_ context.Context, args ...interface{}) (result interface{}, err error) {
				_recover(fFullName, &err)
				return _nil_error(finValue.Call(_args(inParams, args)))
			}
		}
	case 2:
		if contextRequired {
			extCall = func(ctx context.Context, args ...interface{}) (result interface{}, err error) {
				_recover(fFullName, &err)
				return _result_error(finValue.Call(_args_ctx(ctx, inParams, args)))
			}
		} else {
			extCall = func(_ context.Context, args ...interface{}) (result interface{}, err error) {
				_recover(fFullName, &err)
				return _result_error(finValue.Call(_args(inParams, args)))
			}
		}
	default:
		panic(fmt.Sprintf("%s: wrong number of results (%d instead of 2)", fFullName, outParams))
	}
	namespace[fName] = extCall
}

func _recover(fFullName string, err *error) {
	rerr := recover()
	if rerr != nil {
		*err = fmt.Errorf("%s: %s", fFullName, rerr)
	}
}

func _args_ctx(ctx context.Context, inParams int, args []interface{}) []reflect.Value {
	if len(args) != inParams-1 {
		panic(fmt.Sprintf("wrong number of arguments (%d instead of %d)", len(args), inParams))
	}

	in := make([]reflect.Value, inParams)
	in[0] = reflect.ValueOf(ctx)
	for k, param := range args {
		in[k+1] = reflect.ValueOf(param)
	}
	return in
}

func _args(inParams int, args []interface{}) []reflect.Value {
	if len(args) != inParams {
		panic(fmt.Sprintf("wrong number of arguments (%d instead of %d)", len(args), inParams))
	}

	in := make([]reflect.Value, inParams)
	for k, param := range args {
		in[k] = reflect.ValueOf(param)
	}
	return in
}

func _nil_nil(res []reflect.Value) (result interface{}, err error) {
	return nil, nil
}

func _nil_error(res []reflect.Value) (result interface{}, err error) {
	if res[0].Interface() == nil {
		return nil, nil
	}
	return nil, res[0].Interface().(error)
}

func _result_error(res []reflect.Value) (result interface{}, err error) {
	if res[1].Interface() == nil {
		return res[0].Interface(), nil
	}
	return res[0].Interface(), res[1].Interface().(error)
}
