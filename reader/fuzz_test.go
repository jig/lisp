package reader_test

import (
	"testing"

	"github.com/jig/lisp/reader"
)

// fuzzSeeds are shared shapes for the reader and eval fuzzers: valid
// forms exercising every syntactic construct, plus the malformed inputs
// that used to panic (ROADMAP-robustness.md phase 2). The corpus grows
// from here as the fuzzer discovers new inputs under testdata/fuzz/.
var fuzzSeeds = []string{
	// valid
	"(+ 1 2)",
	"(def x 1)",
	"(let [a 1 b 2] (+ a b))",
	"[1 2 3]",
	"{:a 1 :b 2}",
	"#{1 2 3}",
	"(fn [x & xs] xs)",
	"(try 1 (catch e e) (finally 2))",
	"(quasiquote (unquote 5))",
	"`(1 ~x ~@ys)",
	"(loop [i 0] (if (< i 3) (recur (+ i 1)) i))",
	";; a comment\n(+ 1 1)",
	"\"a string\\n\"",
	"'quoted",
	"@deref",
	"1.5",
	"-42",
	":keyword",
	"nil true false",
	// malformed shapes fixed in phase 2 — must read or error, never panic
	"(try ())",
	"(try 1 (catch))",
	"(try 1 (catch) (finally 2))",
	"(quasiquote (unquote))",
	"(quasiquote ((splice-unquote)))",
	"(fn [&])",
	"(fn [& 3])",
	"(fn [3])",
	"(recur 1 2)",
	")",
	"(((((",
	"{:a}",
	"[",
	"\"unterminated",
	"",
}

// FuzzReadStr asserts the reader never panics: any input must yield an
// AST or an error. Read_str is called without an environment, so no Go
// constructors are involved — pure syntax.
func FuzzReadStr(f *testing.F) {
	for _, s := range fuzzSeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Read_str panicked on %q: %v", src, r)
			}
		}()
		_, _ = reader.Read_str(src, nil, nil)
	})
}
