package lisp

import (
	"context"
	"fmt"

	. "github.com/jig/lisp/types"
)

// Take either a MalFunc or regular function and apply it to the
// arguments
func Apply(ctx context.Context, f_mt MalType, a []MalType) (MalType, error) {
	switch f := f_mt.(type) {
	case MalFunc:
		env, e := f.GenEnv(f.Env, f.Params, List{
			Val:    a,
			Cursor: f.Cursor,
		})
		if e != nil {
			return nil, e
		}
		return f.Eval(ctx, f.Exp, env)
	case Func:
		return f.Fn(ctx, a)
	case func([]MalType) (MalType, error):
		return f(a)
	default:
		return nil, fmt.Errorf("invalid function to Apply (%T)", f)
	}
}
