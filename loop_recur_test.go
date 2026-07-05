package lisp

import (
	"context"
	"testing"

	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core"
	"github.com/jig/lisp/types"
)

func loopEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	core.Load(ns)
	core.LoadInput(ns)
	if _, err := REPL(context.Background(), ns, core.HeaderBasic(), nil); err != nil {
		t.Fatalf("HeaderBasic: %v", err)
	}
	return ns
}

// run evaluates src and returns its printed form (REPL returns the printed
// representation).
func run(t *testing.T, ns types.EnvType, src string) string {
	t.Helper()
	res, err := REPL(context.Background(), ns, src, types.NewCursorFile("test"))
	if err != nil {
		t.Fatalf("eval %q: %v", src, err)
	}
	return res.(string)
}

func TestLoopRecur(t *testing.T) {
	ns := loopEnv(t)
	cases := []struct{ src, want string }{
		{"(loop [i 0 acc []] (if (< i 5) (recur (+ i 1) (conj acc i)) acc))", "[0 1 2 3 4]"},
		{"(loop [n 5 acc 1] (if (= n 0) acc (recur (- n 1) (* acc n))))", "120"}, // factorial
		{"(loop [x 41] (+ x 1))", "42"},                                                  // no recur: returns the body
		{"(loop [i 0] (let [j (+ i 1)] (if (< j 5) (recur j) j)))", "5"},                 // recur in tail of let
		{"(loop [i 0] (do 1 (if (< i 3) (recur (+ i 1)) i)))", "3"},                      // recur in tail of do
		{"(do (defn cnt [n] (loop [i 0] (if (< i n) (recur (+ i 1)) i))) (cnt 7))", "7"}, // loop inside fn
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			if got := run(t, ns, tc.src); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.src, got, tc.want)
			}
		})
	}
}

// TestLoopRecurConstantStack forces many iterations; without constant-stack
// recur this would overflow.
func TestLoopRecurConstantStack(t *testing.T) {
	ns := loopEnv(t)
	if got := run(t, ns, "(loop [i 0] (if (< i 1000000) (recur (+ i 1)) :done))"); got != ":done" {
		t.Errorf("got %s, want :done", got)
	}
}

func TestLoopRecurErrors(t *testing.T) {
	ns := loopEnv(t)
	cases := map[string]string{
		"arity mismatch": "(loop [i 0 j 0] (recur 1))",
		"non-tail recur": "(loop [i 0] (+ 1 (recur (+ i 1))))",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := REPL(context.Background(), ns, src, types.NewCursorFile("test")); err == nil {
				t.Errorf("expected an error for %q, got none", src)
			}
		})
	}
}
