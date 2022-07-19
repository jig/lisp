package call2

import (
	"context"
	"fmt"
	"reflect"
)

func Call(fin interface{}) func(context.Context, ...interface{}) (interface{}, error) {
	finType := reflect.TypeOf(fin)
	finValue := reflect.ValueOf(fin)
	inParams := finType.NumIn()
	outParams := finType.NumOut()

	switch finType.NumOut() {
	case 0:
		return func(_ context.Context, args ...interface{}) (result interface{}, err error) {
			malRecover(&err)
			return _nil_nil(finValue.Call(_args(inParams, args)))
		}
	case 1:
		return func(_ context.Context, args ...interface{}) (result interface{}, err error) {
			malRecover(&err)
			return _nil_error(finValue.Call(_args(inParams, args)))
		}
	case 2:
		return func(_ context.Context, args ...interface{}) (result interface{}, err error) {
			malRecover(&err)
			return _result_error(finValue.Call(_args(inParams, args)))
		}
	default:
		panic(fmt.Sprintf("wrong number of results (%d instead of 2)", outParams))
	}
}

func CallWithContext(fin interface{}) func(context.Context, ...interface{}) (interface{}, error) {
	finType := reflect.TypeOf(fin)
	finValue := reflect.ValueOf(fin)
	inParams := finType.NumIn()
	outParams := finType.NumOut()

	switch finType.NumOut() {
	case 0:
		return func(ctx context.Context, args ...interface{}) (result interface{}, err error) {
			malRecover(&err)
			return _nil_nil(finValue.Call(_args_ctx(ctx, inParams, args)))
		}
	case 1:
		return func(ctx context.Context, args ...interface{}) (result interface{}, err error) {
			malRecover(&err)
			return _nil_error(finValue.Call(_args_ctx(ctx, inParams, args)))
		}
	case 2:
		return func(ctx context.Context, args ...interface{}) (result interface{}, err error) {
			malRecover(&err)
			return _result_error(finValue.Call(_args_ctx(ctx, inParams, args)))
		}
	default:
		panic(fmt.Sprintf("wrong number of results (%d instead of 2)", outParams))
	}
}

func malRecover(err *error) {
	rerr := recover()
	if rerr != nil {
		*err = rerr.(error)
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
