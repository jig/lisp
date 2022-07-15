package nscore

import (
	"context"
	"os"

	"github.com/jig/lisp"
	"github.com/jig/lisp/lib/core"
	"github.com/jig/lisp/types"
	. "github.com/jig/lisp/types"
)

const (
	malHostLanguage = `(def *host-language* "go")`
	malNot          = `(def not (fn (a)
							(if a
								false
								true)))`
	malLoadFile = `(def load-file (fn (f)
						(eval
							(read-string
								(str "(do " (slurp f) " nil)")))))`
	malCond = `(defmacro cond (fn (& xs)
					(if (> (count xs) 0)
						(list
							'if (first xs)
								(if (> (count xs) 1)
									(nth xs 1)
									(throw "odd number of forms to cond"))
								(cons 'cond (rest (rest xs)))))))`
)

func Load(repl_env EnvType) error {
	for k, v := range core.NS {
		repl_env.Set(Symbol{Val: k}, Func{Fn: v.(func([]MalType, *context.Context) (MalType, error))})
	}
	repl_env.Set(Symbol{Val: "eval"}, Func{Fn: func(a []MalType, ctx *context.Context) (MalType, error) {
		return lisp.EVAL(a[0], repl_env, ctx)
	}})

	if _, err := lisp.REPL(repl_env, malHostLanguage, nil); err != nil {
		return err
	}
	if _, err := lisp.REPL(repl_env, malNot, nil); err != nil {
		return err
	}
	if _, err := lisp.REPL(repl_env, malCond, nil); err != nil {
		return err
	}
	return nil
}

func LoadInput(repl_env EnvType) error {
	for k, v := range core.NSInput {
		repl_env.Set(Symbol{Val: k}, Func{Fn: v.(func([]MalType, *context.Context) (MalType, error))})
	}
	repl_env.Set(Symbol{Val: "eval"}, Func{Fn: func(a []MalType, ctx *context.Context) (MalType, error) {
		return lisp.EVAL(a[0], repl_env, ctx)
	}})

	if _, err := lisp.REPL(repl_env, malLoadFile, nil); err != nil {
		return err
	}
	return nil
}

func LoadCmdLineArgs(repl_env EnvType) error {
	if len(os.Args) > 2 {
		args := make([]MalType, 0, len(os.Args)-2)
		for _, a := range os.Args[2:] {
			args = append(args, a)
		}
		repl_env.Set(Symbol{Val: "*ARGV*"}, List{Val: args})
		return nil
	} else {
		return LoadNullArgs(repl_env)
	}
}

func LoadNullArgs(repl_env EnvType) error {
	repl_env.Set(Symbol{Val: "*ARGV*"}, types.List{})
	return nil
}
