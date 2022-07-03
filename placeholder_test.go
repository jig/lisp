package lisp

import (
	"context"
	"testing"

	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core"
	"github.com/jig/lisp/reader"
	. "github.com/jig/lisp/types"
)

func TestPlaceholders(t *testing.T) {
	repl_env, _ := env.NewEnv(nil, nil, nil)
	for k, v := range core.NS {
		repl_env.Set(
			Symbol{Val: k},
			Func{Fn: v.(func([]MalType, *context.Context) (MalType, error))},
		)
	}

	str := `(do (def! v2 $2)(def! v0 $0) true)`

	exp, err := reader.Read_str(str, nil, []string{"hello", "world", "44"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := EVAL(exp, repl_env, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.(bool) {
		v0, err := repl_env.Get(Symbol{Val: "v0"})
		if err != nil {
			t.Fatal(err)
		}
		if v0.(string) != "hello" {
			t.Fatal("no hello")
		}
		// v2, err := repl_env.Get(Symbol{Val: "v2"})
		// if err != nil {
		// 	t.Fatal(err)
		// }
		// if v2.(int) != 44 {
		// 	t.Fatal("no hello")
		// }
	}
}
