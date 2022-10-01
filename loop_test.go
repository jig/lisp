package lisp_test

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/types"
)

//go:embed loop_test.lisp
var loop_test string

func TestLoopRecur(t *testing.T) {
	ctx := context.Background()
	ns := env.NewEnv()
	for _, library := range []struct {
		name string
		load func(ns types.EnvType) error
	}{
		{"core mal", nscore.Load},
	} {
		if err := library.load(ns); err != nil {
			log.Fatalf("Library Load Error: %v\n", err)
		}
	}
	ast, err := lisp.READ(loop_test, types.NewCursorFile(t.Name()), ns)
	if err != nil {
		t.Fatal(err)
	}

	res, err := lisp.EVAL(ctx, ast, ns)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(res)
}
