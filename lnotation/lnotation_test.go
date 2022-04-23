package lisp

import (
	"context"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/jig/lisp"
	. "github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core"
	. "github.com/jig/lisp/types"
)

func TestLNotation(t *testing.T) {
	env, err := NewEnv(nil, nil, nil)
	if err != nil {
		panic(err)
	}
	// core.go: defined using go
	for k, v := range core.NS {
		env.Set(Symbol{Val: k}, Func{Fn: v.(func([]MalType, *context.Context) (MalType, error))})
	}
	for k, v := range core.NSInput {
		env.Set(Symbol{Val: k}, Func{Fn: v.(func([]MalType, *context.Context) (MalType, error))})
	}
	env.Set(Symbol{Val: "eval"}, Func{Fn: func(a []MalType, ctx *context.Context) (MalType, error) {
		return lisp.EVAL(a[0], env, ctx)
	}})
	env.Set(Symbol{Val: "*ARGV*"}, List{})

	l := L("range", 0, 4)
	lr, err := lisp.EVAL(l, env, nil)
	if err != nil {
		t.Fatal(err)
	}
	spew.Dump(lr)
}
