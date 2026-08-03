// Package core defines the Lisp built-in functions implemented in Go.
//
// Lists, vectors, and hashmaps returned by these builtins are
// runtime-constructed and therefore intentionally carry a nil Cursor:
// they have no source-code position to attribute. Source positions are
// preserved only on AST nodes produced by the reader.
package core

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/term"

	spew "github.com/davecgh/go-spew/spew"
	"github.com/google/uuid"
	"github.com/jig/lisp/lib/call"
	"github.com/jig/lisp/lisperror"
	"github.com/jig/lisp/marshaler"
	"github.com/jig/lisp/printer"
	"github.com/jig/lisp/reader"

	. "github.com/jig/lisp/types"
)

//go:embed header-basic.lisp
var headerBasic string

func HeaderBasic() string { return headerBasic }

func Load(env EnvType) {
	call.Call(env, assoc_in)
	call.Call(env, update)
	call.Call(env, update_in)
	call.CallOverrideFN(env, "<", ltN, 1)
	call.CallOverrideFN(env, "<=", leN, 1)
	call.CallOverrideFN(env, ">", gtN, 1)
	call.CallOverrideFN(env, ">=", geN, 1)
	call.CallOverrideFN(env, "+", addN)
	call.CallOverrideFN(env, "sort", sortColl)
	call.CallOverrideFN(env, "sort-by", sortByColl)
	call.CallOverrideFN(env, "-", subN, 1)
	call.CallOverrideFN(env, "*", mulN)
	call.CallOverrideFN(env, "/", divN, 1)
	call.CallOverrideFN(env, "=", func(a, b MalType) (MalType, error) { return Equal_Q(a, b), nil })
	call.CallOverrideFN(env, "not=", func(a, b MalType) (MalType, error) { return !Equal_Q(a, b), nil })
	call.Call(env, get)
	call.Call(env, get_in)
	call.CallOverrideFN(env, "contains?", contains_Q)
	call.Call(env, cons)
	call.Call(env, nth)
	call.Call(env, with_meta)
	call.Call(env, rAnge)
	call.Call(env, hash_map_decode)
	call.Call(env, JSON_Decode)
	call.Call(env, mErge)
	call.Call(env, rename_keys)
	call.Call(env, split)
	call.Call(env, subs, 2, 3)
	call.CallOverrideFN(env, "starts-with?", func(s, prefix string) (bool, error) { return strings.HasPrefix(s, prefix), nil })
	call.CallOverrideFN(env, "ends-with?", func(s, suffix string) (bool, error) { return strings.HasSuffix(s, suffix), nil })
	call.Call(env, mAp)
	call.Call(env, throw)
	call.CallOverrideFN(env, "symbol", func(a string) (Symbol, error) { return Symbol{Val: a}, nil })
	call.CallOverrideFN(env, "keyword", func(a MalType) (Keyword, error) {
		switch a := a.(type) {
		case Keyword:
			return a, nil
		case string:
			return KW(a), nil
		default:
			return "", fmt.Errorf("keyword requires a string or a keyword (found %T)", a)
		}
	})
	call.Call(env, sPew)
	call.CallOverrideFN(env, "read-string", func(a MalType) (MalType, error) { return reader.Read_str(a.(string), nil, nil) })
	// read-program reads a whole file's worth of forms into one (do …)
	// AST with positions attributed to module; like read-string it runs
	// without an environment («…» constructors are not resolved).
	call.CallOverrideFN(env, "read-program", func(src, module string) (MalType, error) {
		return reader.Read_program(src, NewCursorFile(module), nil)
	})
	call.CallOverrideFN(env, "set", func(a MalType) (Set, error) { return NewSet(a) })
	call.Call(env, keys)
	call.Call(env, vals)
	call.Call(env, vec)
	call.Call(env, first)
	call.Call(env, rest)
	call.Call(env, count)
	call.Call(env, seq)
	call.Call(env, meta)
	call.Call(env, deref)
	call.Call(env, bAse64)
	call.Call(env, unbase64)
	call.Call(env, str2binary)
	call.Call(env, binary2str)
	call.Call(env, json_encode)
	call.Call(env, sleep)
	call.Call(env, gensym)
	call.Call(env, time_ms)
	call.Call(env, time_ns)
	call.Call(env, time_format)
	call.Call(env, time_parse)
	call.Call(env, time_add)
	call.CallOverrideFN(env, "time-before?", time_before)
	call.CallOverrideFN(env, "time-after?", time_after)
	call.Call(env, uUid)
	call.Call(env, pr_str)
	call.Call(env, str)
	call.Call(env, prn)
	call.Call(env, println)
	call.CallOverrideFN(env, "print", printNoNewline)
	call.CallOverrideFN(env, "list", func(a ...MalType) (List, error) { return List{Val: a}, nil })
	call.CallOverrideFN(env, "vector", func(a ...MalType) (Vector, error) { return Vector{Val: a}, nil })
	call.Call(env, hash_map)
	call.CallOverrideFN(env, "hash-set", func(a ...MalType) (Set, error) { return NewSet(List{Val: a}) })
	call.Call(env, assoc)
	call.Call(env, dissoc)
	call.Call(env, concat)

	call.CallOverrideFN(env, "nil?", func(a MalType) (bool, error) { return Nil_Q(a), nil })
	call.CallOverrideFN(env, "true?", func(a MalType) (bool, error) { return True_Q(a), nil })
	call.CallOverrideFN(env, "false?", func(a MalType) (bool, error) { return False_Q(a), nil })
	call.CallOverrideFN(env, "empty?", empty_Q)
	call.CallOverrideFN(env, "symbol?", func(a MalType) (bool, error) { return Q[Symbol](a), nil })
	call.CallOverrideFN(env, "keyword?", func(a MalType) (bool, error) { return Keyword_Q(a), nil })
	call.CallOverrideFN(env, "string?", func(a MalType) (bool, error) { return String_Q(a), nil })
	call.CallOverrideFN(env, "number?", func(a MalType) (bool, error) { return Q[int](a), nil })
	call.CallOverrideFN(env, "fn?", fn_q)
	call.CallOverrideFN(env, "macro?", func(a MalType) (bool, error) { return Q[MalFunc](a) && a.(MalFunc).GetMacro(), nil })
	call.CallOverrideFN(env, "list?", func(a MalType) (bool, error) { return Q[List](a), nil })
	call.CallOverrideFN(env, "vector?", func(a MalType) (bool, error) { return Q[Vector](a), nil })
	call.CallOverrideFN(env, "map?", func(a MalType) (bool, error) { return Q[HashMap](a), nil })
	call.CallOverrideFN(env, "set?", func(a MalType) (bool, error) { return Q[Set](a), nil })
	call.CallOverrideFN(env, "sequential?", func(a MalType) (bool, error) { return Sequential_Q(a), nil })

	call.Call(env, apply, 2)     // at least two parameters
	call.Call(env, conj, 0)      // at least zero parameters
	call.Call(env, assert, 1, 2) // at least one parameter, at most two

	call.Call(env, go_error, 1) // at least one parameter
	call.Call(env, pAnic)
	call.Call(env, unwrap_error)
	call.Call(env, error_string)

	call.CallOverrideFN(env, "type?", istype)
	call.Call(env, new_error, 1, 2)
	call.Call(env, new_go_error)

	call.Call(env, take)
	call.Call(env, take_last)
	call.Call(env, drop)
	call.Call(env, drop_last)
	call.Call(env, subvec, 2, 3)
	call.CallOverrideFN(env, "doc", docString)

	loadDocs(env)
}

func subvec(args ...MalType) (MalType, error) {
	l := len(args)
	if l != 2 && l != 3 {
		return nil, fmt.Errorf("subvec wrong number of args (%d instead of 2 or 3)", l)
	}
	v, ok := args[0].(Vector)
	if !ok {
		return nil, fmt.Errorf("subvec requires a vector (it was %T)", args[0])
	}
	var from, to int
	if l == 2 {
		from = args[1].(int)
		to = len(v.Val)
	} else {
		from = args[1].(int)
		to = args[2].(int)
	}
	return Vector{
		Val: v.Val[from:to],
	}, nil
}

func take(elems int, arg MalType) (MalType, error) {
	// note that Clojure returns a list, not another vector in this case
	new_list := List{Val: []MalType{}}

	switch arg := arg.(type) {
	case List:
		for i := 0; i < elems && i < len(arg.Val); i++ {
			new_list.Val = append(new_list.Val, arg.Val[i])
		}
	case Vector:
		for i := 0; i < elems && i < len(arg.Val); i++ {
			new_list.Val = append(new_list.Val, arg.Val[i])
		}
	case nil:
		// if nil return an empty list
	default:
		return nil, fmt.Errorf("take called on non-list and non-vector (it was %T)", arg)
	}
	return new_list, nil
}

func take_last(elems int, arg MalType) (MalType, error) {
	// note that Clojure returns a list, not another vector in this case
	new_list := List{}

	if elems < 0 {
		elems = 0
	}

	switch arg := arg.(type) {
	case List:
		start := len(arg.Val) - elems
		if start < 0 {
			start = 0
		}
		for i := start; i < len(arg.Val); i++ {
			new_list.Val = append(new_list.Val, arg.Val[i])
		}
	case Vector:
		start := len(arg.Val) - elems
		if start < 0 {
			start = 0
		}
		for i := start; i < len(arg.Val); i++ {
			new_list.Val = append(new_list.Val, arg.Val[i])
		}
	case nil:
		// if nil return an empty list
	default:
		return nil, fmt.Errorf("take called on non-list and non-vector (it was %T)", arg)
	}

	if len(new_list.Val) == 0 {
		return nil, nil
	}
	return new_list, nil
}

func drop(n int, arg MalType) (MalType, error) {
	// note that Clojure returns a list, not another vector in this case
	new_list := List{Val: []MalType{}}
	if n < 0 {
		n = 0
	}

	switch arg := arg.(type) {
	case List:
		for i := n; i < len(arg.Val); i++ {
			new_list.Val = append(new_list.Val, arg.Val[i])
		}
	case Vector:
		for i := n; i < len(arg.Val); i++ {
			new_list.Val = append(new_list.Val, arg.Val[i])
		}
	case nil:
		// if nil return an empty list
	default:
		return nil, fmt.Errorf("drop called on non-list and non-vector (it was %T)", arg)
	}
	return new_list, nil
}

func drop_last(n int, arg MalType) (MalType, error) {
	// note that Clojure returns a list, not another vector in this case
	new_list := List{Val: []MalType{}}
	if n < 0 {
		n = 0
	}

	switch arg := arg.(type) {
	case List:
		for i := 0; i < len(arg.Val)-n; i++ {
			new_list.Val = append(new_list.Val, arg.Val[i])
		}
	case Vector:
		for i := 0; i < len(arg.Val)-n; i++ {
			new_list.Val = append(new_list.Val, arg.Val[i])
		}
	case nil:
		// if nil return an empty list
	default:
		return nil, fmt.Errorf("drop called on non-list and non-vector (it was %T)", arg)
	}
	return new_list, nil
}

// sortElems copies the elements of a list or vector (nil sorts to an
// empty list) and validates each one is an orderable scalar — the same
// total order hash-map keys use (nil < booleans < ints < floats <
// strings < keywords).
func sortElems(fnName string, coll MalType) ([]MalType, error) {
	var src []MalType
	switch c := coll.(type) {
	case List:
		src = c.Val
	case Vector:
		src = c.Val
	case nil:
		src = nil
	default:
		return nil, fmt.Errorf("%s: expected a list or vector, got %T", fnName, coll)
	}
	out := make([]MalType, len(src))
	copy(out, src)
	return out, nil
}

func validOrderKey(fnName string, k MalType) error {
	switch k.(type) {
	case nil, bool, int, float32, string, Keyword:
		return nil
	}
	return fmt.Errorf("%s: cannot order by %T (nil, booleans, numbers, strings and keywords have a defined order)", fnName, k)
}

func sortColl(coll MalType) (MalType, error) {
	elems, err := sortElems("sort", coll)
	if err != nil {
		return nil, err
	}
	for _, e := range elems {
		if err := validOrderKey("sort", e); err != nil {
			return nil, err
		}
	}
	sort.SliceStable(elems, func(i, j int) bool { return KeyLess(elems[i], elems[j]) })
	return List{Val: elems}, nil
}

// sortByColl sorts by (f element). f is a function, or a keyword used
// as a map accessor (Clojure-style, so (sort-by :ms results) reads
// naturally even though bare keywords are not callable here).
func sortByColl(ctx context.Context, f MalType, coll MalType) (MalType, error) {
	elems, err := sortElems("sort-by", coll)
	if err != nil {
		return nil, err
	}
	keys := make([]MalType, len(elems))
	for i, e := range elems {
		var k MalType
		if kw, ok := f.(Keyword); ok {
			if hm, ok := e.(HashMap); ok {
				k = hm.Items[kw]
			} else {
				return nil, fmt.Errorf("sort-by: keyword key %v needs hash-map elements, got %T", f, e)
			}
		} else {
			if k, err = Apply(ctx, f, []MalType{e}); err != nil {
				return nil, err
			}
		}
		if err := validOrderKey("sort-by", k); err != nil {
			return nil, err
		}
		keys[i] = k
	}
	idx := make([]int, len(elems))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return KeyLess(keys[idx[a]], keys[idx[b]]) })
	out := make([]MalType, len(elems))
	for i, j := range idx {
		out[i] = elems[j]
	}
	return List{Val: out}, nil
}

func LoadInput(env EnvType) {
	call.Call(env, readLine)
	call.CallOverrideFN(env, "read-password", readPassword)
	call.Call(env, exit, 0, 1)

	loadInputDocs(env)
}

func new_go_error(str string) (error, error) {
	return errors.New(str), nil
}

func new_error(err MalType, cursor ...*Position) (lisperror.LispError, error) {
	if len(cursor) == 0 {
		return lisperror.NewLispError(err, nil), nil
	}
	return lisperror.NewLispError(err, cursor[0]), nil
}

// Errors/Exceptions
func throw(a MalType) (MalType, error) {
	switch a := a.(type) {
	case error:
		return nil, a
	default:
		return nil, lisperror.NewLispError(a, nil)
	}
}

func pAnic(arg MalType) {
	panic(arg)
}

func unwrap_error(err error) (MalType, error) {
	return errors.Unwrap(err), nil
}

func error_string(err error) (string, error) {
	return err.Error(), nil
}

func go_error(format string, args ...MalType) (MalType, error) {
	if len(args) == 0 {
		return errors.New(format), nil
	}
	var errorfArgs []any
	for _, i := range args {
		errorfArgs = append(errorfArgs, i)
	}
	return fmt.Errorf(format, errorfArgs...), nil
}

func istype(arg MalType) (string, error) {
	switch arg := arg.(type) {
	case nil:
		return "nil", nil
	case List:
		return "list", nil
	case HashMap:
		return "hash-map", nil
	case Vector:
		return "vector", nil
	case Set:
		return "set", nil
	case int:
		return "integer", nil
	case bool:
		return "boolean", nil
	case Symbol:
		return "symbol", nil
	case Keyword:
		return "keyword", nil
	case string:
		return "string", nil
	case MalFunc:
		return "function", nil
	case interface{ ErrorValue() MalType }:
		return "error", nil
	case Typed:
		return arg.Type(), nil
	case error:
		return "go-error", nil
	case Func:
		return "go-function", nil
	default:
		return fmt.Sprintf("unsupported(%T)", arg), nil
	}
}

func fn_q(a MalType) (MalType, error) {
	switch f := a.(type) {
	case MalFunc:
		return !f.GetMacro(), nil
	case Func:
		return true, nil
	case func([]MalType) (MalType, error):
		return true, nil
	default:
		return false, nil
	}
}

// String functions

func pr_str(a ...MalType) (MalType, error) {
	return printer.Pr_list(a, true, "", "", " "), nil
}

func str(a ...MalType) (string, error) {
	return printer.Pr_list(a, false, "", "", ""), nil
}

// sPew backs the (spew x) Lisp builtin: it deep-prints any value via go-spew.
// The lib/call dispatcher lowercases this Go name to register the symbol.
func sPew(a MalType) (MalType, error) {
	spew.Dump(a)
	return nil, nil
}

func prn(a ...MalType) (MalType, error) {
	fmt.Println(printer.Pr_list(a, true, "", "", " "))
	return nil, nil
}

func println(a ...MalType) (MalType, error) {
	fmt.Println(printer.Pr_list(a, false, "", "", " "))
	return nil, nil
}

// printNoNewline is println without the trailing newline, for composing a
// line from several prints (e.g. styled fragments).
func printNoNewline(a ...MalType) (MalType, error) {
	fmt.Print(printer.Pr_list(a, false, "", "", " "))
	return nil, nil
}

// osExit is the process-exit hook, indirected so tests can observe the
// requested status instead of terminating the test binary.
var osExit = os.Exit

// exit terminates the process with the given status code (0 when omitted),
// like Clojure's System/exit. It does not return; stdout is unbuffered (fmt
// writes straight to os.Stdout), so any preceding output is not lost.
func exit(args ...MalType) (MalType, error) {
	code := 0
	if len(args) == 1 {
		n, ok := args[0].(int)
		if !ok {
			return nil, fmt.Errorf("exit: status must be an integer, got %T", args[0])
		}
		code = n
	}
	osExit(code)
	return nil, nil
}

// gensymCounter feeds gensym; a Go atomic (rather than a lisp atom in
// the header) so the macro-writing primitive is available with only
// core loaded, before the concurrent library provides atoms.
var gensymCounter atomic.Int64

// gensym returns a fresh, hopefully-unique symbol (see "Plugging the
// Leaks", http://www.gigamonkeys.com/book/macros-defining-your-own.html).
func gensym() (Symbol, error) {
	return Symbol{Val: fmt.Sprintf("G__%d", gensymCounter.Add(1))}, nil
}

// Number functions
func time_ms() (int, error) {
	return int(time.Now().UnixMilli()), nil
}

func time_ns() (int, error) {
	return int(time.Now().UnixNano()), nil
}

// Numeric tower for the arithmetic and ordering builtins: machine ints,
// floats and *big.Int, folded variadically with Clojure-style contagion —
// int∘int stays int, a float makes the result float, a big int makes it
// big. Big ints and floats do not mix (no unambiguous conversion). The
// lisp float type is float32 (as read); folds run in float64 and convert
// back on return.

type number struct {
	f     float64
	b     *big.Int
	isBig bool
	isFlt bool
	i     int
}

func toNumber(v MalType) (number, error) {
	switch v := v.(type) {
	case int:
		return number{i: v}, nil
	case float32:
		return number{f: float64(v), isFlt: true}, nil
	case float64:
		return number{f: v, isFlt: true}, nil
	case *big.Int:
		return number{b: v, isBig: true}, nil
	default:
		return number{}, fmt.Errorf("not a number (was of type %T)", v)
	}
}

func (n number) toMal() MalType {
	switch {
	case n.isBig:
		return n.b
	case n.isFlt:
		return float32(n.f)
	default:
		return n.i
	}
}

// promote lifts a pair of numbers to their common kind.
func promote(a, b number) (number, number, error) {
	if a.isBig || b.isBig {
		if a.isFlt || b.isFlt {
			return a, b, errors.New("cannot mix a big int and a float")
		}
		if !a.isBig {
			a = number{b: big.NewInt(int64(a.i)), isBig: true}
		}
		if !b.isBig {
			b = number{b: big.NewInt(int64(b.i)), isBig: true}
		}
		return a, b, nil
	}
	if a.isFlt || b.isFlt {
		if !a.isFlt {
			a = number{f: float64(a.i), isFlt: true}
		}
		if !b.isFlt {
			b = number{f: float64(b.i), isFlt: true}
		}
		return a, b, nil
	}
	return a, b, nil
}

type numOp struct {
	onInt   func(a, b int) (int, error)
	onFloat func(a, b float64) (float64, error)
	onBig   func(a, b *big.Int) (*big.Int, error)
}

func (op numOp) apply(a, b number) (number, error) {
	a, b, err := promote(a, b)
	if err != nil {
		return number{}, err
	}
	switch {
	case a.isBig:
		r, err := op.onBig(a.b, b.b)
		return number{b: r, isBig: true}, err
	case a.isFlt:
		r, err := op.onFloat(a.f, b.f)
		return number{f: r, isFlt: true}, err
	default:
		r, err := op.onInt(a.i, b.i)
		return number{i: r}, err
	}
}

// fold reduces args with op starting from identity; with a single
// argument and unary set, it applies op to (unary, arg) instead — the
// Clojure shapes (- x) → negation and (/ x) → inverse.
func numFold(op numOp, identity number, unary *number, args []MalType) (MalType, error) {
	acc := identity
	if len(args) == 1 && unary != nil {
		n, err := toNumber(args[0])
		if err != nil {
			return nil, err
		}
		r, err := op.apply(*unary, n)
		if err != nil {
			return nil, err
		}
		return r.toMal(), nil
	}
	for i, arg := range args {
		n, err := toNumber(arg)
		if err != nil {
			return nil, err
		}
		if i == 0 && unary != nil {
			acc = n // subtraction/division fold from the first argument
			continue
		}
		acc, err = op.apply(acc, n)
		if err != nil {
			return nil, err
		}
	}
	return acc.toMal(), nil
}

var (
	addOp = numOp{
		onInt:   func(a, b int) (int, error) { return a + b, nil },
		onFloat: func(a, b float64) (float64, error) { return a + b, nil },
		onBig:   func(a, b *big.Int) (*big.Int, error) { return new(big.Int).Add(a, b), nil },
	}
	subOp = numOp{
		onInt:   func(a, b int) (int, error) { return a - b, nil },
		onFloat: func(a, b float64) (float64, error) { return a - b, nil },
		onBig:   func(a, b *big.Int) (*big.Int, error) { return new(big.Int).Sub(a, b), nil },
	}
	mulOp = numOp{
		onInt:   func(a, b int) (int, error) { return a * b, nil },
		onFloat: func(a, b float64) (float64, error) { return a * b, nil },
		onBig:   func(a, b *big.Int) (*big.Int, error) { return new(big.Int).Mul(a, b), nil },
	}
	divOp = numOp{
		onInt: func(a, b int) (int, error) {
			if b == 0 {
				return 0, errors.New("division by zero")
			}
			return a / b, nil
		},
		// float division by zero yields ±Inf, as in Go and Clojure
		onFloat: func(a, b float64) (float64, error) { return a / b, nil },
		onBig: func(a, b *big.Int) (*big.Int, error) {
			if b.Sign() == 0 {
				return nil, errors.New("division by zero")
			}
			return new(big.Int).Quo(a, b), nil
		},
	}
)

func addN(xs ...MalType) (MalType, error) { return numFold(addOp, number{i: 0}, nil, xs) }
func mulN(xs ...MalType) (MalType, error) { return numFold(mulOp, number{i: 1}, nil, xs) }
func subN(xs ...MalType) (MalType, error) {
	zero := number{i: 0}
	return numFold(subOp, number{}, &zero, xs)
}
func divN(xs ...MalType) (MalType, error) {
	one := number{i: 1}
	return numFold(divOp, number{}, &one, xs)
}

// numCmp orders two numbers after promotion (big∘float does not mix).
func numCmp(a, b number) (int, error) {
	a, b, err := promote(a, b)
	if err != nil {
		return 0, err
	}
	switch {
	case a.isBig:
		return a.b.Cmp(b.b), nil
	case a.isFlt:
		switch {
		case a.f < b.f:
			return -1, nil
		case a.f > b.f:
			return 1, nil
		default:
			return 0, nil
		}
	default:
		switch {
		case a.i < b.i:
			return -1, nil
		case a.i > b.i:
			return 1, nil
		default:
			return 0, nil
		}
	}
}

// chainCmp implements the variadic ordering builtins: true when every
// adjacent pair satisfies ok, as in Clojure ((< 1 2 3), (< x) → true).
func chainCmp(ok func(int) bool, xs []MalType) (MalType, error) {
	prev, err := toNumber(xs[0])
	if err != nil {
		return nil, err
	}
	for _, x := range xs[1:] {
		n, err := toNumber(x)
		if err != nil {
			return nil, err
		}
		c, err := numCmp(prev, n)
		if err != nil {
			return nil, err
		}
		if !ok(c) {
			return false, nil
		}
		prev = n
	}
	return true, nil
}

func ltN(xs ...MalType) (MalType, error) { return chainCmp(func(c int) bool { return c < 0 }, xs) }
func leN(xs ...MalType) (MalType, error) { return chainCmp(func(c int) bool { return c <= 0 }, xs) }
func gtN(xs ...MalType) (MalType, error) { return chainCmp(func(c int) bool { return c > 0 }, xs) }
func geN(xs ...MalType) (MalType, error) { return chainCmp(func(c int) bool { return c >= 0 }, xs) }

// rfc3339Milli is time.RFC3339 with fixed millisecond precision, matching
// the resolution of time-ms (a variable-width fraction would not sort
// lexicographically).
const rfc3339Milli = "2006-01-02T15:04:05.000Z07:00"

func time_format(ms int) (string, error) {
	return time.UnixMilli(int64(ms)).UTC().Format(rfc3339Milli), nil
}

func time_parse(s string) (int, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return 0, err
	}
	return int(t.UnixMilli()), nil
}

// time_add returns the instant millis (Unix milliseconds, UTC) shifted by
// the deltas in hm. :years :months :days go through time.AddDate, which is
// calendar-aware (variable month lengths, leap years) and normalise overflow
// like time.AddDate (Jan 31 + 1 month → March 2, Feb 29 + 1 year → March 1);
// :hours :minutes :seconds :milliseconds add a fixed duration. Any key may be omitted (0) or
// negative (shift backwards); an unknown key, or a value that is not an int,
// is an error. Times cross the lisp boundary as Unix-millisecond ints,
// matching core's time-ms/time-format. hm is read, never mutated.
func time_add(millis int, hm HashMap) (int, error) {
	var d struct{ years, months, days, hours, minutes, seconds, millis int }
	fields := map[MalType]*int{
		KW("years"):        &d.years,
		KW("months"):       &d.months,
		KW("days"):         &d.days,
		KW("hours"):        &d.hours,
		KW("minutes"):      &d.minutes,
		KW("seconds"):      &d.seconds,
		KW("milliseconds"): &d.millis,
	}
	for key, v := range hm.Items {
		p, ok := fields[key]
		if !ok {
			return 0, fmt.Errorf("time-add: unsupported parameter %q", key)
		}
		n, ok := v.(int)
		if !ok {
			return 0, fmt.Errorf("time-add: %v must be an integer (was %T)", key, v)
		}
		*p = n
	}
	t := time.UnixMilli(int64(millis)).UTC().
		AddDate(d.years, d.months, d.days).
		Add(time.Duration(d.hours)*time.Hour +
			time.Duration(d.minutes)*time.Minute +
			time.Duration(d.seconds)*time.Second +
			time.Duration(d.millis)*time.Millisecond)
	return int(t.UnixMilli()), nil
}

// time_before reports whether t1 is strictly before t2 (both Unix
// milliseconds). Lisp name: time-before? (predicate convention).
func time_before(t1, t2 int) (bool, error) {
	return t1 < t2, nil
}

// time_after reports whether t1 is strictly after t2 (both Unix
// milliseconds). Lisp name: time-after? (predicate convention).
func time_after(t1, t2 int) (bool, error) {
	return t1 > t2, nil
}

// Hash Map, Set, Vector functions
func copy_hash_map(hm HashMap) HashMap {
	new_hm := HashMap{Items: map[MalType]MalType{}}
	for k, v := range hm.Items {
		new_hm.Items[k] = v
	}
	return new_hm
}

func copy_set(s Set) Set {
	new_s := Set{Items: map[MalType]struct{}{}}
	for k, v := range s.Items {
		new_s.Items[k] = v
	}
	return new_s
}

func copy_vector(v Vector) Vector {
	return Vector{
		Val: append([]MalType{}, v.Val...),
	}
}

func assoc(a ...MalType) (MalType, error) {
	ms := a[0]
	switch ms := ms.(type) {
	case HashMap:
		if len(a) < 3 {
			return nil, errors.New("assoc requires at least 3 arguments")
		}
		if len(a)%2 != 1 {
			return nil, errors.New("assoc requires odd number of arguments")
		}
		new_hm := copy_hash_map(ms)
		for i := 1; i < len(a); i += 2 {
			key := a[i]
			if !ValidKey(key) {
				return nil, errors.New("assoc requires a scalar key (string, keyword, number, boolean or nil)")
			}
			new_hm.Items[key] = a[i+1]
		}
		return new_hm, nil
	case Vector:
		if len(a) < 3 {
			return nil, errors.New("assoc requires at least 3 arguments")
		}
		new_v := copy_vector(ms)
		for i := 1; i < len(a); i += 2 {
			key := a[i]
			keyInt, ok := key.(int)
			if !ok {
				return nil, errors.New("assoc called with non-int key")
			}
			new_v.Val[keyInt] = a[i+1]
		}
		return new_v, nil
	case Set:
		if len(a) < 2 {
			return nil, errors.New("assoc requires at least 2 arguments")
		}
		new_s := copy_set(ms)
		for _, value := range a[1:] {
			if !ValidKey(value) {
				return nil, errors.New("assoc requires a scalar key (string, keyword, number, boolean or nil)")
			}
			new_s.Items[value] = struct{}{}
		}
		return new_s, nil
	default:
		return nil, fmt.Errorf("assoc called on non-hash map and non-set (it was %T)", ms)
	}
}

func dissoc(a ...MalType) (MalType, error) {
	if len(a) < 2 {
		return nil, errors.New("dissoc requires at least 3 arguments")
	}
	ms := a[0]
	switch ms := ms.(type) {
	case HashMap:
		new_hm := copy_hash_map(ms)
		for i := 1; i < len(a); i += 1 {
			key := a[i]
			if !ValidKey(key) {
				return nil, errors.New("dissoc requires a scalar key (string, keyword, number, boolean or nil)")
			}
			delete(new_hm.Items, key)
		}
		return new_hm, nil
	case Set:
		new_s := copy_set(ms)
		for _, value := range a[1:] {
			if !ValidKey(value) {
				return nil, errors.New("dissoc requires a scalar key (string, keyword, number, boolean or nil)")
			}
			delete(new_s.Items, value)
		}
		return new_s, nil
	default:
		return nil, errors.New("assoc called on non-hash map and non-set")
	}
}

func get(hm, key MalType) (MalType, error) {
	if Nil_Q(hm) {
		return nil, nil
	}
	ms := hm
	switch ms := ms.(type) {
	case HashMap:
		if !ValidKey(key) {
			return nil, errors.New("get on a hash-map requires a scalar key (string, keyword, number, boolean or nil)")
		}
		return ms.Items[key], nil
	case Vector:
		i, ok := key.(int)
		if !ok {
			return nil, errors.New("get on a vector requires an int key")
		}
		return ms.Val[i], nil
	case List:
		i, ok := key.(int)
		if !ok {
			return nil, errors.New("get on a list requires an int key")
		}
		return ms.Val[i], nil
	case Set:
		if !ValidKey(key) {
			return nil, errors.New("get on a set requires a scalar key (string, keyword, number, boolean or nil)")
		}
		if _, ok := ms.Items[key]; ok {
			return key, nil
		}
		return nil, nil
	default:
		return nil, errors.New("get called on non-hash map and a non-set")
	}
}

func get_in(hm, _pathVector MalType) (MalType, error) {
	if Nil_Q(hm) {
		return nil, nil
	}
	pathVector, ok := _pathVector.(Vector)
	if !ok {
		return nil, errors.New("get-in index must be a vector")
	}
	return _getIn(hm, pathVector)
}

func _getIn(argMapOrVector MalType, posVector Vector) (MalType, error) {
	switch len(posVector.Val) {
	case 0:
		return argMapOrVector, nil
	case 1:
		index := posVector.Val[0]
		return get(argMapOrVector, index)
	default:
		index := posVector.Val[0]
		rest := Vector{Val: posVector.Val[1:]}
		var branch MalType
		switch argMapOrVector := argMapOrVector.(type) {
		case HashMap:
			branch = argMapOrVector.Items[index]
			if branch == nil {
				branch = HashMap{}
			}
		case List:
			branch = argMapOrVector.Val[index.(int)]
			if branch == nil {
				branch = List{}
			}
		case Vector:
			branch = argMapOrVector.Val[index.(int)]
			if branch == nil {
				branch = Vector{}
			}
		}
		return _getIn(branch, rest)
	}
}

func update(ctx context.Context, hm, pos, f MalType) (MalType, error) {
	if Nil_Q(hm) {
		return nil, nil
	}
	return _update(ctx, hm, pos, f)
}

func _update(ctx context.Context, argMapOrVector, index, f MalType) (MalType, error) {
	switch argMapOrVector := argMapOrVector.(type) {
	case HashMap:
		res, err := Apply(ctx, f, []MalType{argMapOrVector.Items[index]})
		if err != nil {
			return nil, err
		}
		return assoc(argMapOrVector, index, res)
	case Vector:
		res, err := Apply(ctx, f, []MalType{argMapOrVector.Val[index.(int)]})
		if err != nil {
			return nil, err
		}
		return assoc(argMapOrVector, index, res)
	default:
		return nil, fmt.Errorf("expected vector or hash-map but got %T", argMapOrVector)
	}
}

func update_in(ctx context.Context, seq MalType, posVector Vector, f MalType) (MalType, error) {
	if Nil_Q(seq) {
		return nil, nil
	}
	return _updateIn(ctx, seq, posVector, f)
}

func _updateIn(ctx context.Context, seq MalType, posVector Vector, f MalType) (MalType, error) {
	switch len(posVector.Val) {
	case 0:
		return seq, nil
	case 1:
		index := posVector.Val[0]
		return _update(ctx, seq, index, f)
	default:
		index := posVector.Val[0]
		rest := Vector{Val: posVector.Val[1:]}
		var branch MalType
		switch seq := seq.(type) {
		case HashMap:
			branch = seq.Items[index]
			if branch == nil {
				branch = HashMap{}
			}
			inner, err := _updateIn(ctx, branch.(HashMap), rest, f)
			if err != nil {
				return nil, err
			}
			return assoc(seq, index, inner)
		case Vector:
			branch = seq.Val[index.(int)]
			if branch == nil {
				branch = Vector{}
			}
			inner, err := _updateIn(ctx, branch.(Vector), rest, f)
			if err != nil {
				return nil, err
			}
			return assoc(seq, index, inner)
		default:
			return nil, fmt.Errorf("type %T not supported of index of %T", index, seq)
		}
	}
}

func assoc_in(hm MalType, posVector Vector, data MalType) (MalType, error) {
	return _assocIn(hm, posVector, data)
}

func _assocIn(argMapOrVector MalType, posVector Vector, newValue MalType) (MalType, error) {
	switch len(posVector.Val) {
	case 0:
		return argMapOrVector, nil
	case 1:
		index := posVector.Val[0]
		return assoc(argMapOrVector, index, newValue)
	default:
		index := posVector.Val[0]
		rest := Vector{Val: posVector.Val[1:]}
		var branch MalType
		switch argMapOrVector := argMapOrVector.(type) {
		case HashMap:
			branch = argMapOrVector.Items[index]
			if branch == nil {
				branch = HashMap{}
			}
		case Vector:
			branch = argMapOrVector.Val[index.(int)]
			if branch == nil {
				branch = Vector{}
			}
		}
		inner, err := _assocIn(branch, rest, newValue)
		if err != nil {
			return nil, err
		}
		return assoc(argMapOrVector, index, inner)
	}
}

func contains_Q(hm MalType, key MalType) (bool, error) {
	if Nil_Q(hm) {
		return false, nil
	}
	switch hm := hm.(type) {
	case HashMap:
		_, ok := hm.Items[key]
		return ok, nil
	case Set:
		_, ok := hm.Items[key]
		return ok, nil
	default:
		return false, errors.New("get called on non-hash map and a non-set")
	}
}

func keys(hm MalType) (List, error) {
	switch hm := hm.(type) {
	case HashMap:
		slc := []MalType{}
		for k := range hm.Items {
			slc = append(slc, k)
		}
		return List{Val: slc}, nil
	default:
		return List{}, errors.New("keys called on non-hash map")
	}
}

func vals(hm MalType) (List, error) {
	if !Q[HashMap](hm) {
		return List{}, errors.New("vals called on non-hash map")
	}
	slc := []MalType{}
	for _, v := range hm.(HashMap).Items {
		slc = append(slc, v)
	}
	return List{Val: slc}, nil
}

// Sequence functions

func cons(seq, app MalType) (List, error) {
	lst, e := GetSlice(app)
	if e != nil {
		return List{}, e
	}
	return List{Val: append([]MalType{seq}, lst...)}, nil
}

func concat(a ...MalType) (MalType, error) {
	if len(a) == 0 {
		return List{}, nil
	}
	slc1, e := GetSlice(a[0])
	if e != nil {
		return nil, e
	}
	for i := 1; i < len(a); i += 1 {
		slc2, e := GetSlice(a[i])
		if e != nil {
			return nil, e
		}
		slc1 = append(slc1, slc2...)
	}
	return List{Val: slc1}, nil
}

func vec(seq MalType) (MalType, error) {
	array, meta, err := ConvertFrom(seq)
	if err != nil {
		return nil, err
	}
	return Vector{
		Val:  array,
		Meta: meta,
	}, nil
}

func nth(seq MalType, idx int) (MalType, error) {
	// Maps and sets seq (via GetSlice) but are not indexed collections,
	// and their iteration order is unspecified — indexing into them would
	// return a nondeterministic element. Clojure errors here too.
	switch seq.(type) {
	case HashMap:
		return nil, errors.New("nth not supported on a hash-map")
	case Set:
		return nil, errors.New("nth not supported on a set")
	}
	slc, e := GetSlice(seq)
	if e != nil {
		return nil, e
	}
	if idx < len(slc) {
		return slc[idx], nil
	} else {
		return nil, errors.New("nth: index out of range")
	}
}

func first(seq MalType) (MalType, error) {
	if seq == nil {
		return nil, nil
	}
	slc, e := GetSlice(seq)
	if e != nil {
		return nil, e
	}
	if len(slc) == 0 {
		return nil, nil
	}
	return slc[0], nil
}

func rest(seq MalType) (MalType, error) {
	if seq == nil {
		return List{}, nil
	}
	slc, e := GetSlice(seq)
	if e != nil {
		return nil, e
	}
	if len(slc) == 0 {
		return List{}, nil
	}
	return List{Val: slc[1:]}, nil
}

func empty_Q(seq MalType) (bool, error) {
	switch seq := seq.(type) {
	case List:
		return len(seq.Val) == 0, nil
	case Vector:
		return len(seq.Val) == 0, nil
	case HashMap:
		return len(seq.Items) == 0, nil
	case Set:
		return len(seq.Items) == 0, nil
	case nil:
		return true, nil
	default:
		return false, errors.New("empty? called on non-sequence")
	}
}

func count(seq MalType) (int, error) {
	switch seq := seq.(type) {
	case List:
		return len(seq.Val), nil
	case Vector:
		return len(seq.Val), nil
	case HashMap:
		return len(seq.Items), nil
	case Set:
		return len(seq.Items), nil
	case nil:
		return 0, nil
	default:
		return 0, fmt.Errorf("count called on non-sequence type %T", seq)
	}
}

func apply(ctx context.Context, a ...MalType) (MalType, error) {
	if len(a) < 2 {
		return nil, errors.New("apply requires at least 2 args")
	}
	f := a[0]
	args := append(
		[]MalType{},
		a[1:len(a)-1]...,
	)
	last, e := GetSlice(a[len(a)-1])
	if e != nil {
		return nil, e
	}
	args = append(args, last...)
	return Apply(ctx, f, args)
}

func mAp(ctx context.Context, f, seq MalType) (MalType, error) {
	results := []MalType{}
	args, e := GetSlice(seq)
	if e != nil {
		return nil, e
	}
	for _, arg := range args {
		res, e := Apply(ctx, f, []MalType{arg})
		if e != nil {
			return nil, e
		}
		results = append(results, res)
	}
	return List{Val: results}, nil
}

func conj(a ...MalType) (MalType, error) {
	if len(a) == 0 {
		return nil, nil
	}
	seq := a[0]
	switch seq := seq.(type) {
	case List:
		new_slc := []MalType{}
		for i := len(a) - 1; i > 0; i -= 1 {
			new_slc = append(new_slc, a[i])
		}
		return List{Val: append(new_slc, seq.Val...)}, nil
	case Vector:
		new_slc := append(seq.Val, a[1:]...)
		return Vector{Val: new_slc}, nil
	case HashMap:
		new_hm := copy_hash_map(seq)
		// Entry form: each extra argument is a [k v] pair or a whole map —
		// the shape `into` feeds via `(reduce conj m from)`. Detected from
		// the first extra argument; otherwise fall back to the flat
		// key/value form `(conj m k1 v1 …)`.
		if len(a) > 1 && isMapEntry(a[1]) {
			for _, entry := range a[1:] {
				if err := conjMapEntry(new_hm, entry); err != nil {
					return nil, err
				}
			}
			return new_hm, nil
		}
		if len(a)%2 != 1 {
			return nil, errors.New("conj called with on a hash map requires an odd number of arguments")
		}
		for i := 1; i < len(a); i += 2 {
			key := a[i]
			if !ValidKey(key) {
				return nil, errors.New("conj requires a scalar key (string, keyword, number, boolean or nil)")
			}
			new_hm.Items[key] = a[i+1]
		}
		return new_hm, nil
	case Set:
		new_s := copy_set(seq)
		for _, key := range a[1:] {
			if !ValidKey(key) {
				return nil, errors.New("conj requires a scalar key (string, keyword, number, boolean or nil)")
			}
			new_s.Items[key] = struct{}{}
		}
		return new_s, nil
	default:
		return nil, errors.New("conj called on non-hash map and a non-list and a non-set and a non-vector")
	}
}

// isMapEntry reports whether v is what conj accepts as a single map entry:
// a two-element vector/list [k v], or a map to be merged in.
func isMapEntry(v MalType) bool {
	switch e := v.(type) {
	case Vector:
		return len(e.Val) == 2
	case List:
		return len(e.Val) == 2
	case HashMap:
		return true
	}
	return false
}

// conjMapEntry adds one entry — a [key value] pair or a whole map — to hm
// in place. Keys must be valid scalar keys, as elsewhere for maps.
func conjMapEntry(hm HashMap, entry MalType) error {
	if e, ok := entry.(HashMap); ok {
		for k, v := range e.Items {
			hm.Items[k] = v
		}
		return nil
	}
	slc, err := GetSlice(entry)
	if err != nil || len(slc) != 2 {
		return errors.New("conj: map entry must be a [key value] pair or a map")
	}
	key := slc[0]
	if !ValidKey(key) {
		return errors.New("conj requires a scalar key (string, keyword, number, boolean or nil)")
	}
	hm.Items[key] = slc[1]
	return nil
}

func seq(seq MalType) (MalType, error) {
	switch arg := seq.(type) {
	case List:
		if len(arg.Val) == 0 {
			return nil, nil
		}
		return arg, nil
	case Vector:
		if len(arg.Val) == 0 {
			return nil, nil
		}
		return List{Val: arg.Val}, nil
	case HashMap:
		if len(arg.Items) == 0 {
			return nil, nil
		}
		slc, _ := GetSlice(arg)
		return List{Val: slc}, nil
	case Set:
		if len(arg.Items) == 0 {
			return nil, nil
		}
		slc, _ := GetSlice(arg)
		return List{Val: slc}, nil
	case string:
		if len(arg) == 0 {
			return nil, nil
		}
		new_slc := []MalType{}
		for _, ch := range strings.Split(arg, "") {
			new_slc = append(new_slc, ch)
		}
		return List{Val: new_slc}, nil
	case nil:
		return nil, nil
	}
	return nil, errors.New("seq requires string or list or vector or hash-map or set or nil")
}

// Metadata functions
func with_meta(obj, meta MalType) (MalType, error) {
	switch tobj := obj.(type) {
	case List:
		return List{Val: tobj.Val, Meta: meta}, nil
	case Vector:
		return Vector{Val: tobj.Val, Meta: meta}, nil
	case HashMap:
		return HashMap{Items: tobj.Items, Meta: meta}, nil
	case Set:
		return Set{Items: tobj.Items, Meta: meta}, nil
	case Func:
		return Func{Fn: tobj.Fn, Meta: meta, Doc: tobj.Doc, Arglist: tobj.Arglist, Cursor: tobj.Cursor}, nil
	case MalFunc:
		fn := tobj
		fn.Meta = meta
		return fn, nil
	default:
		return nil, errors.New("with-meta not supported on type")
	}
}

func meta(meta MalType) (MalType, error) {
	switch meta := meta.(type) {
	case List:
		return meta.Meta, nil
	case Vector:
		return meta.Meta, nil
	case HashMap:
		return meta.Meta, nil
	case Set:
		return meta.Meta, nil
	case Func:
		return meta.Meta, nil
	case MalFunc:
		return meta.Meta, nil
	default:
		return nil, errors.New("meta not supported on type")
	}
}

// docString returns the documentation string of a function or macro:
// a Go builtin's Doc field (attached with call.Doc), or the {:doc "…"}
// docstring metadata of a lisp function/macro (see the defn macro).
// Returns nil when undocumented. Kept off `meta` so native functions
// keep nil metadata (kanaka/mal compatibility).
func docString(v MalType) (MalType, error) {
	switch f := v.(type) {
	case Func:
		if f.Doc != "" {
			return f.Doc, nil
		}
	case MalFunc:
		if hm, ok := f.Meta.(HashMap); ok {
			if d, ok := hm.Items[KW("doc")]; ok {
				return d, nil
			}
		}
	}
	return nil, nil
}

func deref(ctx context.Context, ref Dereferable) (MalType, error) {
	return ref.Deref(ctx)
}

// Core extended

func uUid() (string, error) {
	return uuid.New().String(), nil
}

// subs returns the substring of s from start (inclusive) to end
// (exclusive), counted in Unicode code points (runes) like Clojure's
// subs. With two arguments it runs to the end of the string.
func subs(args ...MalType) (MalType, error) {
	s, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("subs requires a string (it was %T)", args[0])
	}
	runes := []rune(s)
	start, ok := args[1].(int)
	if !ok {
		return nil, fmt.Errorf("subs start must be an int (it was %T)", args[1])
	}
	end := len(runes)
	if len(args) == 3 {
		end, ok = args[2].(int)
		if !ok {
			return nil, fmt.Errorf("subs end must be an int (it was %T)", args[2])
		}
	}
	if start < 0 || start > end || end > len(runes) {
		return nil, fmt.Errorf("subs index out of range (start=%d end=%d len=%d)", start, end, len(runes))
	}
	return string(runes[start:end]), nil
}

func split(str, sep string) (Vector, error) {
	l := strings.Split(str, sep)
	slc := make([]MalType, len(l))
	for i, v := range l {
		slc[i] = v
	}

	return Vector{Val: slc}, nil
}

func rename_keys(data, alternative HashMap) (HashMap, error) {
	output := map[MalType]MalType{}
	for k, v := range data.Items {
		newKey, ok := alternative.Items[k]
		if ok {
			if !ValidKey(newKey) {
				return HashMap{}, errors.New("rename-keys: new key must be a string or keyword")
			}
			output[newKey] = v
		} else {
			output[k] = v
		}
	}
	return HashMap{
		Items:  output,
		Meta:   data.Meta,
		Cursor: data.Cursor,
	}, nil
}

func assert(a ...MalType) (MalType, error) {
	var a0, a1 MalType
	switch len(a) {
	case 1:
		a0 = a[0]
	case 2:
		a0 = a[0]
		a1 = a[1]
	default:
		return nil, errors.New("one or two parameters required")
	}

	switch a0 := a0.(type) {
	case bool:
		if a0 {
			return nil, nil
		}
	default:
		return nil, nil
	case nil:
	}

	// assertion failed
	switch a1 := a1.(type) {
	case nil:
		switch a0.(type) {
		case nil:
			return nil, errors.New("assertion failed: nil")
		case bool:
			return nil, errors.New("assertion failed: false")
		default:
			return nil, errors.New("internal error")
		}
	case string:
		return nil, errors.New(a1)
	default:
		return nil, lisperror.NewLispError(a1, nil)
	}
}

func mErge(_hm0, _hm1 MalType) (MalType, error) {
	if _hm0 == nil && _hm1 == nil {
		return nil, nil
	}

	var hm0 HashMap
	if _hm0 != nil {
		var ok bool
		hm0, ok = _hm0.(HashMap)
		if !ok {
			return nil, errors.New("expected hash map")
		}
	}
	var hm1 HashMap
	if _hm1 != nil {
		var ok bool
		hm1, ok = _hm1.(HashMap)
		if !ok {
			return nil, errors.New("expected hash map")
		}
	}
	if hm0.Items == nil && hm1.Items == nil {
		return nil, nil
	}
	merged := HashMap{
		Items: make(map[MalType]MalType),
	}
	for k, v := range hm0.Items {
		merged.Items[k] = v
	}
	for k, v := range hm1.Items {
		merged.Items[k] = v
	}
	return merged, nil
}

func json_encode(obj MalType) (MalType, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func hash_map(a ...MalType) (MalType, error) {
	switch len(a) {
	case 0:
		return HashMap{}, nil
	case 1:
		return a[0].(marshaler.HashMap).MarshalHashMap()
	default:
		return NewHashMap(nil, List{Val: a})
	}
}

func hash_map_decode(objFactory marshaler.FactoryHashMap, hm HashMap) (MalType, error) {
	return objFactory.FromHashMap(hm)
}

func JSON_Decode(obj, bytesIn MalType) (MalType, error) {
	var b []byte

	switch a := bytesIn.(type) {
	case string:
		b = []byte(a)
	case []byte:
		b = a
	default:
		return nil, fmt.Errorf("unsupported type %T", a)
	}

	switch value := obj.(type) {
	case marshaler.FactoryJSON:
		return value.FromJSON(b)
	case List:
		v := []interface{}{}
		err := json.Unmarshal(b, &v)
		if err != nil {
			return nil, err
		}
		return array2list(v), nil
	case Vector:
		v := []interface{}{}
		err := json.Unmarshal(b, &v)
		if err != nil {
			return nil, err
		}
		return array2vector(v), nil
	case HashMap:
		v := map[string]interface{}{}
		d := json.NewDecoder(bytes.NewReader(b))
		d.UseNumber()
		err := d.Decode(&v)
		if err != nil {
			return nil, err
		}
		return map2hashmap(v), nil
	case Set:
		v := []interface{}{}
		err := json.Unmarshal(b, &v)
		if err != nil {
			return nil, err
		}
		return NewSet(array2vector(v))
	default:
		return nil, fmt.Errorf("type %T cannot be decoded", value)
	}
}

func map2hashmap(m map[string]interface{}) HashMap {
	hm := HashMap{
		Items: map[MalType]MalType{},
		Meta:  nil,
	}
	for k, v := range m {
		switch v := v.(type) {
		case map[string]interface{}:
			hm.Items[k] = map2hashmap(v)
		case []interface{}:
			hm.Items[k] = array2vector(v)
		default:
			hm.Items[k] = v
		}
	}
	return hm
}

func array2vector(a []interface{}) Vector {
	l := Vector{
		Val:  []MalType{},
		Meta: nil,
	}
	for _, v := range a {
		switch v := v.(type) {
		case map[string]interface{}:
			l.Val = append(l.Val, map2hashmap(v))
		case []interface{}:
			l.Val = append(l.Val, array2vector(v))
		default:
			l.Val = append(l.Val, v)
		}
	}
	return l
}

func array2list(a []interface{}) List {
	l := List{
		Val:  []MalType{},
		Meta: nil,
	}
	for _, v := range a {
		switch v := v.(type) {
		case map[string]interface{}:
			l.Val = append(l.Val, map2hashmap(v))
		case []interface{}:
			l.Val = append(l.Val, array2vector(v))
		default:
			l.Val = append(l.Val, v)
		}
	}
	return l
}

// stdinScanner is created once and reused across readLine calls: a fresh
// bufio.Scanner per call would discard any input it read past the current
// line, dropping subsequent lines in a REPL loop.
var (
	stdinScanner     *bufio.Scanner
	stdinScannerOnce sync.Once
)

// readLine prints prompt and reads one line from stdin. On end of input
// (Ctrl-D) or a read error it returns nil — as kanaka/mal does — so a
// REPL loop reading `(readline …)` can tell EOF apart from an empty line
// (which returns "") and terminate.
func readLine(prompt string) (MalType, error) {
	stdinScannerOnce.Do(func() {
		stdinScanner = bufio.NewScanner(os.Stdin)
	})
	fmt.Print(prompt)
	if !stdinScanner.Scan() {
		return nil, nil
	}
	return stdinScanner.Text(), nil
}

// readPassword prints prompt and reads one line from stdin with terminal
// echo disabled, returning the text without the trailing newline (nil on
// EOF/error, like readLine). When stdin is not a terminal (piped input,
// tests, CI) there is no echo to suppress, so it falls back to a normal
// line read. The prompt goes to stderr so capturing stdout (e.g.
// `pw=$(lisp get-secret.lisp)`) does not swallow it.
func readPassword(prompt string) (MalType, error) {
	fd := int(os.Stdin.Fd())
	fmt.Fprint(os.Stderr, prompt)
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr) // the user's Enter was not echoed
		if err != nil {
			return nil, nil
		}
		return string(b), nil
	}
	stdinScannerOnce.Do(func() {
		stdinScanner = bufio.NewScanner(os.Stdin)
	})
	if !stdinScanner.Scan() {
		return nil, nil
	}
	return stdinScanner.Text(), nil
}

func sleep(ctx context.Context, ms int) error {
	select {
	case <-ctx.Done():
		return errors.New("timeout while evaluating expression")
	case <-time.After(time.Millisecond * time.Duration(ms)):
		return nil
	}
}

func str2binary(str string) ([]byte, error) {
	return []byte(str), nil
}

func binary2str(b []byte) (string, error) {
	return string(b), nil
}

func bAse64(b []byte) (string, error) {
	return base64.StdEncoding.EncodeToString(b), nil
}

func unbase64(str string) ([]byte, error) {
	result, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func rAnge(from, to int) (Vector, error) {
	var value []MalType
	for i := from; i < to; i++ {
		value = append(value, i)
	}
	return Vector{Val: value}, nil
}
