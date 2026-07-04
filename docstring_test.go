package lisp_test

import (
	"context"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/concurrent"
	"github.com/jig/lisp/lib/core"
	"github.com/jig/lisp/lib/coreextented"
	"github.com/jig/lisp/types"
)

// docEnv builds an environment with the libraries needed for defn
// docstrings and the (doc …) helper (core + coreextended).
func docEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	core.Load(ns)
	core.LoadInput(ns)
	concurrent.Load(ns)
	ns.Set(types.Symbol{Val: "eval"}, types.Func{Fn: func(ctx context.Context, a []types.MalType) (types.MalType, error) {
		return lisp.EVAL(ctx, a[0], ns)
	}})
	ctx := context.Background()
	for _, h := range []string{core.HeaderBasic(), core.HeaderLoadFile(), concurrent.HeaderConcurrent(), coreextented.HeaderCoreExtended()} {
		if _, err := lisp.REPL(ctx, ns, h, types.NewCursorFile("preamble")); err != nil {
			t.Fatalf("header: %v", err)
		}
	}
	return ns
}

func eval(t *testing.T, ns types.EnvType, src string) string {
	t.Helper()
	out, err := lisp.REPL(context.Background(), ns, src, types.NewCursorFile("t"))
	if err != nil {
		t.Fatalf("%s: %v", src, err)
	}
	s, _ := out.(string)
	return s
}

func TestDefnDocstring(t *testing.T) {
	ns := docEnv(t)

	// Clojure-style docstring: stored as {:doc "…"} metadata; the
	// function still works normally.
	eval(t, ns, `(defn sq "Squares its argument." [x] (* x x))`)
	if got := eval(t, ns, `(sq 5)`); got != "25" {
		t.Errorf("sq 5: expected 25, got %q", got)
	}
	if got := eval(t, ns, `(doc sq)`); got != `"Squares its argument."` {
		t.Errorf("doc sq: expected the docstring, got %q", got)
	}
	if got := eval(t, ns, `(meta sq)`); got != `{:doc "Squares its argument."}` {
		t.Errorf("meta sq: expected {:doc …}, got %q", got)
	}

	// Backward compatible: no docstring → no metadata, same behaviour.
	eval(t, ns, `(defn plain [x] (+ x 1))`)
	if got := eval(t, ns, `(plain 4)`); got != "5" {
		t.Errorf("plain 4: expected 5, got %q", got)
	}
	if got := eval(t, ns, `(doc plain)`); got != "nil" {
		t.Errorf("doc plain: expected nil, got %q", got)
	}

	// A trailing string that is the body (not a docstring) is not
	// mistaken for one: (defn f [x] "literal") returns the string.
	eval(t, ns, `(defn returns-string [x] "hello")`)
	if got := eval(t, ns, `(returns-string 0)`); got != `"hello"` {
		t.Errorf("returns-string: expected \"hello\", got %q", got)
	}
	if got := eval(t, ns, `(doc returns-string)`); got != "nil" {
		t.Errorf("doc returns-string: expected nil, got %q", got)
	}

	// Docstring with variadic params.
	eval(t, ns, `(defn v "variadic" [a & more] a)`)
	if got := eval(t, ns, `(doc v)`); got != `"variadic"` {
		t.Errorf("doc v: expected the docstring, got %q", got)
	}
	if got := eval(t, ns, `(v 1 2 3)`); got != "1" {
		t.Errorf("v: expected 1, got %q", got)
	}
}

// TestBuiltinDoc verifies (doc name) works for a Go builtin documented
// via call.Doc (its doc lives in dedicated fields), while its metadata
// stays nil for kanaka/mal compatibility.
func TestBuiltinDoc(t *testing.T) {
	ns := docEnv(t)
	if got := eval(t, ns, `(doc assoc)`); got != `"Copy of map with the given key/value pairs added or replaced."` {
		t.Errorf("expected assoc docstring, got %q", got)
	}
	// Native function metadata must remain nil.
	if got := eval(t, ns, `(meta assoc)`); got != "nil" {
		t.Errorf("expected (meta assoc) to be nil, got %q", got)
	}
	if got := eval(t, ns, `(meta +)`); got != "nil" {
		t.Errorf("expected (meta +) to be nil, got %q", got)
	}
}
