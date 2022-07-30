package nscore

import (
	"context"
	"os"
	"reflect"

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
								(str ";; $MODULE " f "\n(do " (slurp f) " nil)")))))`
	malCond = `(defmacro cond (fn (& xs)
					(if (> (count xs) 0)
						(list
							'if (first xs)
								(if (> (count xs) 1)
									(nth xs 1)
									(throw "odd number of forms to cond"))
								(cons 'cond (rest (rest xs)))))))`
)

type Here struct{}

var _package_ = reflect.TypeOf(Here{}).PkgPath()

func Load(env EnvType) error {
	core.Load(env)
	env.Set(Symbol{Val: "eval"}, Func{Fn: func(ctx context.Context, a []MalType) (MalType, error) {
		return lisp.EVAL(ctx, a[0], env)
	}})

	ctx := context.Background()
	if _, err := lisp.REPL(ctx, env, malHostLanguage, types.NewCursorFile(_package_)); err != nil {
		return err
	}
	if _, err := lisp.REPL(ctx, env, malNot, types.NewCursorFile(_package_)); err != nil {
		return err
	}
	if _, err := lisp.REPL(ctx, env, malCond, types.NewCursorFile(_package_)); err != nil {
		return err
	}
	return nil
}

func LoadInput(env EnvType) error {
	core.LoadInput(env)
	env.Set(Symbol{Val: "eval"}, Func{Fn: func(ctx context.Context, a []MalType) (MalType, error) {
		return lisp.EVAL(ctx, a[0], env)
	}})

	ctx := context.Background()
	if _, err := lisp.REPL(ctx, env, malLoadFile, types.NewCursorFile(_package_)); err != nil {
		return err
	}
	return nil
}

func LoadCmdLineArgs(env EnvType) error {
	if len(os.Args) > 2 {
		args := make([]MalType, 0, len(os.Args)-2)
		for _, a := range os.Args[2:] {
			args = append(args, a)
		}
		env.Set(Symbol{Val: "*ARGV*"}, List{Val: args})
		return nil
	} else {
		return LoadNullArgs(env)
	}
}

func LoadNullArgs(env EnvType) error {
	env.Set(Symbol{Val: "*ARGV*"}, types.List{})
	return nil
}
