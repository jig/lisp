package lisp

import (
	"context"
	_ "embed"
	"testing"

	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core"
	"github.com/jig/lisp/types"
)

//go:embed trycatchfinally_test.lisp
var trycatchfinally_test string

func TestTryCatchFinally(t *testing.T) {
	repl_env, _ := env.NewEnv(nil, nil, nil)
	for k, v := range core.NS {
		repl_env.Set(
			types.Symbol{Val: k},
			types.Func{Fn: v.(func([]types.MalType, *context.Context) (types.MalType, error))},
		)
	}

	exp, err := READ(trycatchfinally_test, nil)
	if err != nil {
		t.Fatal(err)
	}
	exp, err = EVAL(exp, repl_env, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := PRINT(exp)
	if err != nil {
		t.Fatal(err)
	}
	if res != "true" {
		t.Fatal(res)
	}
}
