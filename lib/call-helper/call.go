package call

import (
	"context"
	"fmt"

	. "github.com/jig/lisp/types"
)

// Call0e returns a function that checks there are 0 arguments and calls f
func Call0e(f func([]MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 0 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 0)", len(args))
		}
		return f(args)
	}
}

// Call1e returns a function that checks there is 1 argument and calls f
func Call1e(f func([]MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 1)", len(args))
		}
		return f(args)
	}
}

// Call2e returns a function that checks there are 2 arguments and calls f
func Call2e(f func([]MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 2 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 2)", len(args))
		}
		return f(args)
	}
}

// Call3e returns a function that checks there are 3 arguments and calls f
func Call3e(f func([]MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 3 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 3)", len(args))
		}
		return f(args)
	}
}

// CallNe returns a function that checks there are N arguments and calls f... so it does not check anything
func CallNe(f func([]MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	// just for documenting purposes, does not check anything
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		return f(args)
	}
}

// Call1b returns a function that checks there is 1 argument and calls f func() bool
func Call1b(f func(MalType) bool) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 1)", len(args))
		}
		return f(args[0]), nil
	}
}

// Call2b returns a function that checks there are 2 arguments and calls f func() bool
func Call2b(f func(MalType, MalType) bool) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 2 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 2)", len(args))
		}
		return f(args[0], args[1]), nil
	}
}

// CallNeC returns a function that checks there are N arguments and calls f (that requires *context.Context)
func CallNeC(f func([]MalType, *context.Context) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	// just for documenting purposes, does not check anything
	return func(args []MalType, ctx *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		return f(args, ctx)
	}
}

// Call0eC returns a function that checks there are 0 arguments and calls f (that requires *context.Context)
func Call0eC(f func([]MalType, *context.Context) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, ctx *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 0 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 0", len(args))
		}
		return f(args, ctx)
	}
}

// Call1eC returns a function that checks there is 1 argument and calls f (that requires *context.Context)
func Call1eC(f func([]MalType, *context.Context) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, ctx *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 1)", len(args))
		}
		return f(args, ctx)
	}
}

// Call2eC returns a function that checks there are 2 arguments and calls f (that requires *context.Context)
func Call2eC(f func([]MalType, *context.Context) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, ctx *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 2 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 2)", len(args))
		}
		return f(args, ctx)
	}
}

// Call2eC returns a function that checks there are 2 arguments and calls f (that requires *context.Context)
func Call3eC(f func([]MalType, *context.Context) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, ctx *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 3 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 3)", len(args))
		}
		return f(args, ctx)
	}
}

func malRecover(err *error) {
	if rerr := recover(); rerr != nil {
		*err = rerr.(error)
	}
}
