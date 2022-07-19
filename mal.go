package lisp

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	. "github.com/jig/lisp/env"
	"github.com/jig/lisp/printer"
	"github.com/jig/lisp/reader"
	. "github.com/jig/lisp/types"
)

var placeholderRE = regexp.MustCompile(`^(;; \$[\d\w]+)+\s(.+)`)

const preamblePrefix = ";; $"

// READ reads an expression
func READ(str string, cursor *Position) (MalType, error) {
	return reader.Read_str(str, cursor, nil)
}

// READ reads an expression with preamble placeholders
func READWithPreamble(str string, cursor *Position) (MalType, error) {
	placeholderMap := &HashMap{Val: map[string]MalType{}}
	i := 0
	for ; ; i++ {
		var line string
		// line, str, _ = strings.Cut(str, "\n")
		line, str, _ = strings.Cut(str, "\n")
		line = strings.Trim(line, " \t\r\n")
		if len(line) == 0 {
			return reader.Read_str(str, cursor, placeholderMap)
		}
		if !strings.HasPrefix(line, preamblePrefix) {
			return reader.Read_str(line+"\n"+str, cursor, placeholderMap)
		}
		lineItems := placeholderRE.FindAllStringSubmatch(line, -1)
		if len(lineItems) != 1 || len(lineItems[0]) != 3 {
			return nil, MalError{
				Obj: errors.New("invalid preamble format"),
				Cursor: &Position{
					Row: i + 1,
					Col: 1,
				},
			}
		}
		placeholderValue := lineItems[0][2]
		item, _ := reader.Read_str(placeholderValue, &Position{
			Row: i + 1,
			Col: 1,
		}, nil)
		placeholderKey := lineItems[0][1][3:]
		placeholderMap.Val[placeholderKey] = item
	}
}

// AddPreamble
func AddPreamble(str string, placeholderMap map[string]MalType) (string, error) {
	preamble := ""
	for placeholderKey, placeholderValue := range placeholderMap {
		s, err := PRINT(placeholderValue)
		if err != nil {
			return "", err
		}
		preamble = preamble + ";; " + placeholderKey + " " + s + "\n"
	}
	return preamble + "\n" + str, nil
}

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
	if Q[List](ast) {
		slc, _ := GetSlice(ast)
		if len(slc) == 0 {
			return false
		}
		a0 := slc[0]
		if Q[Symbol](a0) && env.Find(a0.(Symbol)) != nil {
			mac, e := env.Get(a0.(Symbol))
			if e != nil {
				return false
			}
			if Q[MalFunc](mac) {
				return mac.(MalFunc).GetMacro()
			}
		}
	}
	return false
}

func macroexpand(ctx context.Context, ast MalType, env EnvType) (MalType, error) {
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
		ast, e = Apply(ctx, fn, slc[1:])
		if e != nil {
			return nil, e
		}
	}
	return ast, nil
}

func eval_ast(ctx context.Context, ast MalType, env EnvType) (MalType, error) {
	if Q[Symbol](ast) {
		value, err := env.Get(ast.(Symbol))
		if err != nil {
			return nil, PushError(ast.(Symbol).Cursor, err)
		}
		return value, nil
	} else if Q[List](ast) {
		lst := []MalType{}
		for _, a := range ast.(List).Val {
			exp, e := EVAL(ctx, a, env)
			if e != nil {
				if a, ok := a.(List); ok {
					return nil, PushError(a.Cursor, e)
				}
				return nil, PushError(nil, e)
			}
			lst = append(lst, exp)
		}
		return List{Val: lst}, nil
	} else if Q[Vector](ast) {
		lst := []MalType{}
		for _, a := range ast.(Vector).Val {
			exp, e := EVAL(ctx, a, env)
			if e != nil {
				return nil, PushError(ast.(Vector).Cursor, e)
			}
			lst = append(lst, exp)
		}
		return Vector{Val: lst}, nil
	} else if Q[HashMap](ast) {
		m := ast.(HashMap)
		new_hm := HashMap{Val: map[string]MalType{}}
		for k, v := range m.Val {
			kv, e2 := EVAL(ctx, v, env)
			if e2 != nil {
				return nil, PushError(ast.(HashMap).Cursor, e2)
			}
			new_hm.Val[k] = kv
		}
		return new_hm, nil
	} else {
		return ast, nil
	}
}

func EVAL(ctx context.Context, ast MalType, env EnvType) (MalType, error) {
	var e error
	for {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return nil, errors.New("timeout while evaluating expression")
			default:
			}
		}

		switch ast := ast.(type) {
		case List: // continue
			// aStr, _ := PRINT(ast)
			// fmt.Printf("%s◉ %s\n", ast.Cursor, aStr)
		default:
			// aStr, _ := PRINT(ast)
			// fmt.Printf("%T○ %s\n", ast, aStr)
			return eval_ast(ctx, ast, env)
		}

		// apply list
		ast, e = macroexpand(ctx, ast, env)
		if e != nil {
			return nil, e
		}
		if !Q[List](ast) {
			return eval_ast(ctx, ast, env)
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
		case 3:
			a1 = ast.(List).Val[1]
			a2 = ast.(List).Val[2]
		default:
			a1 = ast.(List).Val[1]
			a2 = ast.(List).Val[2]
		}
		a0sym := "__<*fn>__"
		if Q[Symbol](a0) {
			a0sym = a0.(Symbol).Val
		}
		switch a0sym {
		case "def":
			res, e := EVAL(ctx, a2, env)
			if e != nil {
				return nil, PushError(ast.(List).Cursor, e)
			}
			switch a1 := a1.(type) {
			case Symbol:
				return env.Set(a1, res), nil
			default:
				return nil, MalError{
					Obj:    fmt.Errorf("cannot use '%T' as identifier", a1),
					Cursor: ast.(List).Cursor,
				}
			}
		case "let":
			let_env, e := NewEnv(env, nil, nil)
			if e != nil {
				return nil, e
			}
			arr1, e := GetSlice(a1)
			if e != nil {
				return nil, e
			}
			if len(arr1)%2 != 0 {
				return nil, MalError{
					Obj:    errors.New("let: odd elements on binding vector"),
					Cursor: a1.(Vector).Cursor,
				}
			}
			for i := 0; i < len(arr1); i += 2 {
				if !Q[Symbol](arr1[i]) {
					return nil, MalError{
						Obj:    errors.New("non-symbol bind value"),
						Cursor: a1.(Vector).Cursor,
					}
				}
				exp, e := EVAL(ctx, arr1[i+1], let_env)
				if e != nil {
					return nil, PushError(arr1[i].(Symbol).Cursor, e)
				}
				let_env.Set(arr1[i].(Symbol), exp)
			}
			astRef := ast.(List)
			ast, e = do(ctx, astRef, 2, -1, let_env)
			if e != nil {
				return nil, e
			}
			env = let_env
		case "quote": // '
			return a1, nil
		case "quasiquoteexpand":
			return quasiquote(a1), nil
		case "quasiquote": // `
			ast = quasiquote(a1)
		case "defmacro":
			fn, e := EVAL(ctx, a2, env)
			fn = fn.(MalFunc).SetMacro()
			if e != nil {
				return nil, e
			}
			return env.Set(a1.(Symbol), fn), nil
		case "macroexpand":
			return macroexpand(ctx, a1, env)
		case "try":
			var exc MalType
			lst := ast.(List).Val
			var last MalType
			var prelast MalType
			switch len(lst) {
			case 1:
				return nil, nil
			case 2:
				last = lst[1]
				prelast = nil
			case 3:
				last = lst[2]
				prelast = lst[1]
			default:
				last = lst[len(lst)-1]
				prelast = lst[len(lst)-2]
			}
			var tryDo, catchDo, finallyDo MalType // Lists
			var catchBind MalType                 // Symbol

			switch first(last) {
			case "catch":
				finallyDo = nil
				catchBind = last.(List).Val[1]
				catchDo = List{Val: last.(List).Val[2:]}
				tryDo = List{Val: lst[1 : len(lst)-1]}
				if len(catchDo.(List).Val) == 0 {
					return nil, PushError(ast.(List).Cursor, errors.New("catch must have 2 arguments at least"))
				}
			case "finally":
				finallyDo = List{Val: last.(List).Val[1:]}
				switch first(prelast) {
				case "catch":
					catchBind = prelast.(List).Val[1]
					catchDo = List{Val: prelast.(List).Val[2:]}
					tryDo = List{Val: lst[1 : len(lst)-2]}
				default:
					catchBind = nil
					catchDo = nil
					tryDo = List{Val: lst[1 : len(lst)-1]}
				}
			default:
				finallyDo = nil
				catchBind = nil
				catchDo = nil
				tryDo = List{Val: lst[1:]}
			}
			exp, e := func() (res MalType, err error) {
				defer malRecover(&err)
				return do(ctx, tryDo, 0, 0, env)
			}()

			defer func() { _, _ = do(ctx, finallyDo, 0, 0, env) }()

			if e == nil {
				return exp, nil
			} else {
				if catchDo != nil {
					switch e := e.(type) {
					case MalError:
						exc = e.Obj
					default:
						exc = MalError{
							Obj:    e,
							Cursor: catchDo.(List).Cursor,
						}
					}
					binds := NewList(catchBind)
					new_env, e := NewEnv(env, binds, NewList(exc))
					if e != nil {
						return nil, e
					}
					ast, e = do(ctx, catchDo, 0, 0, new_env)
					if e != nil {
						return nil, e
					}
					env = new_env
					continue
				}
				return nil, e
			}
		case "context":
			if a2 != nil {
				return nil, MalError{
					Obj:    fmt.Errorf("context does not allow more than one argument"),
					Cursor: a2.(Vector).Cursor,
				}
			}
			childCtx, cancel := context.WithCancel(ctx)
			exp, e := func() (res MalType, err error) {
				defer cancel()
				defer malRecover(&err)
				return EVAL(childCtx, a1, env)
			}()
			if e != nil {
				return nil, e
			}
			return exp, nil
		case "do":
			var err error
			ast, err = do(ctx, ast, 1, -1, env)
			if err != nil {
				return nil, err
			}
		case "if":
			cond, e := EVAL(ctx, a1, env)
			if e != nil {
				return nil, PushError(ast.(List).Cursor, e)
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
		case "fn":
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
			el, e := eval_ast(ctx, ast, env)
			if e != nil {
				return nil, PushError(ast.(List).Cursor, e)
			}
			f := el.(List).Val[0]
			if Q[MalFunc](f) {
				fn := f.(MalFunc)
				ast = fn.Exp
				env, e = NewEnv(fn.Env, fn.Params, List{Val: el.(List).Val[1:]})
				if e != nil {
					return nil, PushError(ast.(List).Cursor, e)
				}
			} else {
				fn, ok := f.(Func)
				if !ok {
					return nil, MalError{
						Obj:    fmt.Errorf("attempt to call non-function (was of type %T)", f),
						Cursor: f.(List).Cursor,
					}
				}
				result, err := fn.Fn(ctx, el.(List).Val[1:])
				if err != nil {
					return nil, PushError(ast.(List).Cursor, err)
				}
				return result, nil
			}
		}
	} // TCO loop
}

func first(list MalType) string {
	if list != nil && Q[List](list) && Q[Symbol](list.(List).Val[0]) {
		return list.(List).Val[0].(Symbol).Val
	}
	return ""
}

func do(ctx context.Context, ast MalType, from, to int, env EnvType) (MalType, error) {
	if ast == nil {
		return nil, nil
	}
	lst := ast.(List).Val
	if len(lst) == from {
		return nil, nil
	}
	evaledAST, e := eval_ast(ctx, List{Val: lst[from : len(lst)+to]}, env)
	if e != nil {
		return nil, PushError(ast.(List).Cursor, e)
	}
	evaledLst := evaledAST.(List).Val
	if to == 0 {
		return evaledLst[len(evaledLst)-1], nil
	}
	return lst[len(lst)-1], nil
}

func PushError(cursor *Position, err error) error {
	switch err := err.(type) {
	case MalError:
		if err.Cursor == nil {
			err.Cursor = cursor
		}
		return err
	default:
		return MalError{
			Obj:    err,
			Cursor: cursor,
		}
	case nil:
		panic(err)
	}
}

func malRecover(err *error) {
	rerr := recover()
	if rerr != nil {
		*err = rerr.(error)
	}
}

// PRINT
func PRINT(exp MalType) (string, error) {
	return printer.Pr_str(exp, true), nil
}

// REPL
func REPL(ctx context.Context, repl_env EnvType, str string, cursor *Position) (MalType, error) {
	return REPLPosition(ctx, repl_env, str, cursor)
}

// REPLPosition
func REPLPosition(ctx context.Context, repl_env EnvType, str string, cursor *Position) (MalType, error) {
	var exp MalType
	var res string
	var e error

	if exp, e = READ(str, cursor); e != nil {
		return nil, e
	}
	if exp, e = EVAL(ctx, exp, repl_env); e != nil {
		return nil, e
	}
	if res, e = PRINT(exp); e != nil {
		return nil, e
	}
	return res, nil
}

// REPLWithPreamble
func REPLWithPreamble(ctx context.Context, repl_env EnvType, str string, cursor *Position) (MalType, error) {
	var exp MalType
	var res string
	var e error
	if exp, e = READWithPreamble(str, cursor); e != nil {
		return nil, e
	}
	if exp, e = EVAL(ctx, exp, repl_env); e != nil {
		return nil, e
	}
	if res, e = PRINT(exp); e != nil {
		return nil, e
	}
	return res, nil
}
