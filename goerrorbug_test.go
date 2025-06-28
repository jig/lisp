package lisp

import (
	"context"
	_ "embed"
	"testing"

	"github.com/jig/lisp/env"
)

//go:embed goerrorbug_test.lisp
var test_code string

func TestSandbox(t *testing.T) {
	repl_env := env.NewEnv()
	LoadCore(repl_env)
	ctx := context.Background()
	exp, err := READ(test_code, nil, repl_env)
	if err != nil {
		t.Fatal(err)
	}
	exp, err = EVAL(ctx, exp, repl_env, nil)
	if err != nil {
		t.Fatal("EVAL", err)
	}

	if res := PRINT(exp); res != `«go-error "simple error"»` {
		t.Fatal("PRINT", res)
	}
}
