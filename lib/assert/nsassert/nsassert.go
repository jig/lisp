package nsassert

import (
	"context"
	"reflect"

	"github.com/jig/lisp"
	assert "github.com/jig/lisp/lib/assert"
	"github.com/jig/lisp/types"
)

type Here struct{}

var _package_ = reflect.TypeOf(Here{}).PkgPath()

func Load(env types.EnvType) error {
	if _, err := lisp.REPL(context.Background(), env, assert.HeaderAssertMacros(), types.NewCursorFile(_package_)); err != nil {
		return err
	}
	return nil
}
