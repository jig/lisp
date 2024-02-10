package lisp

import (
	"context"
	"testing"

	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/concurrent"
	"github.com/jig/lisp/lib/core"
	"github.com/jig/lisp/lib/coreextented"
	"github.com/jig/lisp/types"
)

func TestCursor(t *testing.T) {
	bootEnv := env.NewEnv()
	core.Load(bootEnv)
	core.LoadInput(bootEnv)
	concurrent.Load(bootEnv)

	bootEnv.Set(types.Symbol{Val: "eval"}, types.Func{Fn: func(ctx context.Context, a []types.MalType) (types.MalType, error) {
		return EVAL(ctx, a[0], bootEnv)
	}})
	bootEnv.Set(types.Symbol{Val: "*ARGV*"}, types.List{})

	ctx := context.Background()
	// core.mal: defined using the language itself
	_, err := REPL(ctx, bootEnv, `(def *host-language* "go")`, types.NewCursorFile(t.Name()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := REPL(context.Background(), bootEnv, coreextented.HeaderCoreExtended(), types.NewCursorFile("coreextended")); err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		Module string
		Code   string
		Cursor *types.Position
	}{
		{
			Module: "throw sample",
			Code:   `(try (throw 1234) (catch err err))`,
			Cursor: nil,
		},
		{
			Module: "nested",
			Code: `(do
(def fpum (fn [x] (throw x)))
(def f1 (fn [x] x))
(def f2 (fn [x] x))
(def f3 (fn [x] x))
(def f4 (fn [x] x))
(def f5 (fn [x] x))
(f1 (f2 (f3 (f4 (f5 (fpum "pum")))))))`,
			Cursor: types.NewAnonymousCursorHere(8, 30),
		},
		{
			Module: "simple-A",
			Code:   `1234`,
			Cursor: nil,
		},
		{
			Module: "simple-B",
			Code:   `(nth [1234] 0)`,
			Cursor: nil,
		},
		{
			Module: "simple-C",
			Code:   `(nth [112e] 0)`,
			Cursor: types.NewAnonymousCursorHere(1, 1),
		},
		{
			Module: "arrowMacroOK",
			Code:   arrowMacroOK,
			Cursor: nil,
		},
		{
			Module: "arrowMacroErrorLine1",
			Code: `(pet
	(-> {:base nil}
		(assoc :a 1234)
		(assoc :b "hello"))
	:a)
`,
			Cursor: types.NewAnonymousCursorHere(1, 2),
		},
		{
			Module: "arrowMacroErrorLine2",
			Code: `(get
	(-> {:base nel}
		(assoc :a 1234)
		(assoc :b "hello"))
	:a)
`,
			Cursor: types.NewAnonymousCursorHere(3, 12),
		},

		{
			Module: "singleline-string",
			Code:   `(throw "pum")`,
			Cursor: types.NewAnonymousCursorHere(1, 6),
		},
		{
			Module: "multiline-string",
			Code:   multiline,
			Cursor: types.NewAnonymousCursorHere(6, 2),
		},
		{
			Module: "codeThrow",
			Code:   codeThrow,
			Cursor: types.NewAnonymousCursorHere(4, 2),
		},
		{
			Module: "codeTryAndThrowAndCatch",
			Code:   codeTryAndThrowAndCatch,
			Cursor: nil,
		},
		{
			Module: "codeUndefinedSymbol",
			Code:   codeUndefinedSymbol,
			Cursor: types.NewAnonymousCursorHere(3, 17),
		},
		{
			Module: "codeLetIsBogus",
			Code:   codeLetIsBogus,
			Cursor: types.NewAnonymousCursorHere(4, 7),
		},
		{
			Module: "codeCorrect",
			Code:   codeCorrect,
			Cursor: nil,
		},
		{
			Module: "codeMissingRightBracket",
			Code:   codeMissingRightBracket,
			Cursor: types.NewAnonymousCursorHere(9, 1),
		},
		{
			Module: "codeTooManyRightBrackets",
			Code:   codeTooManyRightBrackets,
			Cursor: types.NewAnonymousCursorHere(9, 28),
		},
	} {
		subEnv := env.NewSubordinateEnv(bootEnv)
		ast, err := REPL(ctx, subEnv, testCase.Code, types.NewCursorFile(testCase.Module))
		switch err := err.(type) {
		case nil:
			if testCase.Cursor != nil {
				t.Fatalf("Expected error %q", testCase.Cursor)
			}
			if ast == "" {
				t.Fatal(testCase.Module, "(no error) AST is nil")
				continue
			}
			if ast != "1234" {
				t.Fatal(testCase.Module, "(no error) REPL didn't reach the end")
				continue
			}
			t.Logf("TEST OK: %s", ast)
		case interface {
			Position() *types.Position
			Error() string
		}:
			// fmt.Printf("\nTEST ERR %s: %s", testCase.Module, err)
			// fmt.Printf("\nTEST ERR %s: %s", testCase.Module, err)
			if err.Position() == nil {
				t.Error("error")
			}
			if testCase.Cursor == nil {
				t.Fatalf("expected no error but got %s", err)
			}
			if !err.Position().Includes(*testCase.Cursor) {
				t.Fatal(err)
			}
		default:
			t.Error(err)
		}
	}
}

var multiline = `(do;; multiline strings

(def multi ¬line1
	line6¬)

(throw "pum"))`

var codeCorrect = `(do
	;; prerequisites
	;; Trivial but convenient functions.

	;; Integer predecessor (number -> number)
	(def inc (fn [a] (+ a 1)))

	;; Integer predecessor (number -> number)
	(def dec (fn (a) (- a 1)))

	;; Integer nullity test (number -> boolean)
	(def zero? (fn (n) (= 0 n)))

	;; Returns the unchanged argument.
	(def identity (fn (x) x))

	;; Generate a hopefully unique symbol. See section "Plugging the Leaks"
	;; of http://www.gigamonkeys.com/book/macros-defining-your-own.html
	(def gensym
	(let [counter (atom 0)]
		(fn []
		(symbol (str "G__" (swap! counter inc))))))

	(def a 1234))
`

var codeMissingRightBracket = `(do
	;; prerequisites
	;; Trivial but convenient functions.

	;; Integer predecessor (number -> number)
	(def inc (fn [a] (+ a 1)))

	;; Integer predecessor (number -> number) ;; MISSING ) ON NEXT LINE:
	(def dec (fn (a) (- a 1))

	;; Integer nullity test (number -> boolean)
	(def zero? (fn (n) (= 0 n)))

	;; Returns the unchanged argument.
	(def identity (fn (x) x))

	;; Generate a hopefully unique symbol. See section "Plugging the Leaks"
	;; of http://www.gigamonkeys.com/book/macros-defining-your-own.html
	(def gensym
	(let [counter (atom 0)]
		(fn []
		(symbol (str "G__" (swap! counter inc))))))

	(def a 1234))
`

var codeTooManyRightBrackets = `(do
;; prerequisites
;; Trivial but convenient functions.

;; Integer predecessor (number -> number)
(def inc (fn [a] (+ a 1)))

;; Integer predecessor (number -> number)
(def dec (fn (a) (- a 1))))

;; Integer nullity test (number -> boolean)
(def zero? (fn (n) (= 0 n)))

;; Returns the unchanged argument.
(def identity (fn (x) x))

;; Generate a hopefully unique symbol. See section "Plugging the Leaks"
;; of http://www.gigamonkeys.com/book/macros-defining-your-own.html
(def gensym
  (let [counter (atom 0)]
    (fn []
      (symbol (str "G__" (swap! counter inc))))))

(def a 1234))
`

var codeThrow = `(do
;; this will throw an error
;; in a trivial way

(throw "boo"))
`

var codeTryAndThrowAndCatch = `(do
;; throwing an error and catching
;; must not involve program lines

(try
	abc
	(catch exc
		(str "exc is:" exc)))

(def a 1234))
`

// var codeTryAndThrow = `;; throwing an error and catching
// ;; must not involve program lines

// (try
// 	abc
// 	(catch exc
// 		(str "exc is:" exc)))

// (def a 1234)
// `

var codeLetIsBogus = `(do
;; let requires a vector with even elements

(let [x 1
	y]
	y))
`

var codeUndefinedSymbol = `(do
;; undefined-symbol is undefined

undefined-symbol
)`

var arrowMacroOK = `(get
	(-> {:base nil}
		(assoc :a 1234)
		(assoc :b "hello"))
	:a)
`
