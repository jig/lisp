package lisp

import (
	"context"
	_ "embed"
	"testing"

	"github.com/jig/lisp/env"
)

//go:embed trycatchfinally_test.lisp
var trycatchfinally_test string

func TestTryCatchFinally(t *testing.T) {
	repl_env := env.NewEnv()
	LoadCore(repl_env)
	ctx := context.Background()
	exp, err := READ(trycatchfinally_test, nil, repl_env)
	if err != nil {
		t.Fatal(err)
	}
	exp, err = EVAL(ctx, exp, repl_env)
	if err != nil {
		t.Fatal(err)
	}

	if res := PRINT(exp); res != "true" {
		t.Fatal(res)
	}
}
