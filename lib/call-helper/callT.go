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
	_call0e(namespace, namespaceName, strings.ReplaceAll(fName, "_", "-"), f)
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
	_call0e(namespace, namespaceName, strings.ReplaceAll(fName, "_", "-"), f)
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
	_call1e(namespace, namespaceName, strings.ReplaceAll(fName, "_", "-"), f)
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
	_call1e(namespace, namespaceName, strings.ReplaceAll(fName, "_", "-"), f)
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

// CallT1eC returns a function that checks checks number of arguments (2) and its type
func CallT1eC[T, R LispType](
	namespace map[string]types.MalType,
	f func(context.Context, T) (R, error),
) {
	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
	namespaceName, fName, ok := strings.Cut(fFullName, "/")
	if !ok {
		panic(fmt.Errorf("%s: invalid function full name", fFullName))
	}
	_call1eC(namespace, namespaceName, strings.ReplaceAll(fName, "_", "-"), f)
}

// CallTNO1eC returns a function that checks checks number of arguments (2) and its type
// and overrides its lisp name (instead of taking the Go name)
func CallTNO1eC[T, R LispType](
	namespace map[string]types.MalType,
	fName string,
	f func(context.Context, T) (R, error),
) {
	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
	namespaceName, _, ok := strings.Cut(fFullName, "/")
	if !ok {
		panic(fmt.Errorf("%s: invalid function full name ", fFullName))
	}
	_call1eC(namespace, namespaceName, strings.ReplaceAll(fName, "_", "-"), f)
}

func _call1eC[T, R LispType](
	namespace map[string]types.MalType,
	namespaceName, fName string,
	f func(context.Context, T) (R, error),
) {
	fFullName := namespaceName + "/" + fName
	namespace[fName] = func(ctx context.Context, args []types.MalType) (result types.MalType, err error) {
		defer malRecover(&err)
		if len(args) != 1 {
			return nil, fmt.Errorf("%s: wrong number of arguments (%d instead of 1)", fFullName, len(args))
		}
		argType0, ok := args[0].(T)
		if !ok {
			if args[0] != nil {
				return nil, fmt.Errorf("%s: first argument of type %T unsupported", fFullName, args[0])
			}
		}
		return f(ctx, argType0)
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
	_call2e(namespace, namespaceName, strings.ReplaceAll(fName, "_", "-"), f)
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
	_call2e(namespace, namespaceName, strings.ReplaceAll(fName, "_", "-"), f)
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

// CallT2eC returns a function that checks checks number of arguments (2) and its type
func CallT2eC[T0, T1, R LispType](
	namespace map[string]types.MalType,
	f func(context.Context, T0, T1) (R, error),
) {
	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
	namespaceName, fName, ok := strings.Cut(fFullName, "/")
	if !ok {
		panic(fmt.Errorf("%s: invalid function full name", fFullName))
	}
	_call2eC(namespace, namespaceName, strings.ReplaceAll(fName, "_", "-"), f)
}

// CallTNO2eC returns a function that checks checks number of arguments (2) and its type
// and overrides its lisp name (instead of taking the Go name)
func CallTNO2eC[T0, T1, R LispType](
	namespace map[string]types.MalType,
	fName string,
	f func(context.Context, T0, T1) (R, error),
) {
	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
	namespaceName, _, ok := strings.Cut(fFullName, "/")
	if !ok {
		panic(fmt.Errorf("%s: invalid function full name ", fFullName))
	}
	_call2eC(namespace, namespaceName, strings.ReplaceAll(fName, "_", "-"), f)
}

func _call2eC[T0, T1, R LispType](
	namespace map[string]types.MalType,
	namespaceName, fName string,
	f func(context.Context, T0, T1) (R, error),
) {
	fFullName := namespaceName + "/" + fName
	namespace[fName] = func(ctx context.Context, args []types.MalType) (result types.MalType, err error) {
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
		return f(ctx, argType0, argType1)
	}
}

// CallT3e returns a function that checks checks number of arguments (2) and its type
func CallT3e[T0, T1, T2, R LispType](
	namespace map[string]types.MalType,
	f func(T0, T1, T2) (R, error),
) {
	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
	namespaceName, fName, ok := strings.Cut(fFullName, "/")
	if !ok {
		panic(fmt.Errorf("%s: invalid function full name", fFullName))
	}
	_call3e(namespace, namespaceName, strings.ReplaceAll(fName, "_", "-"), f)
}

// CallTNO3e returns a function that checks checks number of arguments (2) and its type
// and overrides its lisp name (instead of taking the Go name)
func CallTNO3e[T0, T1, T2, R LispType](
	namespace map[string]types.MalType,
	fName string,
	f func(T0, T1, T2) (R, error),
) {
	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
	namespaceName, _, ok := strings.Cut(fFullName, "/")
	if !ok {
		panic(fmt.Errorf("%s: invalid function full name ", fFullName))
	}
	_call3e(namespace, namespaceName, strings.ReplaceAll(fName, "_", "-"), f)
}

func _call3e[T0, T1, T2, R LispType](
	namespace map[string]types.MalType,
	namespaceName, fName string,
	f func(T0, T1, T2) (R, error),
) {
	fFullName := namespaceName + "/" + fName
	namespace[fName] = func(_ context.Context, args []types.MalType) (result types.MalType, err error) {
		defer malRecover(&err)
		if len(args) != 3 {
			return nil, fmt.Errorf("%s: wrong number of arguments (%d instead of 3)", fFullName, len(args))
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
		argType2, ok := args[2].(T2)
		if !ok {
			if args[1] != nil {
				return nil, fmt.Errorf("%s: second argument of type %T unsupported", fName, args[2])
			}
		}
		return f(argType0, argType1, argType2)
	}
}

// CallT3eC returns a function that checks checks number of arguments (2) and its type
func CallT3eC[T0, T1, T2, R LispType](
	namespace map[string]types.MalType,
	f func(context.Context, T0, T1, T2) (R, error),
) {
	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
	namespaceName, fName, ok := strings.Cut(fFullName, "/")
	if !ok {
		panic(fmt.Errorf("%s: invalid function full name", fFullName))
	}
	_call3eC(namespace, namespaceName, strings.ReplaceAll(fName, "_", "-"), f)
}

// CallTNO3eC returns a function that checks checks number of arguments (2) and its type
// and overrides its lisp name (instead of taking the Go name)
func CallTNO3eC[T0, T1, T2, R LispType](
	namespace map[string]types.MalType,
	fName string,
	f func(context.Context, T0, T1, T2) (R, error),
) {
	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
	namespaceName, _, ok := strings.Cut(fFullName, "/")
	if !ok {
		panic(fmt.Errorf("%s: invalid function full name ", fFullName))
	}
	_call3eC(namespace, namespaceName, strings.ReplaceAll(fName, "_", "-"), f)
}

func _call3eC[T0, T1, T2, R LispType](
	namespace map[string]types.MalType,
	namespaceName, fName string,
	f func(context.Context, T0, T1, T2) (R, error),
) {
	fFullName := namespaceName + "/" + fName
	namespace[fName] = func(ctx context.Context, args []types.MalType) (result types.MalType, err error) {
		defer malRecover(&err)
		if len(args) != 3 {
			return nil, fmt.Errorf("%s: wrong number of arguments (%d instead of 3)", fFullName, len(args))
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
		argType2, ok := args[2].(T2)
		if !ok {
			if args[1] != nil {
				return nil, fmt.Errorf("%s: second argument of type %T unsupported", fName, args[2])
			}
		}
		return f(ctx, argType0, argType1, argType2)
	}
}

// CallTNe returns a function that checks checks number of arguments (2) and its type
func CallTNe[R LispType](
	namespace map[string]types.MalType,
	f func(...types.MalType) (R, error),
) {
	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
	namespaceName, fName, ok := strings.Cut(fFullName, "/")
	if !ok {
		panic(fmt.Errorf("%s: invalid function full name", fFullName))
	}
	_callNe(namespace, namespaceName, strings.ReplaceAll(fName, "_", "-"), f)
}

// CallTNONe returns a function that checks checks number of arguments (2) and its type
// and overrides its lisp name (instead of taking the Go name)
func CallTNONe[R LispType](
	namespace map[string]types.MalType,
	fName string,
	f func(...types.MalType) (R, error),
) {
	n := strings.Split(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "/")
	fFullName := strings.Replace(n[len(n)-1], ".", "/", 1)
	namespaceName, _, ok := strings.Cut(fFullName, "/")
	if !ok {
		panic(fmt.Errorf("%s: invalid function full name ", fFullName))
	}
	_callNe(namespace, namespaceName, strings.ReplaceAll(fName, "_", "-"), f)
}

func _callNe[R LispType](
	namespace map[string]types.MalType,
	namespaceName, fName string,
	f func(...types.MalType) (R, error),
) {
	// fFullName := namespaceName + "/" + fName
	namespace[fName] = func(_ context.Context, args []types.MalType) (result types.MalType, err error) {
		defer malRecover(&err)
		return f(args...)
	}
}
