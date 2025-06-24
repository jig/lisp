package lisp

import (
	"context"
	"os"
	"reflect"

	. "github.com/jig/lisp/types"
)

type HereNSCore struct{}

var _packageNSCore_ = reflect.TypeOf(HereNSCore{}).PkgPath()

func LoadNSCore(env EnvType) error {
	LoadCore(env)
	env.Set(Symbol{Val: "eval"}, Func{Fn: func(ctx context.Context, a []MalType) (MalType, error) {
		return EVAL(ctx, a[0], env)
	}})

	if _, err := REPL(context.Background(), env, HeaderBasic(), NewCursorFile(_package_)); err != nil {
		return err
	}
	return nil
}

func LoadNSCoreInput(env EnvType) error {
	LoadCoreInput(env)
	env.Set(Symbol{Val: "eval"}, Func{Fn: func(ctx context.Context, a []MalType) (MalType, error) {
		return EVAL(ctx, a[0], env)
	}})

	if _, err := REPL(context.Background(), env, HeaderLoadFile(), NewCursorFile(_package_)); err != nil {
		return err
	}
	return nil
}

func LoadNSCoreCmdLineArgs(env EnvType) error {
	if len(os.Args) > 2 {
		args := make([]MalType, 0, len(os.Args)-2)
		for _, a := range os.Args[2:] {
			args = append(args, a)
		}
		env.Set(Symbol{Val: "*ARGV*"}, List{Val: args})
		return nil
	} else {
		return LoadNSCoreNullArgs(env)
	}
}

func LoadNSCoreNullArgs(env EnvType) error {
	env.Set(Symbol{Val: "*ARGV*"}, List{})
	return nil
}
