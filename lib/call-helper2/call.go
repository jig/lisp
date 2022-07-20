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

func Call(namespace types.EnvType, fIn interface{}, args ...string) {
	call(nil, namespace, fIn, args...)
}

func CallOverrideFN(namespace types.EnvType, overrideFN string, fIn interface{}, args ...string) {
	call(&overrideFN, namespace, fIn, args...)
}

func call(overrideFN *string, namespace types.EnvType, fIn interface{}, args ...string) {
	if len(args) > 1 {
		panic("invalid arguments in environment setup")
	}
	functionFullName := strings.ToLower(runtime.FuncForPC(reflect.ValueOf(fIn).Pointer()).Name())
	n := strings.LastIndex(functionFullName, ".")
	if len(functionFullName) == -1 {
		panic(fmt.Errorf("invalid function full name (name is %s)", runtime.FuncForPC(reflect.ValueOf(fIn).Pointer()).Name()))
	}
	packageName := functionFullName[:n]
	var functionName string
	if overrideFN != nil {
		functionName = *overrideFN
	} else {
		functionName = strings.Replace(functionFullName[n+1:], "_", "-", -1)
	}

	finType := reflect.TypeOf(fIn)
	finValue := reflect.ValueOf(fIn)
	outParams := finType.NumOut()
	inParams := finType.NumIn()
	contextRequired := false
	if inParams >= 1 && finType.In(0).Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) {
		contextRequired = true
	}

	var extCall externalCall
	switch finType.NumOut() {
	case 0:
		if contextRequired {
			extCall = func(ctx context.Context, args ...interface{}) (result interface{}, err error) {
				_recover(functionFullName, &err)
				return _nil_nil(finValue.Call(_args_ctx(ctx, inParams, args)))
			}
		} else {
			extCall = func(_ context.Context, args ...interface{}) (result interface{}, err error) {
				_recover(functionFullName, &err)
				return _nil_nil(finValue.Call(_args(inParams, args)))
			}
		}
	case 1:
		if contextRequired {
			extCall = func(ctx context.Context, args ...interface{}) (result interface{}, err error) {
				_recover(functionFullName, &err)
				return _nil_error(finValue.Call(_args_ctx(ctx, inParams, args)))
			}
		} else {
			extCall = func(_ context.Context, args ...interface{}) (result interface{}, err error) {
				_recover(functionFullName, &err)
				return _nil_error(finValue.Call(_args(inParams, args)))
			}
		}
	case 2:
		if contextRequired {
			extCall = func(ctx context.Context, args ...interface{}) (result interface{}, err error) {
				_recover(functionFullName, &err)
				return _result_error(finValue.Call(_args_ctx(ctx, inParams, args)))
			}
		} else {
			extCall = func(_ context.Context, args ...interface{}) (result interface{}, err error) {
				_recover(functionFullName, &err)
				return _result_error(finValue.Call(_args(inParams, args)))
			}
		}
	default:
		panic(fmt.Sprintf("%s: wrong number of results (%d instead of 2)", functionFullName, outParams))
	}

	namespace.Set(types.Symbol{Val: functionName}, extCall)
	_, err := namespace.Update(types.Symbol{Val: "_PACKAGES_"}, func(_hm types.MalType) (types.MalType, error) {
		if _hm == nil {
			_hm = types.HashMap{Val: make(map[string]types.MalType)}
		}
		hm := _hm.(types.HashMap)
		set, ok := hm.Val[packageName].(types.Set)
		if !ok {
			set = types.Set{Val: make(map[string]struct{})}
		}
		set.Val[functionName] = struct{}{}
		hm.Val[packageName] = set
		return hm, nil
	})
	if err != nil {
		panic(err)
	}
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
