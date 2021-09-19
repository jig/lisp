package mal

import (
	"context"
	"testing"

	"github.com/jig/mal/env"
	"github.com/jig/mal/lib/core"
	"github.com/jig/mal/types"
)

func TestCursor(t *testing.T) {
	bootEnv, err := env.NewEnv(nil, nil, nil)
	if err != nil {
		panic(err)
	}
	// core.go: defined using go
	for k, v := range core.NS {
		bootEnv.Set(types.Symbol{k}, types.Func{Fn: v.(func([]types.MalType, *context.Context) (types.MalType, error))})
	}
	for k, v := range core.NSInput {
		bootEnv.Set(types.Symbol{k}, types.Func{Fn: v.(func([]types.MalType, *context.Context) (types.MalType, error))})
	}
	bootEnv.Set(types.Symbol{"eval"}, types.Func{Fn: func(a []types.MalType, ctx *context.Context) (types.MalType, error) {
		return EVAL(a[0], bootEnv, ctx)
	}})
	bootEnv.Set(types.Symbol{"*ARGV*"}, types.List{})

	// core.mal: defined using the language itself
	REPL(bootEnv, `(def! *host-language* "go")`, nil)

	for _, code := range []string{
		codeExecutionError,
		codeCorrect,
		codeMissingRightBracket,
		codeTooManyRightBrackets,
	} {
		subEnv, err := env.NewEnv(bootEnv, nil, nil)
		if err != nil {
			panic(err)
		}
		ast, err := REPLPosition(subEnv, "(do\n"+code+"\na)", nil, &types.Position{Row: 0})
		if err != nil {
			t.Error(err)
			continue
		}
		if ast.(string) != "1234" {
			t.Error("REPL didn't reach the end")
			continue
		}
	}
}

var codeCorrect = `;; prerequisites
;; Trivial but convenient functions.   
      
;; Integer predecessor (number -> number)   
(def! inc (fn* [a] (+ a 1)))
    
;; Integer predecessor (number -> number)
(def! dec (fn* (a) (- a 1)))
    
;; Integer nullity test (number -> boolean)
(def! zero? (fn* (n) (= 0 n)))
 
;; Returns the unchanged argument.
(def! identity (fn* (x) x))

;; Generate a hopefully unique symbol. See section "Plugging the Leaks"
;; of http://www.gigamonkeys.com/book/macros-defining-your-own.html
(def! gensym
  (let* [counter (atom 0)]
    (fn* []
      (symbol (str "G__" (swap! counter inc))))))

(def! a 1234)
`

var codeMissingRightBracket = `;; prerequisites
;; Trivial but convenient functions.   
      
;; Integer predecessor (number -> number)   
(def! inc (fn* [a] (+ a 1)))
    
;; Integer predecessor (number -> number) ;; MISSING ) ON NEXT LINE:
(def! dec (fn* (a) (- a 1))
    
;; Integer nullity test (number -> boolean)
(def! zero? (fn* (n) (= 0 n)))
 
;; Returns the unchanged argument.
(def! identity (fn* (x) x))

;; Generate a hopefully unique symbol. See section "Plugging the Leaks"
;; of http://www.gigamonkeys.com/book/macros-defining-your-own.html
(def! gensym
  (let* [counter (atom 0)]
    (fn* []
      (symbol (str "G__" (swap! counter inc))))))

(def! a 1234)
`

var codeTooManyRightBrackets = `;; prerequisites
;; Trivial but convenient functions.   
      
;; Integer predecessor (number -> number)   
(def! inc (fn* [a] (+ a 1)))
    
;; Integer predecessor (number -> number)
(def! dec (fn* (a) (- a 1))))
    
;; Integer nullity test (number -> boolean)
(def! zero? (fn* (n) (= 0 n)))
 
;; Returns the unchanged argument.
(def! identity (fn* (x) x))

;; Generate a hopefully unique symbol. See section "Plugging the Leaks"
;; of http://www.gigamonkeys.com/book/macros-defining-your-own.html
(def! gensym
  (let* [counter (atom 0)]
    (fn* []
      (symbol (str "G__" (swap! counter inc))))))

(def! a 1234)
`
var codeExecutionError = `;; this will throw an error
;; in a trivial way

(throw "boo")
`
