package lisp

import (
	"context"
	"errors"
	"fmt"

	. "github.com/jig/lisp/env"
	"github.com/jig/lisp/printer"
	"github.com/jig/lisp/reader"
	"github.com/jig/lisp/types"
	. "github.com/jig/lisp/types"
)

// read
func READ(str string, cursor *Position) (MalType, error) {
	return reader.Read_str(str, cursor)
}

// eval
func starts_with(xs []MalType, sym string) bool {
	if 0 < len(xs) {
		switch s := xs[0].(type) {
		case Symbol:
			return s.Val == sym
		default:
		}
	}
	return false
}

func qq_loop(xs []MalType) MalType {
	acc := NewList()
	for i := len(xs) - 1; 0 <= i; i -= 1 {
		elt := xs[i]
		switch e := elt.(type) {
		case List:
			if starts_with(e.Val, "splice-unquote") {
				acc = NewList(Symbol{Val: "concat"}, e.Val[1], acc)
				continue
			}
		default:
		}
		acc = NewList(Symbol{Val: "cons"}, quasiquote(elt), acc)
	}
	return acc
}

func quasiquote(ast MalType) MalType {
	switch a := ast.(type) {
	case Vector:
		return NewList(Symbol{Val: "vec"}, qq_loop(a.Val))
	case HashMap, Symbol:
		return NewList(Symbol{Val: "quote"}, ast)
	case List:
		if starts_with(a.Val, "unquote") {
			return a.Val[1]
		} else {
			return qq_loop(a.Val)
		}
	default:
		return ast
	}
}

func is_macro_call(ast MalType, env EnvType) bool {
	if List_Q(ast) {
		slc, _ := GetSlice(ast)
		if len(slc) == 0 {
			return false
		}
		a0 := slc[0]
		if Symbol_Q(a0) && env.Find(a0.(Symbol)) != nil {
			mac, e := env.Get(a0.(Symbol))
			if e != nil {
				return false
			}
			if MalFunc_Q(mac) {
				return mac.(MalFunc).GetMacro()
			}
		}
	}
	return false
}

func macroexpand(ast MalType, env EnvType, ctx *context.Context) (MalType, error) {
	var mac MalType
	var e error
	for is_macro_call(ast, env) {
		slc, _ := GetSlice(ast)
		a0 := slc[0]
		mac, e = env.Get(a0.(Symbol))
		if e != nil {
			return nil, e
		}
		fn := mac.(MalFunc)
		ast, e = Apply(fn, slc[1:], ctx)
		if e != nil {
			return nil, e
		}
	}
	return ast, nil
}

func eval_ast(ast MalType, env EnvType, ctx *context.Context) (MalType, error) {
	//fmt.Printf("eval_ast: %#v\n", ast)
	if Symbol_Q(ast) {
		value, err := env.Get(ast.(Symbol))
		if err != nil {
			return nil, PushError(ast.(Symbol).Cursor, ast, err)
		}
		return value, nil
	} else if List_Q(ast) {
		lst := []MalType{}
		for _, a := range ast.(List).Val {
			exp, e := EVAL(a, env, ctx)
			if e != nil {
				if a, ok := a.(List); ok {
					return nil, PushError(a.Cursor, a, e)
				}
				return nil, PushError(nil, a, e)
			}
			lst = append(lst, exp)
		}
		return List{Val: lst}, nil
	} else if Vector_Q(ast) {
		lst := []MalType{}
		for _, a := range ast.(Vector).Val {
			exp, e := EVAL(a, env, ctx)
			if e != nil {
				return nil, PushError(ast.(Vector).Cursor, ast, e)
			}
			lst = append(lst, exp)
		}
		return Vector{Val: lst}, nil
	} else if HashMap_Q(ast) {
		m := ast.(HashMap)
		new_hm := HashMap{Val: map[string]MalType{}}
		for k, v := range m.Val {
			kv, e2 := EVAL(v, env, ctx)
			if e2 != nil {
				return nil, PushError(ast.(HashMap).Cursor, ast, e2)
			}
			new_hm.Val[k] = kv
		}
		return new_hm, nil
	} else {
		return ast, nil
	}
}

func EVAL(ast MalType, env EnvType, ctx *context.Context) (MalType, error) {
	var e error
	for {
		if ctx != nil {
			select {
			case <-(*ctx).Done():
				return nil, errors.New("timeout while evaluating expression")
			default:
			}
		}

		switch ast.(type) {
		case List: // continue
		default:
			return eval_ast(ast, env, ctx)
		}

		if env.Trace() {
			fmt.Printf("> %v\n", printer.Pr_str(ast, true))
		}

		// apply list
		ast, e = macroexpand(ast, env, ctx)
		if e != nil {
			return nil, e
		}
		if !List_Q(ast) {
			return eval_ast(ast, env, ctx)
		}
		if len(ast.(List).Val) == 0 {
			return ast, nil
		}

		a0 := ast.(List).Val[0]
		var a1 MalType
		var a2 MalType
		switch len(ast.(List).Val) {
		case 1:
			a1 = nil
			a2 = nil
		case 2:
			a1 = ast.(List).Val[1]
			a2 = nil
		default:
			a1 = ast.(List).Val[1]
			a2 = ast.(List).Val[2]
		}
		a0sym := "__<*fn*>__"
		if Symbol_Q(a0) {
			a0sym = a0.(Symbol).Val
		}
		switch a0sym {
		case "def!":
			res, e := EVAL(a2, env, ctx)
			if e != nil {
				return nil, PushError(ast.(List).Cursor, ast, e)
			}
			switch a1 := a1.(type) {
			case Symbol:
				return env.Set(a1, res), nil
			default:
				return nil, RuntimeError{
					ErrorVal: fmt.Errorf("cannot use '%T' as identifier", a1),
					Cursor:   ast.(List).Cursor,
				}
			}
		case "let*":
			let_env, e := NewEnv(env, nil, nil)
			if e != nil {
				return nil, e
			}
			arr1, e := GetSlice(a1)
			if e != nil {
				return nil, e
			}
			if len(arr1)%2 != 0 {
				return nil, RuntimeError{
					ErrorVal: errors.New("let*: odd elements on binding vector"),
					Cursor:   a1.(Vector).Cursor,
				}
			}
			for i := 0; i < len(arr1); i += 2 {
				if !Symbol_Q(arr1[i]) {
					return nil, RuntimeError{
						ErrorVal: errors.New("non-symbol bind value"),
						Cursor:   a1.(Vector).Cursor,
					}
				}
				exp, e := EVAL(arr1[i+1], let_env, ctx)
				if e != nil {
					return nil, PushError(arr1[i].(Symbol).Cursor, arr1[i], e)
				}
				let_env.Set(arr1[i].(Symbol), exp)
			}
			// ast = a2
			// env = let_env
			lst := ast.(List).Val
			if len(lst) == 2 {
				return nil, nil
			}
			if _, e := eval_ast(List{Val: lst[2 : len(lst)-1]}, let_env, ctx); e != nil {
				return nil, e
			}
			ast = lst[len(lst)-1]
			env = let_env
		case "quote": // '
			return a1, nil
		case "quasiquoteexpand":
			return quasiquote(a1), nil
		case "quasiquote": // `
			ast = quasiquote(a1)
		case "defmacro!":
			fn, e := EVAL(a2, env, ctx)
			fn = fn.(MalFunc).SetMacro()
			if e != nil {
				return nil, e
			}
			return env.Set(a1.(Symbol), fn), nil
		case "macroexpand":
			return macroexpand(a1, env, ctx)
		case "try*":
			var exc MalType
			exp, e := func() (res MalType, err error) {
				defer malRecover(&err)
				return EVAL(a1, env, ctx)
			}()
			if e == nil {
				return exp, nil
			} else {
				if a2 != nil && List_Q(a2) {
					a2s, _ := GetSlice(a2)
					if Symbol_Q(a2s[0]) && (a2s[0].(Symbol).Val == "catch*") {
						switch e := e.(type) {
						case MalError:
							exc = e.Obj
						case RuntimeError:
							exc = e.Error()
						default:
							exc = e.Error()
						}
						binds := NewList(a2s[1])
						new_env, e := NewEnv(env, binds, NewList(exc))
						if e != nil {
							return nil, e
						}
						exp, e = EVAL(a2s[2], new_env, ctx)
						if e == nil {
							return exp, nil
						}
					}
					return nil, e
				}
				return nil, e
			}
		case "context*":
			if a2 != nil {
				return nil, RuntimeError{
					ErrorVal: fmt.Errorf("context* does not allow more than one argument"),
					Cursor:   a2.(Vector).Cursor,
				}
			}
			childCtx, cancel := context.WithCancel(*ctx)
			exp, e := func() (res MalType, err error) {
				defer cancel()
				defer malRecover(&err)
				return EVAL(a1, env, &childCtx)
			}()
			if e != nil {
				return nil, e
			}
			return exp, nil
		case "trace":
			if a2 != nil {
				return nil, RuntimeError{
					ErrorVal: fmt.Errorf("trace does not allow more than one argument"),
					Cursor:   a2.(Vector).Cursor,
				}
			}
			exp, e := func() (res MalType, err error) {
				newEnv, e := NewEnv(env, nil, nil)
				if err != nil {
					return nil, e
				}
				newEnv.SetTrace(true)
				defer malRecover(&err)
				return EVAL(a1, newEnv, ctx)
			}()
			if e != nil {
				return nil, e
			}
			return exp, nil
		case "do":
			lst := ast.(List).Val
			if len(lst) == 1 {
				return nil, nil
			}
			if _, e := eval_ast(List{Val: lst[1 : len(lst)-1]}, env, ctx); e != nil {
				return nil, PushError(ast.(List).Cursor, ast, e)
			}
			ast = lst[len(lst)-1]
		case "if":
			cond, e := EVAL(a1, env, ctx)
			if e != nil {
				return nil, PushError(ast.(List).Cursor, ast, e)
			}
			if cond == nil || cond == false {
				if len(ast.(List).Val) >= 4 {
					ast = ast.(List).Val[3]
				} else {
					return nil, nil
				}
			} else {
				ast = a2
			}
		case "fn*":
			fn := MalFunc{
				Eval:    EVAL,
				Exp:     a2,
				Env:     env,
				Params:  a1,
				IsMacro: false,
				GenEnv:  NewEnv,
				Meta:    nil,
				Cursor:  ast.(List).Cursor,
			}
			return fn, nil
		default:
			el, e := eval_ast(ast, env, ctx)
			if e != nil {
				return nil, PushError(ast.(List).Cursor, ast, e)
			}
			f := el.(List).Val[0]
			if MalFunc_Q(f) {
				fn := f.(MalFunc)
				ast = fn.Exp
				env, e = NewEnv(fn.Env, fn.Params, List{Val: el.(List).Val[1:]})
				if e != nil {
					return nil, PushError(ast.(List).Cursor, ast, e)
				}
			} else {
				fn, ok := f.(Func)
				if !ok {
					switch f := f.(type) {
					case int:
						return nil, RuntimeError{
							ErrorVal: fmt.Errorf("attempt to call non-function (was integer %v)", f),
							Cursor:   ast.(List).Cursor,
						}
					case string:
						return nil, RuntimeError{
							ErrorVal: fmt.Errorf("attempt to call non-function (was string %q)", f),
							Cursor:   ast.(List).Cursor,
						}
					case Symbol:
						return nil, RuntimeError{
							ErrorVal: fmt.Errorf("attempt to call non-function (was symbol %q)", f.Val),
							Cursor:   ast.(List).Cursor,
						}
					default:
						return nil, RuntimeError{
							ErrorVal: fmt.Errorf("attempt to call non-function (was %T)", f),
							Cursor:   ast.(List).Cursor,
						}
					}
				}
				result, err := fn.Fn(el.(List).Val[1:], ctx)
				if err != nil {
					return nil, PushError(ast.(List).Cursor, ast, err)
				}
				return result, nil
			}
		}
	} // TCO loop
}

func PushError(cursor *Position, ast MalType, err error) error {
	switch err := err.(type) {
	case types.MalError:
		if err.Cursor == nil {
			err.Cursor = cursor
		}
		return err
	case RuntimeError:
		return RuntimeError{
			ErrorVal: nil,
			Parent:   &err,
			Trace:    printer.Pr_str(ast, true),
			Cursor:   cursor,
		}
	default:
		return RuntimeError{
			ErrorVal: err,
			Trace:    printer.Pr_str(ast, true),
			Cursor:   cursor,
		}
	case nil:
		return nil
	}
}

func malRecover(err *error) {
	if rerr := recover(); rerr != nil {
		*err = rerr.(error)
	}
}

// print
func PRINT(exp MalType) (string, error) {
	return printer.Pr_str(exp, true), nil
}

// repl
func REPL(repl_env EnvType, str string, ctx *context.Context) (MalType, error) {
	var exp MalType
	var res string
	var e error
	if exp, e = READ(str, nil); e != nil {
		return nil, e
	}
	if exp, e = EVAL(exp, repl_env, ctx); e != nil {
		return nil, e
	}
	if res, e = PRINT(exp); e != nil {
		return nil, e
	}
	return res, nil
}

// repl
func REPLPosition(repl_env EnvType, str string, ctx *context.Context, cursor *Position) (MalType, error) {
	var exp MalType
	var res string
	var e error
	if exp, e = READ(str, cursor); e != nil {
		return nil, e
	}
	if exp, e = EVAL(exp, repl_env, ctx); e != nil {
		return nil, e
	}
	if res, e = PRINT(exp); e != nil {
		return nil, e
	}
	return res, nil
}
