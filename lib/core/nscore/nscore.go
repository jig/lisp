package nscore

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/jig/lisp"
	"github.com/jig/lisp/lib/call"
	"github.com/jig/lisp/lib/core"
	. "github.com/jig/lisp/types"
)

// evalDoc documents the eval builtin, registered directly (not via
// call.Call) in Load and LoadInput; call.Doc must run after each Set so
// the doc rides on the current Func value.
func docEval(env EnvType) {
	call.Doc(env, "eval", "[form]", "Evaluates a lisp form (AST) and returns its result.")
}

type Here struct{}

var (
	__package_fullpath__ = strings.Split(reflect.TypeFor[Here]().PkgPath(), "/")
	_package_            = "$" + __package_fullpath__[len(__package_fullpath__)-1]
)

func Load(env EnvType) error {
	core.Load(env)
	env.Set(Symbol{Val: "eval"}, Func{Fn: func(ctx context.Context, a []MalType) (MalType, error) {
		return lisp.EVAL(ctx, a[0], env)
	}})
	docEval(env)

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
	docEval(env)

	if _, err := lisp.REPL(context.Background(), env, core.HeaderLoadFile(), NewCursorFile(_package_)); err != nil {
		return err
	}

	// load-file-once needs mutable state for its seen-set; a Go closure
	// keeps it available with only core loaded (atoms belong to the
	// concurrent library).
	var mu sync.Mutex
	seen := map[string]bool{}
	env.Set(Symbol{Val: "load-file-once"}, Func{Fn: func(ctx context.Context, a []MalType) (MalType, error) {
		if len(a) != 1 {
			return nil, fmt.Errorf("load-file-once: wrong number of arguments (%d instead of 1)", len(a))
		}
		path, ok := a[0].(string)
		if !ok {
			return nil, fmt.Errorf("load-file-once: file-path must be a string (was of type %T)", a[0])
		}
		mu.Lock()
		already := seen[path]
		seen[path] = true
		mu.Unlock()
		if already {
			return nil, nil
		}
		return lisp.EVAL(ctx, NewList(nil, Symbol{Val: "load-file"}, path), env)
	}})
	call.Doc(env, "load-file-once", "[file-path]", "Like load-file, but never loads the same path twice.")

	return nil
}

func LoadCmdLineArgs(scriptArgs []string) func(EnvType) error {
	return func(env EnvType) error {
		return loadCmdLineArgs(env, scriptArgs)
	}
}

func loadCmdLineArgs(env EnvType, scriptArgs []string) error {
	if len(scriptArgs) > 0 {
		args := make([]MalType, 0, len(scriptArgs))
		for _, a := range scriptArgs {
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
