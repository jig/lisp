package nsassert

import (
	"context"
	"reflect"

	"github.com/jig/lisp"
	"github.com/jig/lisp/debug"
	assert "github.com/jig/lisp/lib/assert"
	"github.com/jig/lisp/types"
)

type Here struct{}

var _package_ = reflect.TypeOf(Here{}).PkgPath()

func Load(env types.EnvType, dbg debug.Debug) error {
	if dbg != nil {
		dbg.PushFile(_package_, assert.HeaderAssertMacros())
	}
	if _, err := lisp.REPL(context.Background(), env, assert.HeaderAssertMacros(), types.NewCursorFile(_package_), dbg); err != nil {
		return err
	}
	return nil
}
