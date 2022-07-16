package lisp

import (
	"context"
	"sync"
	"testing"

	. "github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core"
	. "github.com/jig/lisp/types"
)

func BenchmarkLoadSymbols(b *testing.B) {
	repl_env, _ := NewEnv(nil, nil, nil)
	for i := 0; i < b.N; i++ {
		for k, v := range core.NS {
			repl_env.Set(Symbol{Val: k}, Func{Fn: v.(func(context.Context, []MalType) (MalType, error))})
		}
	}
}

func BenchmarkMAL1(b *testing.B) {
	repl_env, _ := NewEnv(nil, nil, nil)
	for k, v := range core.NS {
		repl_env.Set(Symbol{Val: k}, Func{Fn: v.(func(context.Context, []MalType) (MalType, error))})
	}
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		repl_env.Set(Symbol{Val: "eval"}, Func{Fn: func(ctx context.Context, a []MalType) (MalType, error) {
			return EVAL(ctx, a[0], repl_env)
		}})
		repl_env.Set(Symbol{Val: "*ARGV*"}, List{})

		// core.mal: defined using the language itself
		if _, err := REPL(ctx, repl_env, `(def *host-language* "go")`); err != nil {
			b.Fatal(err)
		}
		if _, err := REPL(ctx, repl_env, `(def not (fn (a) (if a false true)))`); err != nil {
			b.Fatal(err)
		}
		if _, err := REPL(ctx, repl_env, `(def load-file (fn (f) (eval (read-string (str "(do " (slurp f) "\nnil)")))))`); err != nil {
			b.Fatal(err)
		}
		if _, err := REPL(ctx, repl_env, `(defmacro cond (fn (& xs) (if (> (count xs) 0) (list 'if (first xs) (if (> (count xs) 1) (nth xs 1) (throw "odd number of forms to cond")) (cons 'cond (rest (rest xs)))))))`); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMAL2(b *testing.B) {
	repl_env, _ := NewEnv(nil, nil, nil)
	for k, v := range core.NS {
		repl_env.Set(Symbol{Val: k}, Func{Fn: v.(func(context.Context, []MalType) (MalType, error))})
	}
	repl_env.Set(Symbol{Val: "eval"}, Func{Fn: func(ctx context.Context, a []MalType) (MalType, error) {
		return EVAL(ctx, a[0], repl_env)
	}})
	repl_env.Set(Symbol{Val: "*ARGV*"}, List{})
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		if _, err := REPL(ctx, repl_env, `(def not (fn (a) (if a false true)))`); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParallelREAD(b *testing.B) {
	repl_env, _ := NewEnv(nil, nil, nil)
	for k, v := range core.NS {
		repl_env.Set(Symbol{Val: k}, Func{Fn: v.(func(context.Context, []MalType) (MalType, error))})
	}
	repl_env.Set(Symbol{Val: "eval"}, Func{Fn: func(ctx context.Context, a []MalType) (MalType, error) {
		return EVAL(ctx, a[0], repl_env)
	}})
	repl_env.Set(Symbol{Val: "*ARGV*"}, List{})
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			str := `(def not (fn (a) (if a false true)))`

			var e error
			if _, e = READ(str, nil); e != nil {
				b.Fatal(e)
			}

		}
	})
}

func BenchmarkParallelREP(b *testing.B) {
	repl_env, _ := NewEnv(nil, nil, nil)
	for k, v := range core.NS {
		repl_env.Set(Symbol{Val: k}, Func{Fn: v.(func(context.Context, []MalType) (MalType, error))})
	}
	repl_env.Set(Symbol{Val: "eval"}, Func{Fn: func(ctx context.Context, a []MalType) (MalType, error) {
		return EVAL(ctx, a[0], repl_env)
	}})
	repl_env.Set(Symbol{Val: "*ARGV*"}, List{})
	ctx := context.Background()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := REPL(ctx, repl_env, `(def not (fn (a) (if a false true)))`); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkREP(b *testing.B) {
	repl_env, _ := NewEnv(nil, nil, nil)
	for k, v := range core.NS {
		repl_env.Set(Symbol{Val: k}, Func{Fn: v.(func(context.Context, []MalType) (MalType, error))})
	}
	repl_env.Set(Symbol{Val: "eval"}, Func{Fn: func(ctx context.Context, a []MalType) (MalType, error) {
		return EVAL(ctx, a[0], repl_env)
	}})
	repl_env.Set(Symbol{Val: "*ARGV*"}, List{})
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		if _, err := REPL(ctx, repl_env, `(def not (fn (a) (if a false true)))`); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFibonacci(b *testing.B) {
	repl_env, _ := NewEnv(nil, nil, nil)
	for k, v := range core.NS {
		repl_env.Set(Symbol{Val: k}, Func{Fn: v.(func(context.Context, []MalType) (MalType, error))})
	}
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		if _, err := REPL(ctx, repl_env, `(do
				(def fib
				(fn [n]                              ; non-negative number
				(if (<= n 1)
					n
					(+ (fib (- n 1)) (fib (- n 2))))))
				(fib 10))`); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParallelFibonacci(b *testing.B) {
	repl_env, _ := NewEnv(nil, nil, nil)
	for k, v := range core.NS {
		repl_env.Set(Symbol{Val: k}, Func{Fn: v.(func(context.Context, []MalType) (MalType, error))})
	}
	ctx := context.Background()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := REPL(ctx, repl_env, `(do
					(def fib
					(fn [n]                              ; non-negative number
					(if (<= n 1)
						n
						(+ (fib (- n 1)) (fib (- n 2))))))
					(fib 0))`); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkParallelFibonacciExec(b *testing.B) {
	repl_env, _ := NewEnv(nil, nil, nil)
	for k, v := range core.NS {
		repl_env.Set(Symbol{Val: k}, Func{Fn: v.(func(context.Context, []MalType) (MalType, error))})
	}
	ctx := context.Background()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := REPL(ctx, repl_env, `(do
				(def fib
				(fn [n]                              ; non-negative number
				(if (<= n 1)
					n
					(+ (fib (- n 1)) (fib (- n 2))))))
				(fib 0))`); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func TestAtomParallel(t *testing.T) {
	repl_env, _ := NewEnv(nil, nil, nil)

	// core.go: defined using go
	for k, v := range core.NS {
		repl_env.Set(Symbol{Val: k}, Func{Fn: v.(func(context.Context, []MalType) (MalType, error))})
	}
	repl_env.Set(Symbol{Val: "eval"}, Func{Fn: func(ctx context.Context, a []MalType) (MalType, error) {
		return EVAL(ctx, a[0], repl_env)
	}})
	repl_env.Set(Symbol{Val: "*ARGV*"}, List{})
	ctx := context.Background()
	// core.mal: defined using the language itself
	if _, err := REPL(ctx, repl_env, "(def *host-language* \"go\")"); err != nil {
		t.Fatal(err)
	}
	if _, err := REPL(ctx, repl_env, "(def not (fn (a) (if a false true)))"); err != nil {
		t.Fatal(err)
	}
	if _, err := REPL(ctx, repl_env, "(def load-file (fn (f) (eval (read-string (str \"(do \" (slurp f) \"\nnil)\")))))"); err != nil {
		t.Fatal(err)
	}
	if _, err := REPL(ctx, repl_env, "(defmacro cond (fn (& xs) (if (> (count xs) 0) (list 'if (first xs) (if (> (count xs) 1) (nth xs 1) (throw \"odd number of forms to cond\")) (cons 'cond (rest (rest xs)))))))"); err != nil {
		t.Fatal(err)
	}

	if _, err := REPL(ctx, repl_env, "(def count (atom 0))"); err != nil {
		t.Fatal(err)
	}
	if _, err := REPL(ctx, repl_env, "(def inc (fn [x] (+ 1 x)))"); err != nil {
		t.Fatal(err)
	}

	wd := sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		wd.Add(1)
		go func() {
			for j := 0; j < 1000; j++ {
				if _, err := REPL(ctx, repl_env, "(swap! count inc)"); err != nil {
					return
				}
			}
			wd.Done()
		}()
	}
	wd.Wait()
	if _, err := REPL(ctx, repl_env, "(if (not (= @count 100000)) (throw @count))"); err != nil {
		t.Fatal(REPL(ctx, repl_env, `(println "@count != " @count)`))
	}
}

func BenchmarkAtomParallel(b *testing.B) {
	repl_env, _ := NewEnv(nil, nil, nil)

	// core.go: defined using go
	for k, v := range core.NS {
		repl_env.Set(Symbol{Val: k}, Func{Fn: v.(func(context.Context, []MalType) (MalType, error))})
	}
	repl_env.Set(Symbol{Val: "eval"}, Func{Fn: func(ctx context.Context, a []MalType) (MalType, error) {
		return EVAL(ctx, a[0], repl_env)
	}})
	repl_env.Set(Symbol{Val: "*ARGV*"}, List{})
	ctx := context.Background()

	// core.mal: defined using the language itself
	if _, err := REPL(ctx, repl_env, "(def *host-language* \"go\")"); err != nil {
		b.Fatal(err)
	}
	if _, err := REPL(ctx, repl_env, "(def not (fn (a) (if a false true)))"); err != nil {
		b.Fatal(err)
	}
	if _, err := REPL(ctx, repl_env, "(def load-file (fn (f) (eval (read-string (str \"(do \" (slurp f) \"\nnil)\")))))"); err != nil {
		b.Fatal(err)
	}
	if _, err := REPL(ctx, repl_env, "(defmacro cond (fn (& xs) (if (> (count xs) 0) (list 'if (first xs) (if (> (count xs) 1) (nth xs 1) (throw \"odd number of forms to cond\")) (cons 'cond (rest (rest xs)))))))"); err != nil {
		b.Fatal(err)
	}

	if _, err := REPL(ctx, repl_env, "(def count (atom 0))"); err != nil {
		b.Fatal(err)
	}
	if _, err := REPL(ctx, repl_env, "(def inc (fn [x] (+ 1 x)))"); err != nil {
		b.Fatal(err)
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := REPL(ctx, repl_env, "(swap! count inc)"); err != nil {
				b.Fatal(err)
			}
			// exp, err := READ("(swap! count inc)")
			// if err != nil {
			// 	b.Fatal(err)
			// }
			// if exp, err = EVAL(exp, repl_env); err != nil {
			// 	b.Fatal(err)
			// }
			// if _, err = PRINT(exp); err != nil {
			// 	b.Fatal(err)
			// }
		}
	})
}
