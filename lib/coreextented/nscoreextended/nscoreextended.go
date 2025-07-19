package nscoreextended

import (
	"context"
	_ "embed"
	"reflect"

	"github.com/jig/lisp"
	"github.com/jig/lisp/debug"
	"github.com/jig/lisp/lib/coreextented"
	"github.com/jig/lisp/types"
)

type Here struct{}

var _package_ = reflect.TypeOf(Here{}).PkgPath()

func Load(env types.EnvType, dbg debug.Debug) error {
	if dbg != nil {
		dbg.PushFile(_package_, coreextented.HeaderCoreExtended())
	}
	if _, err := lisp.REPL(context.Background(), env, coreextented.HeaderCoreExtended(), types.NewCursorFile(_package_), dbg); err != nil {
		return err
	}
	return nil
}
