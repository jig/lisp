package coreextented_test

import (
	"context"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/concurrent/nsconcurrent"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/lib/coreextented/nscoreextended"
	"github.com/jig/lisp/types"
)

func newEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	for _, load := range []func(types.EnvType) error{
		nscore.Load,         // core builtins + basic header (defn, cond, …)
		nscore.LoadInput,    // load-file (used by the header's load-file-once)
		nsconcurrent.Load,   // atom/swap! (used by the header's gensym/memoize)
		nscoreextended.Load, // the header under test
	} {
		if err := load(ns); err != nil {
			t.Fatalf("load: %v", err)
		}
	}
	return ns
}

// run evaluates src and returns its printed form (REPL returns the printed
// representation, which is convenient for asserting on results).
func run(t *testing.T, ns types.EnvType, src string) string {
	t.Helper()
	res, err := lisp.REPL(context.Background(), ns, src, types.NewCursorFile("test"))
	if err != nil {
		t.Fatalf("eval %q: %v", src, err)
	}
	s, ok := res.(string)
	if !ok {
		t.Fatalf("REPL returned %T, want printed string", res)
	}
	return s
}

func TestPrelude(t *testing.T) {
	ns := newEnv(t)
	cases := []struct{ src, want string }{
		// quot / rem / mod, including the negative cases where they diverge
		{"(quot 17 5)", "3"},
		{"(quot -17 5)", "-3"},
		{"(rem 17 5)", "2"},
		{"(rem -17 5)", "-2"},
		{"(mod 17 5)", "2"},
		{"(mod -17 5)", "3"},
		{"(mod 17 -5)", "-3"},
		{"(mod 6 3)", "0"},
		// abs / min / max
		{"(abs -4)", "4"},
		{"(abs 4)", "4"},
		{"(min 3 1 2)", "1"},
		{"(max 3 1 2)", "3"},
		{"(min 5)", "5"},
		// predicates
		{"(pos? 1)", "true"},
		{"(pos? 0)", "false"},
		{"(neg? -1)", "true"},
		{"(even? 4)", "true"},
		{"(even? 0)", "true"},
		{"(odd? 3)", "true"},
		{"(odd? -3)", "true"},
		{"(even? -4)", "true"},
		// sequence filters
		{"(filter even? [1 2 3 4])", "(2 4)"},
		{"(filter even? (list 1 2 3 4))", "(2 4)"},
		{"(remove even? [1 2 3 4])", "(1 3)"},
		{"(take-while (fn [x] (< x 3)) [1 2 3 1])", "(1 2)"},
		{"(take-while (fn [x] (< x 3)) [])", "()"},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			if got := run(t, ns, tc.src); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.src, got, tc.want)
			}
		})
	}
}
