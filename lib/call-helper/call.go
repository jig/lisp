package call

import (
	"context"
	"fmt"

	. "github.com/jig/lisp/types"
)

// callXX functions check the number of arguments
func Call0e(f func([]MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 0 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 0)", len(args))
		}
		return f(args)
	}
}

func Call1e(f func([]MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 1)", len(args))
		}
		return f(args)
	}
}

func Call2e(f func([]MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 2 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 2)", len(args))
		}
		return f(args)
	}
}

func CallNe(f func([]MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	// just for documenting purposes, does not check anything
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		return f(args)
	}
}

func Call1b(f func(MalType) bool) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 1)", len(args))
		}
		return f(args[0]), nil
	}
}

func Call2b(f func(MalType, MalType) bool) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 2 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 2)", len(args))
		}
		return f(args[0], args[1]), nil
	}
}

func CallNeC(f func([]MalType, *context.Context) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	// just for documenting purposes, does not check anything
	return func(args []MalType, ctx *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		return f(args, ctx)
	}
}

func Call0eC(f func([]MalType, *context.Context) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, ctx *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 0 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 0", len(args))
		}
		return f(args, ctx)
	}
}

func Call1eC(f func([]MalType, *context.Context) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, ctx *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 1)", len(args))
		}
		return f(args, ctx)
	}
}

func Call2eC(f func([]MalType, *context.Context) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, ctx *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 2 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 2)", len(args))
		}
		return f(args, ctx)
	}
}

func malRecover(err *error) {
	if rerr := recover(); rerr != nil {
		*err = rerr.(error)
	}
}
