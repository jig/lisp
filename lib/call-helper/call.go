package call

import (
	"context"
	"fmt"

	. "github.com/jig/lisp/types"
)

// Call0e returns a function that checks there are 0 arguments and calls f
func Call0e(f func() (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 0 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 0)", len(args))
		}
		return f()
	}
}

// Call1e returns a function that checks there is 1 argument and calls f
func Call1e(f func(MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 1)", len(args))
		}
		return f(args[0])
	}
}

// Call2e returns a function that checks there are 2 arguments and calls f
func Call2e(f func(MalType, MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 2 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 2)", len(args))
		}
		return f(args[0], args[1])
	}
}

// Call3e returns a function that checks there are 3 arguments and calls f
func Call3e(f func(MalType, MalType, MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 3 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 3)", len(args))
		}
		return f(args[0], args[1], args[2])
	}
}

// CallNe returns a function that checks there are N arguments and calls f... so it does not check anything
func CallNe(f func(...MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	// just for documenting purposes, does not check anything
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		return f(args...)
	}
}

// CallVe returns a function that checks there are a variable number of arguments between minArg and maxArg (both of them included)
func CallVe(minArg, maxArg int, f func(...MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if l := len(args); l > maxArg || l < minArg {
			return nil, fmt.Errorf("wrong number of arguments (%d is out of range [%d,%d])", len(args), minArg, maxArg)
		}
		return f(args...)
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
func CallNeC(f func(context.Context, ...MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	// just for documenting purposes, does not check anything
	return func(args []MalType, ctx *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		return f(*ctx, args...)
	}
}

// Call0eC returns a function that checks there are 0 arguments and calls f (that requires *context.Context)
func Call0eC(f func(context.Context) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, ctx *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 0 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 0", len(args))
		}
		return f(*ctx)
	}
}

// Call1eC returns a function that checks there is 1 argument and calls f (that requires *context.Context)
func Call1eC(f func(context.Context, MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, ctx *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 1)", len(args))
		}
		return f(*ctx, args[0])
	}
}

// Call2eC returns a function that checks there are 2 arguments and calls f (that requires *context.Context)
func Call2eC(f func(context.Context, MalType, MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, ctx *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 2 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 2)", len(args))
		}
		return f(*ctx, args[0], args[1])
	}
}

// Call3eC returns a function that checks there are 2 arguments and calls f (that requires *context.Context)
func Call3eC(f func(context.Context, MalType, MalType, MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, ctx *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 3 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 3)", len(args))
		}
		return f(*ctx, args[0], args[1], args[2])
	}
}

// Call4eC returns a function that checks there are 2 arguments and calls f (that requires *context.Context)
func Call4eC(f func(context.Context, MalType, MalType, MalType, MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, ctx *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 4 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 4)", len(args))
		}
		return f(*ctx, args[0], args[1], args[2], args[3])
	}
}

// CallVeC returns a function that checks there are a variable number of arguments between minArg and maxArg (both of them included)
func CallVeC(minArg, maxArg int, f func(context.Context, ...MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, ctx *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if l := len(args); l > maxArg || l < minArg {
			return nil, fmt.Errorf("wrong number of arguments (%d is out of range [%d,%d])", len(args), minArg, maxArg)
		}
		return f(*ctx, args...)
	}
}

func malRecover(err *error) {
	if rerr := recover(); rerr != nil {
		*err = rerr.(error)
	}
}
