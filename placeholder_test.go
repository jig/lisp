package lisp

import (
	"context"
	"testing"

	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core"

	. "github.com/jig/lisp/lnotation"
	. "github.com/jig/lisp/types"
)

type Example struct {
	A int
	B string
}

func TestPlaceholders(t *testing.T) {
	repl_env, _ := env.NewEnv(nil, nil, nil)
	for k, v := range core.NS {
		repl_env.Set(
			Symbol{Val: k},
			Func{Fn: v.(func([]MalType, *context.Context) (MalType, error))},
		)
	}

	str := `(do
				(def! v0 $0)
				(def! v1 $1)
				(def! vNUMBER $NUMBER)
				(def! v3 $3)
				(def! v4 $4)
				true)`

	exp, err := READ_WithPlaceholders(
		str,
		nil,
		&HashMap{
			Val: map[string]MalType{
				"$0":      "hello",
				"$1":      "{\"key\": \"value\"}",
				"$NUMBER": 44,
				"$3":      LS("+", 1, 1),
				"$4": LS("jsonencode",
					Example{A: 3, B: "blurp"}),
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	// fmt.Println(PRINT(exp))

	res, err := EVAL(exp, repl_env, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.(bool) {
		v0, err := repl_env.Get(Symbol{Val: "v0"})
		if err != nil {
			t.Fatal(err)
		}
		if v0.(string) != "hello" {
			t.Fatal("no hello")
		}

		v1, err := repl_env.Get(Symbol{Val: "v1"})
		if err != nil {
			t.Fatal(err)
		}
		if v1.(string) != "{\"key\": \"value\"}" {
			t.Fatal("no {\"key\": \"value\"}")
		}

		v2, err := repl_env.Get(Symbol{Val: "vNUMBER"})
		if err != nil {
			t.Fatal(err)
		}
		if v2.(int) != 44 {
			t.Fatal("no 44")
		}

		v3, err := repl_env.Get(Symbol{Val: "v3"})
		if err != nil {
			t.Fatal(err)
		}
		if v3.(int) != 2 {
			t.Fatal("no 2")
		}

		v4, err := repl_env.Get(Symbol{Val: "v4"})
		if err != nil {
			t.Fatal(err)
		}
		switch v4 := v4.(type) {
		case string:
			if v4 != "{\"A\":3,\"B\":\"blurp\"}" {
				t.Fatal("invalid value")
			}
		default:
			t.Fatal("invalid type")
		}
	}
}

func TestPlaceholdersEmbedded(t *testing.T) {
	repl_env, _ := env.NewEnv(nil, nil, nil)
	for k, v := range core.NS {
		repl_env.Set(
			Symbol{Val: k},
			Func{Fn: v.(func([]MalType, *context.Context) (MalType, error))},
		)
	}

	str :=
		`;; $0 "hello"
;; $1 {"key" "value"}
;; $NUMBER 44
;; $4 (+ 1 1)

(do
	(def! v0 $0)
	(def! v1 $1)
	(def! v2 $NUMBER)
	(def! v3 $3) ;; this is nil
	(def! v4 '$4)
	true)
`

	// exp, err := READ_WithPlaceholders(str, nil, []MalType{"hello", "{\"key\": \"value\"}", 44, List{Val: []MalType{Symbol{Val: "quote"}, List{Val: []MalType{23, 37}}}}})
	exp, err := READ_WithPreamble(str, nil)
	if err != nil {
		t.Fatal(err)
	}

	// fmt.Println(PRINT(exp))

	res, err := EVAL(exp, repl_env, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.(bool) {
		v0, err := repl_env.Get(Symbol{Val: "v0"})
		if err != nil {
			t.Fatal(err)
		}
		if v0.(string) != "hello" {
			t.Fatal("no hello")
		}

		v1, err := repl_env.Get(Symbol{Val: "v1"})
		if err != nil {
			t.Fatal(err)
		}
		h, ok := v1.(HashMap)
		if !ok {
			t.Fatal("no {\"key\": \"value\"}")
		}
		if len(h.Val) != 1 {
			t.Fatal("pum")
		}
		if h.Val["key"].(string) != "value" {
			t.Fatal("pum2")
		}

		v2, err := repl_env.Get(Symbol{Val: "v2"})
		if err != nil {
			t.Fatal(err)
		}
		if v2.(int) != 44 {
			t.Fatal("no 44")
		}

		v3, err := repl_env.Get(Symbol{Val: "v3"})
		if err != nil {
			t.Fatal(err)
		}
		if v3 != nil {
			t.Fatal("no 2")
		}

		v4, err := repl_env.Get(Symbol{Val: "v4"})
		if err != nil {
			t.Fatal(err)
		}
		l, ok := v4.(List)
		if !ok {
			t.Fatal("no (+ 1 1)")
		}
		if len(l.Val) != 3 {
			t.Fatal("pum3")
		}
		if l.Val[0].(Symbol).Val != "+" {
			t.Fatal("pum4")
		}
		if l.Val[1].(int) != 1 {
			t.Fatal("pum5")
		}
		if l.Val[2].(int) != 1 {
			t.Fatal("pum6")
		}
	}
}

func TestPlaceholdersEmbeddedWrong1(t *testing.T) {
	repl_env, _ := env.NewEnv(nil, nil, nil)
	for k, v := range core.NS {
		repl_env.Set(
			Symbol{Val: k},
			Func{Fn: v.(func([]MalType, *context.Context) (MalType, error))},
		)
	}

	str :=
		`$0 "hello"
;; $1 {"key" "value"}
;; $NUMBER 44
;; $4 (+ 1 1)

(do
	(def! v0 $0)
	(def! v1 $1)
	(def! v2 $NUMBER)
	(def! v3 $3) ;; this is nil
	(def! v4 '$4)
	true)
`

	// exp, err := READ_WithPlaceholders(str, nil, []MalType{"hello", "{\"key\": \"value\"}", 44, List{Val: []MalType{Symbol{Val: "quote"}, List{Val: []MalType{23, 37}}}}})
	_, err := READ_WithPreamble(str, nil)
	if err == nil {
		t.Fatal("error expected but err was nil")
	}
	if err.Error() != "Error: not all tokens where parsed" {
		t.Fatal(err)
	}
}
