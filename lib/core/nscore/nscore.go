package nscore

import (
	"context"
	"os"
	"reflect"

	"github.com/jig/lisp"
	"github.com/jig/lisp/lib/core"
	. "github.com/jig/lisp/types"
)

type Here struct{}

var _package_ = reflect.TypeOf(Here{}).PkgPath()

func Load(env EnvType) error {
	core.Load(env)
	env.Set(Symbol{Val: "eval"}, Func{Fn: func(ctx context.Context, a []MalType) (MalType, error) {
		return lisp.EVAL(ctx, a[0], env)
	}})

	if _, err := lisp.REPL(context.Background(), env, core.HeaderBasic(), NewCursorFile(_package_)); err != nil {
		return err
	}
	return nil
}

func LoadInput(env EnvType) error {
	core.LoadInput(env)
	env.Set(Symbol{Val: "eval"}, Func{Fn: func(ctx context.Context, a []MalType) (MalType, error) {
		return lisp.EVAL(ctx, a[0], env)
	}})

	if _, err := lisp.REPL(context.Background(), env, core.HeaderLoadFile(), NewCursorFile(_package_)); err != nil {
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
	env.Set(Symbol{Val: "*ARGV*"}, List{})
	return nil
}
