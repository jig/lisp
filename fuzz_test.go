package lisp_test

import (
	"context"
	"testing"
	"time"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core/nscore"
)

// evalFuzzSeeds mirrors the reader seeds but as whole programs worth
// evaluating: happy-path forms plus the phase-2 malformed inputs, which
// must now return an error rather than panic.
var evalFuzzSeeds = []string{
	"(+ 1 2)",
	"(let [a 1 b 2] (+ a b))",
	"(loop [i 0] (if (< i 3) (recur (+ i 1)) i))",
	"(try 1 (catch e e))",
	"(try (throw \"boom\") (catch e e))",
	"(map (fn [x] (* x x)) (list 1 2 3))",
	"(reduce + 0 [1 2 3 4])",
	"`(1 ~(+ 1 1) ~@(list 3 4))",
	"(def f (fn [& xs] (count xs))) (f 1 2 3)",
	// phase-2 shapes: error, not panic
	"(try ())",
	"(try 1 (catch))",
	"(quasiquote (unquote))",
	"(def g (fn [&] 1)) (g)",
	"(recur 1 2)",
	"((fn [] (recur 1)))",
}

// FuzzEval asserts EVAL never panics on any well-read input. Each input
// is read, then evaluated under a short timeout in a fresh child of a
// core-loaded env, so top-level defs don't leak between iterations and
// infinite loops are cut off by the context.
//
// Note: like any interpreter for a Turing-complete language, deep
// non-tail Lisp recursion can still exhaust the Go stack (an
// unrecoverable fatal error, not a catchable panic). Byte-level fuzzing
// from these seeds is very unlikely to synthesise such programs; a
// recursion-depth limit is tracked separately, not here.
func FuzzEval(f *testing.F) {
	base := env.NewEnv()
	if err := nscore.Load(base); err != nil {
		f.Fatalf("load core: %v", err)
	}
	for _, s := range evalFuzzSeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		ast, err := lisp.READ(src, nil, base)
		if err != nil {
			return // a read error is a valid outcome
		}
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("EVAL panicked on %q: %v", src, r)
			}
		}()
		_, _ = lisp.EVAL(ctx, ast, env.NewSubordinateEnv(base))
	})
}
