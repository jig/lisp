package lisp

import (
	"context"
	"testing"

	. "github.com/jig/lisp/env"
	. "github.com/jig/lisp/types"
)

// TestRecurOutsideLoopErrors checks that a `recur` that no enclosing
// `loop` consumes surfaces an error instead of leaking the internal
// «recur» sentinel to the caller (ROADMAP-robustness.md 2.5).
func TestRecurOutsideLoopErrors(t *testing.T) {
	for _, src := range []string{
		"(recur 1 2)",         // top level
		"((fn [] (recur 1)))", // bare function body, no loop
	} {
		e := NewEnv()
		ast, err := READ(src, nil, e)
		if err != nil {
			t.Fatalf("READ(%q): %v", src, err)
		}
		res, err := EVAL(context.Background(), ast, e)
		if err == nil {
			t.Fatalf("EVAL(%q): expected an error, got result %v", src, res)
		}
	}
}

// TestRecurInsideLoopStillWorks pins that the recur boundary in EVAL
// does not break legitimate loop/recur.
func TestRecurInsideLoopStillWorks(t *testing.T) {
	e := NewEnv()
	src := "(loop [i 0] (if (< i 3) (recur (+ i 1)) i))"
	// (< and (+ are builtins; register just what the expression needs.
	e.Set(Symbol{Val: "<"}, Func{Fn: func(_ context.Context, a []MalType) (MalType, error) {
		return a[0].(int) < a[1].(int), nil
	}})
	e.Set(Symbol{Val: "+"}, Func{Fn: func(_ context.Context, a []MalType) (MalType, error) {
		return a[0].(int) + a[1].(int), nil
	}})
	ast, err := READ(src, nil, e)
	if err != nil {
		t.Fatalf("READ: %v", err)
	}
	res, err := EVAL(context.Background(), ast, e)
	if err != nil {
		t.Fatalf("EVAL: %v", err)
	}
	if res != 3 {
		t.Fatalf("loop/recur result = %v, want 3", res)
	}
}

// TestNilContextTryNoPanic guards the nil-context contract: EVAL accepts
// a nil context, and `try` must not panic looking up its deadline
// (ROADMAP-robustness.md 2.6).
func TestNilContextTryNoPanic(t *testing.T) {
	e := NewEnv()
	ast, err := READ("(try 1 (catch e e))", nil, e)
	if err != nil {
		t.Fatalf("READ: %v", err)
	}
	res, err := evalNilCtxNoPanic(t, ast, e)
	if err != nil {
		t.Fatalf("EVAL with nil ctx: %v", err)
	}
	if res != 1 {
		t.Fatalf("result = %v, want 1", res)
	}
}

func evalNilCtxNoPanic(t *testing.T, ast MalType, e EnvType) (res MalType, err error) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("EVAL panicked on nil context: %v", r)
		}
	}()
	// SA1012: passing a nil context on purpose — it is a supported input.
	return EVAL(nil, ast, e) //nolint:staticcheck
}

// TestMalRecoverNonError checks malRecover turns a non-error panic value
// into an error rather than re-panicking on the type assertion
// (ROADMAP-robustness.md 2.7).
func TestMalRecoverNonError(t *testing.T) {
	var err error
	func() {
		defer malRecover(&err)
		panic("not a Go error")
	}()
	if err == nil {
		t.Fatal("malRecover should have set an error for a non-error panic value")
	}
}
