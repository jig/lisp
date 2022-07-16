package call

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/jig/lisp/types"
)

type LispType interface{ any }

// CallT0e returns a function that checks checks number of arguments (2) and its type
func CallT0e[R LispType](
	namespace map[string]types.MalType,
	f func() (R, error),
) {
	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
	namespaceName, fName, ok := strings.Cut(fFullName, "/")
	if !ok {
		panic(fmt.Errorf("%s: invalid function full name", fFullName))
	}
	_call0e(namespace, namespaceName, fName, f)
}

// CallTNO0e returns a function that checks checks number of arguments (2) and its type
// and overrides its lisp name (instead of taking the Go name)
func CallTNO0e[R LispType](
	namespace map[string]types.MalType,
	fName string,
	f func() (R, error),
) {
	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
	namespaceName, _, ok := strings.Cut(fFullName, "/")
	if !ok {
		panic(fmt.Errorf("%s: invalid function full name ", fFullName))
	}
	_call0e(namespace, namespaceName, fName, f)
}

func _call0e[R LispType](
	namespace map[string]types.MalType,
	namespaceName, fName string,
	f func() (R, error),
) {
	fFullName := namespaceName + "/" + fName
	namespace[fName] = func(
		_ context.Context, args []types.MalType) (result types.MalType, err error) {
		defer malRecover(&err)
		if len(args) != 0 {
			return nil, fmt.Errorf("%s: arguments not allowed (%d instead of 0)", fFullName, len(args))
		}
		return f()
	}
}

// CallT1e returns a function that checks checks number of arguments (2) and its type
func CallT1e[T, R LispType](
	namespace map[string]types.MalType,
	f func(T) (R, error),
) {
	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
	namespaceName, fName, ok := strings.Cut(fFullName, "/")
	if !ok {
		panic(fmt.Errorf("%s: invalid function full name", fFullName))
	}
	_call1e(namespace, namespaceName, fName, f)
}

// CallTNO1e returns a function that checks checks number of arguments (2) and its type
// and overrides its lisp name (instead of taking the Go name)
func CallTNO1e[T, R LispType](
	namespace map[string]types.MalType,
	fName string,
	f func(T) (R, error),
) {
	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
	namespaceName, _, ok := strings.Cut(fFullName, "/")
	if !ok {
		panic(fmt.Errorf("%s: invalid function full name ", fFullName))
	}
	_call1e(namespace, namespaceName, fName, f)
}

func _call1e[T, R LispType](
	namespace map[string]types.MalType,
	namespaceName, fName string,
	f func(T) (R, error),
) {
	fFullName := namespaceName + "/" + fName
	namespace[fName] = func(
		_ context.Context, args []types.MalType) (result types.MalType, err error) {
		defer malRecover(&err)
		if len(args) != 1 {
			return nil, fmt.Errorf("%s: wrong number of arguments (%d instead of 1)", fFullName, len(args))
		}
		argType0, ok := args[0].(T)
		if !ok {
			if args[0] != nil {
				return nil, fmt.Errorf("%s: argument of type %T unsupported", fFullName, args[0])
			}
		}
		return f(argType0)
	}
}

// CallT2e returns a function that checks checks number of arguments (2) and its type
func CallT2e[T0, T1, R LispType](
	namespace map[string]types.MalType,
	f func(T0, T1) (R, error),
) {
	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
	namespaceName, fName, ok := strings.Cut(fFullName, "/")
	if !ok {
		panic(fmt.Errorf("%s: invalid function full name", fFullName))
	}
	_call2e(namespace, namespaceName, fName, f)
}

// CallTNO2e returns a function that checks checks number of arguments (2) and its type
// and overrides its lisp name (instead of taking the Go name)
func CallTNO2e[T0, T1, R LispType](
	namespace map[string]types.MalType,
	fName string,
	f func(T0, T1) (R, error),
) {
	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
	namespaceName, _, ok := strings.Cut(fFullName, "/")
	if !ok {
		panic(fmt.Errorf("%s: invalid function full name ", fFullName))
	}
	_call2e(namespace, namespaceName, fName, f)
}

func _call2e[T0, T1, R LispType](
	namespace map[string]types.MalType,
	namespaceName, fName string,
	f func(T0, T1) (R, error),
) {
	fFullName := namespaceName + "/" + fName
	namespace[fName] = func(
		_ context.Context, args []types.MalType) (result types.MalType, err error) {
		defer malRecover(&err)
		if len(args) != 2 {
			return nil, fmt.Errorf("%s: wrong number of arguments (%d instead of 2)", fFullName, len(args))
		}
		argType0, ok := args[0].(T0)
		if !ok {
			if args[0] != nil {
				return nil, fmt.Errorf("%s: first argument of type %T unsupported", fFullName, args[0])
			}
		}
		argType1, ok := args[1].(T1)
		if !ok {
			if args[1] != nil {
				return nil, fmt.Errorf("%s: second argument of type %T unsupported", fName, args[1])
			}
		}
		return f(argType0, argType1)
	}
}

// package call

// import (
// 	"context"
// 	"fmt"
// 	"reflect"
// 	"runtime"
// 	"strings"

// 	"github.com/jig/lisp/types"
// )

// type LispType interface{ any }

// // CallT0e returns a function that checks checks number of arguments is 0
// func CallT0e[R LispType](
// 	namespace map[string]types.MalType,
// 	f func() (R, error),
// ) {
// 	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
// 	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
// 	namespaceName, fName, ok := strings.Cut(fFullName, "/")
// 	if !ok {
// 		panic(fmt.Errorf("%s: invalid function full name", fFullName))
// 	}
// 	_call0e(namespace, namespaceName, fName, f)
// }

// // CallTNO0e returns a function that checks checks number of arguments (0) and its type
// // and overrides its lisp name (instead of taking the Go name)
// func CallTNO0e[R LispType](
// 	namespace map[string]types.MalType,
// 	fName string,
// 	f func() (R, error),
// ) {
// 	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
// 	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
// 	namespaceName, _, ok := strings.Cut(fFullName, "/")
// 	if !ok {
// 		panic(fmt.Errorf("%s: cannot get namespace name out of ", fFullName))
// 	}
// 	_call0e(namespace, namespaceName, fName, f)
// }

// func _call0e[R LispType](
// 	namespace map[string]types.MalType,
// 	namespaceName, fName string,
// 	f func() (R, error),
// ) {
// 	fFullName := namespaceName + "/" + fName
// 	namespace[fName] = func(args []types.MalType, _ *context.Context) (result types.MalType, err error) {
// 		defer malRecover(&err)
// 		if len(args) != 0 {
// 			return nil, fmt.Errorf("%s: wrong number of arguments (%d instead of 0)", fFullName, len(args))
// 		}
// 		return f()
// 	}
// }

// // CallT0e returns a function that checks checks number of arguments (1) and its type
// func CallT0e[T, R LispType](
// 	namespace map[string]types.MalType,
// 	f func(T) (R, error),
// ) {
// 	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
// 	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
// 	namespaceName, fName, ok := strings.Cut(fFullName, "/")
// 	if !ok {
// 		panic(fmt.Errorf("%s: invalid function full name", fFullName))
// 	}
// 	_call1e(namespace, namespaceName, fName, f)
// }

// // CallT0eC
// func CallT0eC[T, R LispType](
// 	namespace map[string]types.MalType,
// 	f func(context.Context, T) (R, error),
// ) {
// 	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
// 	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
// 	namespaceName, fName, ok := strings.Cut(fFullName, "/")
// 	if !ok {
// 		panic(fmt.Errorf("%s: invalid function full name", fFullName))
// 	}
// 	_call1eC(namespace, namespaceName, fName, f)
// }

// // CallTNO1e returns a function that checks checks number of arguments (2) and its type
// // and overrides its lisp name (instead of taking the Go name)
// func CallTNO1e[T, R LispType](
// 	namespace map[string]types.MalType,
// 	fName string,
// 	f func(T) (R, error),
// ) {
// 	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
// 	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
// 	namespaceName, _, ok := strings.Cut(fFullName, "/")
// 	if !ok {
// 		panic(fmt.Errorf("%s: cannot get namespace name out of ", fFullName))
// 	}
// 	_call1e(namespace, namespaceName, fName, f)
// }

// func _call1e[T, R LispType](
// 	namespace map[string]types.MalType,
// 	namespaceName, fName string,
// 	f func(T) (R, error),
// ) {
// 	fFullName := namespaceName + "/" + fName
// 	namespace[fName] = func(args []types.MalType, _ *context.Context) (result types.MalType, err error) {
// 		defer malRecover(&err)
// 		if len(args) != 1 {
// 			return nil, fmt.Errorf("%s: wrong number of arguments (%d instead of 1)", fFullName, len(args))
// 		}
// 		argType, ok := args[0].(T)
// 		if !ok {
// 			return nil, fmt.Errorf("%s: argument of type %T unsupported", fFullName, args[0])
// 		}
// 		return f(argType)
// 	}
// }

// func _call1eC[T, R LispType](
// 	namespace map[string]types.MalType,
// 	namespaceName, fName string,
// 	f func(context.Context, T) (R, error),
// ) {
// 	fFullName := namespaceName + "/" + fName
// 	namespace[fName] = func(args []types.MalType, ctx *context.Context) (result types.MalType, err error) {
// 		defer malRecover(&err)
// 		if len(args) != 1 {
// 			return nil, fmt.Errorf("%s: wrong number of arguments (%d instead of 1)", fFullName, len(args))
// 		}
// 		argType, ok := args[0].(T)
// 		if !ok {
// 			return nil, fmt.Errorf("%s: argument of type %T unsupported", fFullName, args[0])
// 		}
// 		return f(*ctx, argType)
// 	}
// }

// // CallT1e returns a function that checks checks number of arguments (2) and its type
// func CallT1e[T0, T1, R LispType](
// 	namespace map[string]types.MalType,
// 	f func(T0, T1) (R, error),
// ) {
// 	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
// 	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
// 	namespaceName, fName, ok := strings.Cut(fFullName, "/")
// 	if !ok {
// 		panic(fmt.Errorf("%s: invalid function full name", fFullName))
// 	}
// 	_call2e(namespace, namespaceName, fName, f)
// }

// // CallTNO2e returns a function that checks checks number of arguments (2) and its type
// // and overrides its lisp name (instead of taking the Go name)
// func CallTNO2e[T0, T1, R LispType](
// 	namespace map[string]types.MalType,
// 	fName string,
// 	f func(T0, T1) (R, error),
// ) {
// 	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
// 	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
// 	namespaceName, _, ok := strings.Cut(fFullName, "/")
// 	if !ok {
// 		panic(fmt.Errorf("%s: cannot get namespace name out of ", fFullName))
// 	}
// 	_call2e(namespace, namespaceName, fName, f)
// }

// func _call2e[T0, T1, R LispType](
// 	namespace map[string]types.MalType,
// 	namespaceName, fName string,
// 	f func(T0, T1) (R, error),
// ) {
// 	fFullName := namespaceName + "/" + fName
// 	namespace[fName] = func(
// 		args []types.MalType, _ *context.Context) (result types.MalType, err error) {
// 		defer malRecover(&err)
// 		if len(args) != 2 {
// 			return nil, fmt.Errorf("%s: wrong number of arguments (%d instead of 2)", fFullName, len(args))
// 		}
// 		argType0, ok := args[0].(T0)
// 		if !ok {
// 			return nil, fmt.Errorf("%s: first argument of type %T unsupported", fFullName, args[0])
// 		}
// 		argType1, ok := args[1].(T1)
// 		if !ok {
// 			return nil, fmt.Errorf("%s: second argument of type %T unsupported", fName, args[1])
// 		}
// 		return f(argType0, argType1)
// 	}
// }

// func CallTOp1b[T LispType](
// 	namespace map[string]types.MalType,
// 	operatorName string,
// 	f func(T) bool,
// ) {
// 	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
// 	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
// 	namespaceName, _, ok := strings.Cut(fFullName, "/")
// 	fFullName = namespaceName + "/" + operatorName
// 	if !ok {
// 		panic(fmt.Errorf("%s: invalid function full name", fFullName))
// 	}
// 	namespace[operatorName] = func(args []types.MalType, _ *context.Context) (result types.MalType, err error) {
// 		defer malRecover(&err)
// 		if len(args) != 1 {
// 			return nil, fmt.Errorf("%s: wrong number of arguments (%d instead of 1)", operatorName, len(args))
// 		}
// 		argType, ok := args[0].(T)
// 		if !ok {
// 			// if not passed right type, return false instead of error
// 			return false, nil
// 			// return nil, fmt.Errorf("%s: argument of type %T unsupported", operatorName, args[0])
// 		}
// 		return f(argType), nil
// 	}
// }

// func CallTOp2b[T0, T0 LispType](
// 	namespace map[string]types.MalType,
// 	operatorName string,
// 	f func(T0, T0) bool,
// ) {
// 	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
// 	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
// 	namespaceName, _, ok := strings.Cut(fFullName, "/")
// 	fFullName = namespaceName + "/" + operatorName
// 	if !ok {
// 		panic(fmt.Errorf("%s: invalid function full name", fFullName))
// 	}
// 	namespace[operatorName] = func(args []types.MalType, _ *context.Context) (result types.MalType, err error) {
// 		defer malRecover(&err)
// 		if len(args) != 2 {
// 			return nil, fmt.Errorf("%s: wrong number of arguments (%d instead of 2)", operatorName, len(args))
// 		}

// 		argType0, ok := args[0].(T0)
// 		if !ok {
// 			return nil, fmt.Errorf("%s: first argument of type %T unsupported", operatorName, args[0])
// 		}
// 		argType1, ok := args[1].(T0)
// 		if !ok {
// 			return nil, fmt.Errorf("%s: second argument of type %T unsupported", operatorName, args[1])
// 		}
// 		return f(argType0, argType1), nil
// 	}
// }
