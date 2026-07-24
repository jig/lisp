package env

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/jig/lisp/lisperror"
	"github.com/jig/lisp/types"
)

type Env struct {
	mu    *sync.RWMutex
	data  map[string]interface{}
	outer *Env
}

func NewEnv() types.EnvType {
	return _newEnv()
}

func NewSubordinateEnv(outer types.EnvType) types.EnvType {
	return _newSubordinateEnv(outer.(*Env))
}

func NewSubordinateEnvWithBinds(outer types.EnvType, binds_mt types.MalType, exprs_mt types.MalType) (types.EnvType, error) {
	return _newSubordinateEnvWithBinds(outer.(*Env), binds_mt, exprs_mt)
}

func _newEnv() *Env {
	return &Env{
		data: map[string]interface{}{},
		mu:   &sync.RWMutex{},
	}
}

func _newSubordinateEnv(outer *Env) *Env {
	env := _newEnv()
	env.outer = outer
	return env
}

func _newSubordinateEnvWithBinds(outer *Env, binds_mt types.MalType, exprs_mt types.MalType) (types.EnvType, error) {
	env := _newSubordinateEnv(outer)

	if binds_mt != nil && exprs_mt != nil {
		binds, e := types.GetSlice(binds_mt)
		if e != nil {
			return nil, e
		}
		exprs, e := types.GetSlice(exprs_mt)
		if e != nil {
			return nil, e
		}
		// Return a new Env with types.symbols in binds bound to
		// corresponding values in exprs
		var varargs bool
		i := 0
		for ; i < len(binds); i++ {
			if types.Q[types.Symbol](binds[i]) && binds[i].(types.Symbol).Val == "&" {
				if i+1 >= len(binds) {
					return nil, lisperror.NewLispError(errors.New("missing symbol after '&' in binding list"), nil)
				}
				if err := env.bindPattern(binds[i+1], types.List{Val: exprs[i:]}); err != nil {
					return nil, err
				}
				varargs = true
				break
			} else {
				if i == len(exprs) {
					return nil, lisperror.NewLispError(fmt.Errorf("too few arguments passed (%d binds, %d arguments passed)", len(binds), len(exprs)), nil)
				}
				if err := env.bindPattern(binds[i], exprs[i]); err != nil {
					return nil, err
				}
			}
		}
		if !varargs && len(exprs) != i {
			return nil, lisperror.NewLispError(fmt.Errorf("too many arguments passed (%d binds, %d arguments passed)", len(binds), len(exprs)), nil)
		}
	}
	return env, nil
}

// Bind binds pattern to value in env: a symbol binds directly, a vector
// destructures value positionally (Clojure sequential destructuring).
// Used by binding special forms (let) on their own environments.
func Bind(env types.EnvType, pattern, value types.MalType) error {
	return env.(*Env).bindPattern(pattern, value)
}

// bindPattern binds one binding-form element. A symbol binds the value as
// is. A vector is a sequential destructuring pattern: each element binds
// the corresponding element of value (which must be seqable, or nil),
// recursively; missing elements bind nil, extra elements are ignored, and
// `&` binds the remainder as a list (Clojure semantics, except that an
// empty remainder is the empty list, as with varargs, rather than nil).
func (e *Env) bindPattern(pattern, value types.MalType) error {
	switch pattern := pattern.(type) {
	case types.Symbol:
		// Direct write: binding always targets a freshly created,
		// not-yet-shared environment, as the previous inline binds did.
		e.data[pattern.Val] = value
		return nil
	case types.Vector:
		var elems []types.MalType
		if value != nil {
			var err error
			elems, err = types.GetSlice(value)
			if err != nil {
				return lisperror.NewLispError(fmt.Errorf("cannot destructure a %T as a sequence", value), nil)
			}
		}
		for i := 0; i < len(pattern.Val); i++ {
			if types.Q[types.Symbol](pattern.Val[i]) && pattern.Val[i].(types.Symbol).Val == "&" {
				if i+1 >= len(pattern.Val) {
					return lisperror.NewLispError(errors.New("missing symbol after '&' in binding list"), nil)
				}
				rest := types.List{}
				if i < len(elems) {
					rest = types.List{Val: elems[i:]}
				}
				return e.bindPattern(pattern.Val[i+1], rest)
			}
			var v types.MalType
			if i < len(elems) {
				v = elems[i]
			}
			if err := e.bindPattern(pattern.Val[i], v); err != nil {
				return err
			}
		}
		return nil
	default:
		return lisperror.NewLispError(fmt.Errorf("binding list expected symbol or vector, got %T", pattern), nil)
	}
}

func (e *Env) Find(key types.Symbol) types.EnvType {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.FindNT(key)
}

func (e *Env) Set(key types.Symbol, value types.MalType) types.MalType {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.SetNT(key, value)
}

func (e *Env) Remove(key types.Symbol) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.RemoveNT(key)
}

func (e *Env) Get(key types.Symbol) (types.MalType, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.GetNT(key)
}

func (e *Env) Update(key types.Symbol, f func(types.MalType) (types.MalType, error)) (types.MalType, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	v, _ := e.GetNT(key)
	newV, err := f(v)
	if err != nil {
		return nil, err
	}
	return e.SetNT(key, newV), nil
}

func (e *Env) Symbols(newLine [][]rune, lastPartial string) [][]rune {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var localNewLine []string

	for key := range e.data {
		if strings.HasPrefix(key, lastPartial) {
			localNewLine = append(localNewLine, key[len(lastPartial):])
		}
	}
	sort.Strings(localNewLine)

	// append localNewLine to newLine
	for _, s := range localNewLine {
		newLine = append(newLine, []rune(s))
	}

	if e.outer != nil {
		return e.outer.Symbols(newLine, lastPartial)
	}
	return newLine
}

// Outer returns the enclosing environment, or nil for the root.
func (e *Env) Outer() types.EnvType {
	if e.outer == nil {
		return nil
	}
	return e.outer
}

// LocalSymbols returns the names bound in this environment only (the
// outer chain is not consulted), sorted. Used by the require library to
// enumerate a module's top-level definitions.
func (e *Env) LocalSymbols() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]string, 0, len(e.data))
	for key := range e.data {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func (e *Env) FindNT(key types.Symbol) types.EnvType {
	if _, ok := e.data[key.Val]; ok {
		return e
	} else if e.outer != nil {
		// do-not-use-FindNT-here
		return e.outer.Find(key)
	} else {
		return nil
	}
}

func (e *Env) SetNT(key types.Symbol, value types.MalType) types.MalType {
	e.data[key.Val] = value
	return value
}

func (e *Env) GetNT(key types.Symbol) (types.MalType, error) {
	if v, ok := e.data[key.Val]; ok {
		return v, nil
	} else if e.outer != nil {
		// do-not-use-GetNT-here
		return e.outer.Get(key)
	} else {
		return nil, lisperror.NewLispError(fmt.Errorf("symbol '%w' not found", errors.New(key.Val)), key)
	}
}

func (e *Env) RemoveNT(key types.Symbol) error {
	if _, ok := e.data[key.Val]; !ok {
		return lisperror.NewLispError(fmt.Errorf("symbol '%w' not found", errors.New(key.Val)), key)
	}
	delete(e.data, key.Val)
	return nil
}
