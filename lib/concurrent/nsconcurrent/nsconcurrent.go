package nsconcurrent

import (
	"context"
	"reflect"

	"github.com/jig/lisp"
	"github.com/jig/lisp/lib/concurrent"
	"github.com/jig/lisp/types"
)

type Here struct{}

var _package_ = reflect.TypeFor[Here]().PkgPath()

func Load(env types.EnvType) error {
	concurrent.Load(env)

	if _, err := lisp.REPL(context.Background(), env, concurrent.HeaderConcurrent(), types.NewCursorFile(_package_)); err != nil {
		return err
	}

	return nil
}
