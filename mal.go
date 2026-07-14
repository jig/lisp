// Copyright (C) 2015 Joel Martin <github@martintribe.org>
// This code is derived from MAL (Make-A-Lisp) by Joel Martin and follows same licence.
// license that can be found in the LICENSE file.

// Copyright 2022 Jordi Íñigo Griera. All rights reserved.

// Package lisp provides a minimal Lisp interpreter focused on embedded work in
// Go code, as config or as a transmission format.
// Lisp external libraries are loaded from the Go code, and loading them from Lisp code is
// not allowed (on purpose).
//
// This interpreter is based on [kanaka/mal] implementation that is inspired on Clojure.
// It is still mostly compatible with kanaka/mal except that def!, try*, etc. symbols
// have been changed to def, try, etc. See ./examples/mal.lisp as a port of mal.mal
//
// Overview of this implementation addition to kanaka/mal:
//   - simpler embedded use with a simple package API (mostly inherited, just code reorganisation)
//   - testing based on Go tooling (all python tests scripts substituted by Go tests, see ./run_test.go)
//   - support of Go constructors to simplify extendability
//   - slightly faster parsing by swapping regex implementation for a text/scanner one
//   - support of preamble (AKA "placeholders") to simplify parametrisation of Go functions implemented on Lisp
//   - easier library development (using reflect)
//   - line numbers
//
// Functions and file directories keep the same structure as original MAL, this is way
// main functions [READ], [EVAL] and [PRINT] keep its all caps (non Go standard) names.
//
// # Embedding contract
//
// The interpreter is meant to run untrusted or hand-written Lisp, so a
// few boundaries are guaranteed:
//
//   - [READ] and [EVAL] report problems as an error (a
//     lisperror.LispError with position and stack trace). Malformed
//     Lisp input returns an error and never panics the host; a panic
//     escaping [EVAL] on a plain Lisp string is a bug. Exceptions:
//     unbounded non-tail recursion can exhaust the Go stack (tail calls
//     and loop/recur run in constant stack), and a builtin handed a
//     wrong-typed Go value by embedder code may still panic.
//   - [EVAL] accepts a nil context (cancellation and try timeouts are
//     then disabled); pass a context.WithTimeout deadline to bound
//     execution.
//   - An env is internally synchronised; share a base env across
//     goroutines, give each [EVAL] its own child env, and use atoms for
//     mutable state shared between goroutines.
//
// See the README "Embedding contract" section for the full version.
//
// [kanaka/mal]: https://github.com/kanaka/mal
package lisp

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	. "github.com/jig/lisp/env"
	"github.com/jig/lisp/lisperror"
	"github.com/jig/lisp/printer"
	"github.com/jig/lisp/reader"
	"github.com/jig/lisp/runtime"
	. "github.com/jig/lisp/types"
)

var placeholderRE = regexp.MustCompile(`^(;; \$[\-\d\w]+)+\s(.+)`)

const preamblePrefix = ";; $"

// DebugEvalEnabled is a deprecated shim for the legacy DEBUG-EVAL print
// behaviour. Setting it to true takes effect only in `lispdebug` builds,
// where it installs runtime.PrintEvalHook on the next EVAL call. In
// release builds (the default) the flag has no effect because the hook
// dispatch in EVAL is compiled out.
//
// Deprecated: in `lispdebug` builds, install a runtime.EvalHook directly
// via `runtime.Hook = runtime.PrintEvalHook{}` (or your own
// implementation). New code in this repo wires --debug through that path.
var DebugEvalEnabled = false

// READ reads Lisp source code and generates an AST that might be evaled by [EVAL] or printed by [PRINT].
//
// cursor and environment might be passed nil and READ will provide correct values for you.
// It is recommended though that cursor is initialised with a source code file identifier to
// provide better positioning information in case of encountering an execution error.
//
// EnvType is required in case you expect to parse Go constructors
func READ(sourceCode string, cursor *Position, ns EnvType) (MalType, error) {
	return reader.Read_str(sourceCode, cursor, nil, ns)
}

// READWithPreamble reads Lisp source code with preamble placeholders and generates an
// AST that might be evaled by [EVAL] or printed by [PRINT].
//
// cursor and environment might be passed nil and READ will provide correct values for you.
// It is recommended though that cursor is initialised with a source code file identifier to
// provide better positioning information in case of encountering an execution error.
//
// # EnvType is required in case you expect to parse Go constructors.
//
// Preamble placeholders are prefix the source code and have the following format:
//
//	;; <$-prefixed-var-name> <Lisp readable expression>
//
// For example:
//
//	;; $1 {:key "value"}
//	;; $NUMBER 1984
//	;; $EXPR1 (+ 1 1)
//
// will create three values that will fill the placeholders in the source code. Following the example
// the source code might look like:
//
//	...some code...
//	(prn "$NUMBER is" $NUMBER)
//
// note that the actual code to be parsed will be:
//
//	(prn "$NUMBER is" 1984)
//
// this simplifies inserting Lisp code in Go packages and passing Go parameters to it.
//
// Look for the "L-notation" to simplify the pass of complex Lisp structures as placeholders.
//
// READWithPreamble is used to read code (actually decode) on transmission. Use [AddPreamble]
// when calling from Go code.
func READWithPreamble(str string, cursor *Position, ns EnvType) (MalType, error) {
	placeholderMap := &HashMap{Val: map[string]MalType{}}
	i := 0
	for ; ; i++ {
		var line string
		line, str, _ = strings.Cut(str, "\n")
		line = strings.Trim(line, " \t\r\n")
		if len(line) == 0 {
			return reader.Read_str(str, cursor, placeholderMap, ns)
		}
		if !strings.HasPrefix(line, preamblePrefix) {
			return reader.Read_str(line+"\n"+str, cursor, placeholderMap, ns)
		}
		lineItems := placeholderRE.FindAllStringSubmatch(line, -1)
		if len(lineItems) != 1 || len(lineItems[0]) != 3 {
			return nil, lisperror.NewLispError(errors.New("invalid preamble format"), &Position{
				Row: i + 1,
				Col: 1,
			})
		}
		placeholderValue := lineItems[0][2]
		item, _ := reader.Read_str(placeholderValue, &Position{
			Row: i + 1,
			Col: 1,
		}, nil, ns)
		placeholderKey := lineItems[0][1][3:]
		placeholderMap.Val[placeholderKey] = item
	}
}

// AddPreamble combines prefix variables into a preamble to the provided source code.
//
// Source code encoded be readed with [READWithPreamble].
// placeholderMap must contain a map with keys being the variable names on the placeholder and the
// values the AST assigned to each placeholder. Value ASTs might be generated with [READ] or [EVAL] or with the
// [lnotation] package (most likely). Key names must contain the '$' prefix.
func AddPreamble(str string, placeholderMap map[string]MalType) (string, error) {
	preamble := ""
	for placeholderKey, placeholderValue := range placeholderMap {
		preamble = preamble + ";; " + placeholderKey + " " + PRINT(placeholderValue) + "\n"
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

func qq_loop(xs []MalType) (MalType, error) {
	acc := NewList(nil)
	for i := len(xs) - 1; 0 <= i; i -= 1 {
		elt := xs[i]
		switch e := elt.(type) {
		case List:
			if starts_with(e.Val, "splice-unquote") {
				if len(e.Val) < 2 {
					return nil, lisperror.NewLispError(errors.New("splice-unquote requires exactly 1 argument"), elt)
				}
				acc = NewList(lisperror.GetPosition(elt), Symbol{Val: "concat"}, e.Val[1], acc)
				continue
			}
		default:
		}
		q, err := quasiquote(elt)
		if err != nil {
			return nil, err
		}
		acc = NewList(lisperror.GetPosition(elt), Symbol{Val: "cons"}, q, acc)
	}
	return acc, nil
}

func quasiquote(ast MalType) (MalType, error) {
	switch a := ast.(type) {
	case Vector:
		inner, err := qq_loop(a.Val)
		if err != nil {
			return nil, err
		}
		return NewList(a.Cursor, Symbol{Val: "vec"}, inner), nil
	case HashMap, Symbol:
		return NewList(lisperror.GetPosition(ast), Symbol{Val: "quote"}, ast), nil
	case List:
		if starts_with(a.Val, "unquote") {
			if len(a.Val) < 2 {
				return nil, lisperror.NewLispError(errors.New("unquote requires exactly 1 argument"), ast)
			}
			return a.Val[1], nil
		}
		return qq_loop(a.Val)
	default:
		return ast, nil
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

// macroexpand fully expands ast while its head is a macro call. The
// boolean reports whether any expansion happened, so EVAL can re-run
// the debug hook on the expanded form (a breakpoint on the line of the
// outermost expanded call would otherwise never fire).
func macroexpand(ctx context.Context, ast MalType, env EnvType) (MalType, bool, error) {
	var mac MalType
	var e error
	origPos := lisperror.GetPosition(ast)
	expanded := false
	for is_macro_call(ast, env) {
		slc, _ := GetSlice(ast)
		a0 := slc[0]
		mac, e = env.Get(a0.(Symbol))
		if e != nil {
			return nil, expanded, e
		}
		fn := mac.(MalFunc)
		ast, e = Apply(ctx, fn, slc[1:])
		if e != nil {
			return nil, expanded, e
		}
		expanded = true
	}
	if expanded {
		ast = fillExpansionCursors(ast, origPos)
	}
	return ast, expanded, nil
}

// fillExpansionCursors assigns source positions to the forms a macro
// expansion created. Macro results are built at runtime by cons/concat
// (via quasiquote), which produce lists without cursors, so stepping
// and error reporting lost track of where expanded code came from —
// e.g. the debugger skipped every stage of a threading macro
// `(-> x (assoc …) (assoc …))`. Original subforms spliced into the
// expansion keep their cursors, so a generated list inherits the
// position of its first positioned child (for a threading stage, the
// head symbol the user wrote); anything else falls back to the
// position of the original macro call. Subtrees that already carry a
// cursor are original source and are left untouched.
func fillExpansionCursors(ast MalType, fallback *Position) MalType {
	switch n := ast.(type) {
	case List:
		if n.Cursor != nil {
			return n
		}
		for i, c := range n.Val {
			n.Val[i] = fillExpansionCursors(c, fallback)
		}
		n.Cursor = firstChildPosition(n.Val, fallback)
		return n
	case Vector:
		if n.Cursor != nil {
			return n
		}
		for i, c := range n.Val {
			n.Val[i] = fillExpansionCursors(c, fallback)
		}
		n.Cursor = firstChildPosition(n.Val, fallback)
		return n
	case HashMap:
		if n.Cursor != nil {
			return n
		}
		for k, v := range n.Val {
			n.Val[k] = fillExpansionCursors(v, fallback)
		}
		n.Cursor = fallback.Copy()
		return n
	default:
		return ast
	}
}

// firstChildPosition returns a copy of the first child's position, or
// a copy of fallback when no child carries one.
func firstChildPosition(vals []MalType, fallback *Position) *Position {
	for _, v := range vals {
		if p := lisperror.GetPosition(v); p != nil {
			return p.Copy()
		}
	}
	return fallback.Copy()
}

func eval_ast(ctx context.Context, ast MalType, env EnvType) (MalType, error) {
	if Q[Symbol](ast) {
		value, err := env.Get(ast.(Symbol))
		if err != nil {
			return nil, lisperror.NewLispError(err, ast)
		}
		return value, nil
	} else if Q[List](ast) {
		lst := []MalType{}
		origList := ast.(List)
		for _, a := range origList.Val {
			exp, e := evalInternal(ctx, a, env)
			if e != nil {
				// Preserve error and add context about which element failed
				return nil, e
			}
			lst = append(lst, exp)
		}
		return List{Val: lst, Cursor: origList.Cursor}, nil
	} else if Q[Vector](ast) {
		lst := []MalType{}
		origVec := ast.(Vector)
		for _, a := range origVec.Val {
			exp, e := evalInternal(ctx, a, env)
			if e != nil {
				// Preserve error and add context about which element failed
				return nil, e
			}
			lst = append(lst, exp)
		}
		return Vector{Val: lst, Cursor: origVec.Cursor}, nil
	} else if Q[HashMap](ast) {
		m := ast.(HashMap)
		new_hm := HashMap{Val: map[string]MalType{}, Cursor: m.Cursor}
		for k, v := range m.Val {
			kv, e2 := evalInternal(ctx, v, env)
			if e2 != nil {
				// Preserve error and add context about which key failed
				return nil, e2
			}
			new_hm.Val[k] = kv
		}
		return new_hm, nil
	} else {
		return ast, nil
	}
}

// extractFunctionName extracts the function, macro, or special form name from the AST
// Returns the name with "macro:" prefix if isMacro is true, or empty string if not applicable
func extractFunctionName(ast MalType, isMacro bool) string {
	if !Q[List](ast) {
		return "" // Not a function call
	}

	lst := ast.(List)
	if len(lst.Val) == 0 {
		return "" // Empty list
	}

	switch a0 := lst.Val[0].(type) {
	case Symbol:
		if isMacro {
			return "macro:" + a0.Val
		}
		return a0.Val
	case List:
		// Anonymous function: ((fn [...] ...) args)
		return "<lambda>"
	default:
		return "<unknown>"
	}
}

func do(ctx context.Context, ast MalType, from, to int, env EnvType) (MalType, error) {
	if ast == nil {
		return nil, nil
	}
	lst := ast.(List).Val
	if len(lst) == from {
		return nil, nil
	}
	evaledAST, e := eval_ast(ctx, List{Val: lst[from : len(lst)+to], Cursor: ast.(List).Cursor}, env)
	if e != nil {
		return nil, e
	}
	evaledLst := evaledAST.(List).Val
	if to == 0 {
		return evaledLst[len(evaledLst)-1], nil
	}
	return lst[len(lst)-1], nil
}

// recurValue is the sentinel `recur` produces: the evaluated rebind values,
// caught by the nearest enclosing `loop`. Using a distinct value (rather than
// an error) means a `recur` left in non-tail position reaches a real consumer
// (e.g. arithmetic) and fails there, instead of silently jumping.
type recurValue struct {
	args []MalType
}

func (recurValue) LispPrint(_ func(MalType, bool) string) string { return "«recur»" }

// EVAL evaluates an Abstract Syntaxt Tree (AST) and returns a result (a reduced AST).
// It takes a context that might cancel execution (a nil context is
// accepted and simply disables cancellation and try timeouts), and an
// environment that might be modified.
// AST usually is generated by [READ] or [READWithPreamble].
func EVAL(ctx context.Context, ast MalType, env EnvType) (MalType, error) {
	res, err := evalInternal(ctx, ast, env)
	if err != nil {
		return nil, err
	}
	// A recurValue reaching here means no enclosing `loop` consumed it:
	// `recur` was used outside any loop (top level, or a bare function
	// body). Surface it as an error rather than leaking the internal
	// sentinel — which otherwise prints as «recur» with a zero exit.
	if _, ok := res.(recurValue); ok {
		return nil, lisperror.NewLispError(errors.New("recur used outside of a loop"), ast)
	}
	return res, nil
}

// evalInternal is the interpreter core. Recursive calls inside the
// evaluator go through it (not EVAL) so a `recurValue` propagates
// untouched up to the enclosing `loop`; EVAL is the boundary that
// rejects a recur that escaped every loop.
func evalInternal(ctx context.Context, ast MalType, env EnvType) (res MalType, e error) {
	// Extract function name before any processing (to capture macro names before expansion)
	isMacro := is_macro_call(ast, env)
	functionName := extractFunctionName(ast, isMacro)

	// Live execution stack (debug builds only). In release builds the
	// helpers are no-ops and `runtime.Enabled` is the compile-time
	// constant `false`, so the whole block is dead code.
	var frame *runtime.Frame
	if runtime.Enabled {
		frame = runtime.MakeFrame(functionName, ast, env, lisperror.GetPosition(ast))
		if runtime.PushFrame(ctx, frame) {
			defer runtime.PopFrame(ctx)
		} else {
			frame = nil
		}
		// Record this frame's result as it completes, so the debugger can
		// show the return value after a step. A form's own EVAL returns
		// after its sub-forms, so the outermost stepped form records last.
		// recurValue is an internal loop sentinel, not a user value.
		defer func() {
			if e == nil {
				if _, isRecur := res.(recurValue); !isRecur {
					runtime.RecordResult(ctx, res)
				}
			}
		}()
	}

	// Add stack frame to any error that propagates out of this EVAL call.
	// In debug builds, first let an installed hook observe the error at
	// its raise point — detected by the stack still being empty, which is
	// true only at the innermost frame the error passes through — so
	// exception breakpoints can stop with the full call stack intact,
	// before it unwinds. The hook only observes; it must never wrap or
	// alter the propagating error (external catch / error-parsing code
	// depends on the format). In release builds runtime.Enabled is the
	// constant false and the whole DispatchError branch is dead code.
	defer func() {
		if e != nil {
			if lispErr, ok := e.(lisperror.LispError); ok {
				if runtime.Enabled && len(lispErr.Stack) == 0 {
					runtime.DispatchError(ctx, lispErr, ast, env, lisperror.GetPosition(ast), functionName)
				}
				e = lispErr.AddStackFrame(lisperror.GetPosition(ast), functionName)
			}
		}
	}()

	for {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return nil, lisperror.NewLispError(errors.New("timeout while evaluating expression"), ast)
			default:
			}
		}

		// Pluggable hook (debug builds only). When `runtime.Enabled` is
		// the compile-time constant `false` (release build), the entire
		// branch is dead code and the compiler removes it.
		if runtime.Enabled {
			runtime.UpdateFrame(frame, ast, env, lisperror.GetPosition(ast))
			if DebugEvalEnabled {
				installLegacyDebugHook()
			}
			if err := runtime.Dispatch(ctx, ast, env, lisperror.GetPosition(ast)); err != nil {
				return nil, err
			}
		}

		switch ast := ast.(type) {
		case List: // continue
		default:
			return eval_ast(ctx, ast, env)
		}

		// apply list
		var wasMacro bool
		ast, wasMacro, e = macroexpand(ctx, ast, env)
		if e != nil {
			return nil, e
		}
		// A macro expansion replaces the form mid-iteration: the hook
		// already ran for the original call, so run it again for the
		// expanded form. Otherwise a breakpoint or step on the line of
		// the outermost expanded call would never trigger.
		if runtime.Enabled && wasMacro {
			runtime.UpdateFrame(frame, ast, env, lisperror.GetPosition(ast))
			if err := runtime.Dispatch(ctx, ast, env, lisperror.GetPosition(ast)); err != nil {
				return nil, err
			}
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
			res, e := evalInternal(ctx, a2, env)
			if e != nil {
				return nil, e
			}
			switch a1 := a1.(type) {
			case Symbol:
				return env.Set(a1, res), nil
			default:
				return nil, lisperror.NewLispError(fmt.Errorf("cannot use '%T' as identifier", a1), ast)
			}
		case "let":
			let_env := NewSubordinateEnv(env)
			arr1, e := GetSlice(a1)
			if e != nil {
				return nil, e
			}
			if len(arr1)%2 != 0 {
				return nil, lisperror.NewLispError(errors.New("let: odd elements on binding vector"), a1)
			}
			for i := 0; i < len(arr1); i += 2 {
				if !Q[Symbol](arr1[i]) {
					return nil, lisperror.NewLispError(errors.New("non-symbol bind value"), a1)
				}
				exp, e := evalInternal(ctx, arr1[i+1], let_env)
				if e != nil {
					return nil, e
				}
				let_env.Set(arr1[i].(Symbol), exp)
			}
			astRef := ast.(List)
			ast, e = do(ctx, astRef, 2, -1, let_env)
			if e != nil {
				return nil, e
			}
			env = let_env
		case "loop":
			// Like `let`, but a recursion point: `recur` in tail position
			// rebinds these symbols and jumps back here, in constant stack.
			arr1, e := GetSlice(a1)
			if e != nil {
				return nil, e
			}
			if len(arr1)%2 != 0 {
				return nil, lisperror.NewLispError(errors.New("loop: odd elements on binding vector"), a1)
			}
			loop_env := NewSubordinateEnv(env)
			syms := make([]Symbol, 0, len(arr1)/2)
			for i := 0; i < len(arr1); i += 2 {
				if !Q[Symbol](arr1[i]) {
					return nil, lisperror.NewLispError(errors.New("loop: non-symbol bind value"), a1)
				}
				val, e := evalInternal(ctx, arr1[i+1], loop_env)
				if e != nil {
					return nil, e
				}
				loop_env.Set(arr1[i].(Symbol), val)
				syms = append(syms, arr1[i].(Symbol))
			}
			astRef := ast.(List)
			for {
				res, e := do(ctx, astRef, 2, 0, loop_env)
				if e != nil {
					return nil, e
				}
				rv, ok := res.(recurValue)
				if !ok {
					return res, nil
				}
				if len(rv.args) != len(syms) {
					return nil, lisperror.NewLispError(fmt.Errorf("recur: got %d arguments but loop has %d bindings", len(rv.args), len(syms)), ast)
				}
				for i, sym := range syms {
					loop_env.Set(sym, rv.args[i])
				}
			}
		case "recur":
			args := ast.(List).Val[1:]
			vals := make([]MalType, len(args))
			for i, arg := range args {
				v, e := evalInternal(ctx, arg, env)
				if e != nil {
					return nil, e
				}
				vals[i] = v
			}
			return recurValue{args: vals}, nil
		case "quote": // '
			return a1, nil
		case "quasiquoteexpand":
			return quasiquote(a1)
		case "quasiquote": // `
			var e error
			ast, e = quasiquote(a1)
			if e != nil {
				return nil, e
			}
		case "defmacro":
			fn, e := evalInternal(ctx, a2, env)
			if e != nil {
				return nil, e
			}
			switch fn := fn.(type) {
			case MalFunc:
				return env.Set(a1.(Symbol), fn.SetMacro()), nil
			default:
				return nil, lisperror.NewLispError(fmt.Errorf("defmacro: second argument must be a function (was of type %T)", fn), ast)
			}
		case "macroexpand":
			expanded, _, err := macroexpand(ctx, a1, env)
			return expanded, err
		case "try":
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
				if len(last.(List).Val) < 3 {
					return nil, lisperror.NewLispError(errors.New("catch must have 2 arguments at least"), last)
				}
				finallyDo = nil
				catchBind = last.(List).Val[1]
				catchDo = List{Val: last.(List).Val[2:], Cursor: last.(List).Cursor}
				tryDo = List{Val: lst[1 : len(lst)-1], Cursor: ast.(List).Cursor}
			case "finally":
				finallyDo = List{Val: last.(List).Val[1:], Cursor: last.(List).Cursor}
				switch first(prelast) {
				case "catch":
					if len(prelast.(List).Val) < 3 {
						return nil, lisperror.NewLispError(errors.New("catch must have 2 arguments at least"), prelast)
					}
					catchBind = prelast.(List).Val[1]
					catchDo = List{Val: prelast.(List).Val[2:], Cursor: prelast.(List).Cursor}
					tryDo = List{Val: lst[1 : len(lst)-2], Cursor: ast.(List).Cursor}
				default:
					catchBind = nil
					catchDo = nil
					tryDo = List{Val: lst[1 : len(lst)-1], Cursor: ast.(List).Cursor}
				}
			default:
				finallyDo = nil
				catchBind = nil
				catchDo = nil
				tryDo = List{Val: lst[1:], Cursor: ast.(List).Cursor}
			}
			exp, e := func() (res MalType, err error) {
				defer malRecover(&err)
				// A nil context is accepted throughout EVAL (it just
				// disables cancellation); guard the deadline lookup so
				// `try` does not panic on it.
				if ctx != nil {
					if dl, ok := ctx.Deadline(); ok {
						// give 80% of the time to the try, and the remaining 20% to the catch + finally
						timeout := (time.Until(dl) / 10) * 8
						ctx, cancel := context.WithTimeout(ctx, timeout)
						defer cancel()
						return do(ctx, tryDo, 0, 0, env)
					}
				}
				return do(ctx, tryDo, 0, 0, env)
			}()

			defer func() { _, _ = do(ctx, finallyDo, 0, 0, env) }()

			if e == nil {
				return exp, nil
			} else {
				if catchDo != nil {
					var caughtError MalType
					if er, ok := e.(interface{ ErrorValue() MalType }); ok {
						caughtError = er.ErrorValue()
					} else {
						caughtError = e.Error()
					}
					binds := NewList(nil, catchBind)
					new_env, err := NewSubordinateEnvWithBinds(env, binds, NewList(nil, caughtError))
					if err != nil {
						return nil, err
					}
					// TCO like `let`: evaluate all but the last catch form and
					// hand the last one to the trampoline. Fully evaluating
					// here (to == 0) and then continuing would evaluate the
					// catch result a second time, re-raising any `throw` form
					// carried inside it as data (issue #87).
					ast, err = do(ctx, catchDo, 0, -1, new_env)
					if err != nil {
						return nil, err
					}
					env = new_env
					continue
				}
				return nil, e
			}
		case "do":
			var err error
			ast, err = do(ctx, ast, 1, -1, env)
			if err != nil {
				return nil, err
			}
		case "if":
			cond, e := evalInternal(ctx, a1, env)
			if e != nil {
				return nil, e
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
			// Guard the body slice: `(fn)` has no parameter list, so
			// Val[2:] would be out of range. Treat it like `(fn nil)` — a
			// no-op function — instead of panicking.
			var body []MalType
			if l := ast.(List).Val; len(l) >= 2 {
				body = l[2:]
			}
			fn := MalFunc{
				Eval:    EVAL,
				Exp:     List{Val: append([]MalType{Symbol{Val: "do"}}, body...), Cursor: ast.(List).Cursor},
				Env:     env,
				Params:  a1,
				IsMacro: false,
				GenEnv:  NewSubordinateEnvWithBinds,
				Meta:    nil,
				Cursor:  ast.(List).Cursor,
			}
			return fn, nil
		default:
			el, e := eval_ast(ctx, ast, env)
			if e != nil {
				return nil, e
			}
			f := el.(List).Val[0]
			if Q[MalFunc](f) {
				fn := f.(MalFunc)
				ast = fn.Exp
				env, e = NewSubordinateEnvWithBinds(fn.Env, fn.Params, List{Val: el.(List).Val[1:], Cursor: el.(List).Cursor})
				if e != nil {
					if ast == nil {
						return nil, lisperror.NewLispError(e, nil)
					}
					switch v := ast.(List).Val[0].(type) {
					case Symbol:
						return nil, lisperror.NewLispError(fmt.Errorf("%s (around %s)", e, v.Val), ast)
					default:
						return nil, lisperror.NewLispError(e, ast)
					}
				}
			} else {
				fn, ok := f.(Func)
				if !ok {
					return nil, lisperror.NewLispError(fmt.Errorf("attempt to call non-function (was of type %T)", f), el)
				}
				result, err := fn.Fn(ctx, el.(List).Val[1:])
				if err != nil {
					return nil, lisperror.NewLispError(err, ast)
				}
				return result, nil
			}
		}
	} // TCO loop
}

func first(list MalType) string {
	if list != nil && Q[List](list) {
		l := list.(List).Val
		if len(l) > 0 && Q[Symbol](l[0]) {
			return l[0].(Symbol).Val
		}
	}
	return ""
}

func malRecover(err *error) {
	rerr := recover()
	if rerr == nil {
		return
	}
	// A recovered value is usually a Go error (runtime panics satisfy
	// error), but a raw non-error panic value would make an unchecked
	// type assertion re-panic. Wrap it as a LispError instead.
	if e, ok := rerr.(error); ok {
		*err = e
	} else {
		*err = lisperror.NewLispError(rerr, nil)
	}
}

// PRINT converts an AST to a string, suitable for printing
// AST might be generated by [EVAL] or by [READ] or [READWithPreamble].
func PRINT(ast MalType) string {
	return printer.Pr_str(ast, true)
}

// REPL or [READ], [EVAL] and [PRINT] loop execute those three functions in sequence.
// (but the loop "L" actually must be executed by the caller)
func REPL(ctx context.Context, env EnvType, sourceCode string, cursor *Position) (MalType, error) {
	ast, err := READ(sourceCode, cursor, env)
	if err != nil {
		return nil, err
	}
	exp, err := EVAL(ctx, ast, env)
	if err != nil {
		return nil, err
	}
	return PRINT(exp), nil
}

// REPLWithPreamble or [READ], [EVAL] and [PRINT] loop with preamble execute those three functions in sequence.
// (but the loop "L" actually must be executed by the caller)
//
// Source code might include a preamble with the values for the placeholders. See [READWithPreamble]
func REPLWithPreamble(ctx context.Context, env EnvType, sourceCode string, cursor *Position) (MalType, error) {
	ast, err := READWithPreamble(sourceCode, cursor, env)
	if err != nil {
		return nil, err
	}
	exp, err := EVAL(ctx, ast, env)
	if err != nil {
		return nil, err
	}
	return PRINT(exp), nil
}

// ReadEvalWithPreamble or [READ] and [EVAL] with preamble execute those three functions in sequence.
// (but the loop "L" actually must be executed by the caller)
//
// Source code might include a preamble with the values for the placeholders. See [READWithPreamble]
// ReadEvalWithPreamble returns the result in AST structure.
func ReadEvalWithPreamble(ctx context.Context, env EnvType, sourceCode string, cursor *Position) (MalType, error) {
	ast, err := READWithPreamble(sourceCode, cursor, env)
	if err != nil {
		return nil, err
	}
	return EVAL(ctx, ast, env)
}
