package concurrent

import (
	"context"

	"github.com/jig/lisp/lib/call"
	"github.com/jig/lisp/types"
	. "github.com/jig/lisp/types"
)

func Load(env types.EnvType) {
	call.Call(env, future_call)
	call.Call(env, future_cancel)
	call.CallOverrideFN(env, "future-cancelled?", func(f *Future) (bool, error) { return f.Cancelled, nil })
	call.CallOverrideFN(env, "future-done?", func(f *Future) (bool, error) { return f.Done, nil })
	call.CallOverrideFN(env, "future?", func(f MalType) (bool, error) { return Q[*Future](f), nil })
}

func future_call(ctx context.Context, f MalFunc) (*Future, error) {
	return NewFuture(ctx, f), nil
}

func future_cancel(f *Future) (bool, error) {
	return f.Cancel(), nil
}
