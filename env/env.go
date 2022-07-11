package env

import (
	"errors"
	"sync"

	. "github.com/jig/lisp/types"
)

type Env struct {
	data  sync.Map
	outer EnvType
	trace int
}

func NewEnv(outer EnvType, binds_mt MalType, exprs_mt MalType) (EnvType, error) {
	env := &Env{
		data: sync.Map{},
	}
	if outer != nil {
		env.outer = outer
		if outer.Trace() > 0 {
			env.SetTrace(outer.Trace() + 1)
		}
	}

	if binds_mt != nil && exprs_mt != nil {
		binds, e := GetSlice(binds_mt)
		if e != nil {
			return nil, e
		}
		exprs, e := GetSlice(exprs_mt)
		if e != nil {
			return nil, e
		}
		// Return a new Env with symbols in binds bound to
		// corresponding values in exprs
		var varargs bool
		i := 0
		for ; i < len(binds); i++ {
			if Symbol_Q(binds[i]) && binds[i].(Symbol).Val == "&" {
				env.data.Store(binds[i+1].(Symbol).Val, List{Val: exprs[i:]})
				varargs = true
				break
			} else {
				if i == len(exprs) {
					return nil, errors.New("not enough arguments passed")
				}
				env.data.Store(binds[i].(Symbol).Val, exprs[i])
			}
		}
		if !varargs && len(exprs) != i {
			return nil, errors.New("too many arguments passed")
		}
	}
	//return &et, nil
	return env, nil
}

func (e *Env) Find(key Symbol) EnvType {
	if _, ok := e.data.Load(key.Val); ok {
		return e
	} else if e.outer != nil {
		return e.outer.Find(key)
	} else {
		return nil
	}
}

func (e *Env) Set(key Symbol, value MalType) MalType {
	e.data.Store(key.Val, value)
	return value
}

func (e *Env) Remove(key Symbol) error {
	if _, ok := e.data.LoadAndDelete(key.Val); !ok {
		return errors.New("symbol not found")
	}
	return nil
}

func (e *Env) Get(key Symbol) (MalType, error) {
	env := e.Find(key)
	if env == nil {
		return nil, errors.New("'" + key.Val + "' not found")
	}

	v, _ := env.(*Env).data.Load(key.Val)
	return v, nil
}

func (e *Env) Map() *sync.Map {
	return &e.data
}

func (e *Env) Trace() int {
	return e.trace
}

func (e *Env) SetTrace(newVal int) {
	e.trace = newVal
}
