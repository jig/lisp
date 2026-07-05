package lisp

import (
	"context"
	"testing"

	. "github.com/jig/lisp/env"
	. "github.com/jig/lisp/types"
)

// TestFnNoParamsNoPanic guards against a panic evaluating a malformed
// `(fn)` with no parameter list. It must degrade to a no-op function,
// like `(fn nil)`, instead of slicing the body out of range.
func TestFnNoParamsNoPanic(t *testing.T) {
	e := NewEnv()
	ast, err := READ("(fn)", nil, e)
	if err != nil {
		t.Fatalf("READ (fn): %v", err)
	}
	res, err := EVAL(context.Background(), ast, e)
	if err != nil {
		t.Fatalf("EVAL (fn): %v", err)
	}
	if _, ok := res.(MalFunc); !ok {
		t.Fatalf("(fn) should evaluate to a function, got %T", res)
	}
}
