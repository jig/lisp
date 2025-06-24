package lisp

import (
	"context"
	"sync"
	"testing"

	. "github.com/jig/lisp/env"
	. "github.com/jig/lisp/types"
)

func BenchmarkLoadSymbols(b *testing.B) {
	repl_env := NewEnv()
	for i := 0; i < b.N; i++ {
		LoadCore(repl_env)
	}
}

func BenchmarkMAL1(b *testing.B) {
	repl_env := NewEnv()
	LoadCore(repl_env)
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		repl_env.Set(Symbol{Val: "eval"}, Func{Fn: func(ctx context.Context, a []MalType) (MalType, error) {
			return EVAL(ctx, a[0], repl_env)
		}})
		repl_env.Set(Symbol{Val: "*ARGV*"}, List{})

		// core.mal: defined using the language itself
		if _, err := REPL(ctx, repl_env, `(def *host-language* "go")`, NewCursorFile(b.Name())); err != nil {
			b.Fatal(err)
		}
		if _, err := REPL(ctx, repl_env, `(def not (fn (a) (if a false true)))`, NewCursorFile(b.Name())); err != nil {
			b.Fatal(err)
		}
		if _, err := REPL(ctx, repl_env, `(def load-file (fn (f) (eval (read-string (str "(do " (slurp f) "\nnil)")))))`, NewCursorFile(b.Name())); err != nil {
			b.Fatal(err)
		}
		if _, err := REPL(ctx, repl_env, `(defmacro cond (fn (& xs) (if (> (count xs) 0) (list 'if (first xs) (if (> (count xs) 1) (nth xs 1) (throw "odd number of forms to cond")) (cons 'cond (rest (rest xs)))))))`, NewCursorFile(b.Name())); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMAL2(b *testing.B) {
	repl_env := NewEnv()
	LoadCore(repl_env)
	repl_env.Set(Symbol{Val: "eval"}, Func{Fn: func(ctx context.Context, a []MalType) (MalType, error) {
		return EVAL(ctx, a[0], repl_env)
	}})
	repl_env.Set(Symbol{Val: "*ARGV*"}, List{})
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		if _, err := REPL(ctx, repl_env, `(def not (fn (a) (if a false true)))`, NewCursorFile(b.Name())); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParallelREAD(b *testing.B) {
	repl_env := NewEnv()
	LoadCore(repl_env)
	repl_env.Set(Symbol{Val: "eval"}, Func{Fn: func(ctx context.Context, a []MalType) (MalType, error) {
		return EVAL(ctx, a[0], repl_env)
	}})
	repl_env.Set(Symbol{Val: "*ARGV*"}, List{})
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			str := `(def not (fn (a) (if a false true)))`

			var e error
			if _, e = READ(str, nil, repl_env); e != nil {
				b.Fatal(e)
			}

		}
	})
}

func BenchmarkParallelREP(b *testing.B) {
	repl_env := NewEnv()
	LoadCore(repl_env)
	repl_env.Set(Symbol{Val: "eval"}, Func{Fn: func(ctx context.Context, a []MalType) (MalType, error) {
		return EVAL(ctx, a[0], repl_env)
	}})
	repl_env.Set(Symbol{Val: "*ARGV*"}, List{})
	ctx := context.Background()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := REPL(ctx, repl_env, `(def not (fn (a) (if a false true)))`, NewCursorFile(b.Name())); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkREP(b *testing.B) {
	repl_env := NewEnv()
	LoadCore(repl_env)
	repl_env.Set(Symbol{Val: "eval"}, Func{Fn: func(ctx context.Context, a []MalType) (MalType, error) {
		return EVAL(ctx, a[0], repl_env)
	}})
	repl_env.Set(Symbol{Val: "*ARGV*"}, List{})
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		if _, err := REPL(ctx, repl_env, `(def not (fn (a) (if a false true)))`, NewCursorFile(b.Name())); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFibonacci(b *testing.B) {
	repl_env := NewEnv()
	LoadCore(repl_env)
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		_, err := REPL(ctx, repl_env, `(do
			(def fib
			(fn [n]                              ; non-negative number
			(if (<= n 1)
				n
				(+ (fib (- n 1)) (fib (- n 2))))))
			(fib 10))`, NewCursorFile(b.Name()))

		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParallelFibonacci(b *testing.B) {
	repl_env := NewEnv()
	LoadCore(repl_env)
	ctx := context.Background()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := REPL(ctx, repl_env, `(do
					(def fib
					(fn [n]                              ; non-negative number
					(if (<= n 1)
						n
						(+ (fib (- n 1)) (fib (- n 2))))))
					(fib 9))`, NewCursorFile(b.Name())); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func TestAtomParallel(t *testing.T) {
	repl_env := NewEnv()

	LoadCore(repl_env)
	LoadConcurrent(repl_env)
	repl_env.Set(Symbol{Val: "eval"}, Func{Fn: func(ctx context.Context, a []MalType) (MalType, error) {
		return EVAL(ctx, a[0], repl_env)
	}})
	repl_env.Set(Symbol{Val: "*ARGV*"}, List{})
	ctx := context.Background()
	// core.mal: defined using the language itself
	if _, err := REPL(ctx, repl_env, "(def *host-language* \"go\")", NewCursorFile(t.Name())); err != nil {
		t.Fatal(err)
	}
	if _, err := REPL(ctx, repl_env, "(def not (fn (a) (if a false true)))", NewCursorFile(t.Name())); err != nil {
		t.Fatal(err)
	}
	if _, err := REPL(ctx, repl_env, `(def load-file (fn (f) (eval (read-string (str "(do " (slurp f) "\nnil)")))))`, NewCursorFile(t.Name())); err != nil {
		t.Fatal(err)
	}
	if _, err := REPL(ctx, repl_env, "(defmacro cond (fn (& xs) (if (> (count xs) 0) (list 'if (first xs) (if (> (count xs) 1) (nth xs 1) (throw \"odd number of forms to cond\")) (cons 'cond (rest (rest xs)))))))", NewCursorFile(t.Name())); err != nil {
		t.Fatal(err)
	}

	if _, err := REPL(ctx, repl_env, "(def count (atom 0))", NewCursorFile(t.Name())); err != nil {
		t.Fatal(err)
	}
	if _, err := REPL(ctx, repl_env, "(def inc (fn [x] (+ 1 x)))", NewCursorFile(t.Name())); err != nil {
		t.Fatal(err)
	}

	wd := sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		wd.Add(1)
		go func() {
			for j := 0; j < 1000; j++ {
				if _, err := REPL(ctx, repl_env, "(swap! count inc)", NewCursorFile(t.Name())); err != nil {
					return
				}
			}
			wd.Done()
		}()
	}
	wd.Wait()
	if _, err := REPL(ctx, repl_env, "(if (not (= @count 100000)) (throw @count))", NewCursorFile(t.Name())); err != nil {
		t.Fatal(REPL(ctx, repl_env, `(println "@count != " @count)`, NewCursorFile(t.Name())))
	}
}

func BenchmarkAtomParallel(b *testing.B) {
	repl_env := NewEnv()

	LoadCore(repl_env)
	LoadConcurrent(repl_env)
	repl_env.Set(Symbol{Val: "eval"}, Func{Fn: func(ctx context.Context, a []MalType) (MalType, error) {
		return EVAL(ctx, a[0], repl_env)
	}})
	repl_env.Set(Symbol{Val: "*ARGV*"}, List{})
	ctx := context.Background()

	// core.mal: defined using the language itself
	if _, err := REPL(ctx, repl_env, "(def *host-language* \"go\")", NewCursorFile(b.Name())); err != nil {
		b.Fatal(err)
	}
	if _, err := REPL(ctx, repl_env, "(def not (fn (a) (if a false true)))", NewCursorFile(b.Name())); err != nil {
		b.Fatal(err)
	}
	if _, err := REPL(ctx, repl_env, `(def load-file (fn (f) (eval (read-string (str "(do " (slurp f) "\nnil)")))))`, NewCursorFile(b.Name())); err != nil {
		b.Fatal(err)
	}
	if _, err := REPL(ctx, repl_env, "(defmacro cond (fn (& xs) (if (> (count xs) 0) (list 'if (first xs) (if (> (count xs) 1) (nth xs 1) (throw \"odd number of forms to cond\")) (cons 'cond (rest (rest xs)))))))", NewCursorFile(b.Name())); err != nil {
		b.Fatal(err)
	}

	if _, err := REPL(ctx, repl_env, "(def count (atom 0))", NewCursorFile(b.Name())); err != nil {
		b.Fatal(err)
	}
	if _, err := REPL(ctx, repl_env, "(def inc (fn [x] (+ 1 x)))", NewCursorFile(b.Name())); err != nil {
		b.Fatal(err)
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := REPL(ctx, repl_env, "(swap! count inc)", NewCursorFile(b.Name())); err != nil {
				b.Fatal(err)
			}
			// exp, err := READ("(swap! count inc)")
			// if err != nil {
			// 	b.Fatal(err)
			// }
			// if exp, err = EVAL(exp, repl_env, NewCursorFile(b.Name())); err != nil {
			// 	b.Fatal(err)
			// }
			// if _, err = PRINT(exp, NewCursorFile(b.Name())); err != nil {
			// 	b.Fatal(err)
			// }
		}
	})
}
