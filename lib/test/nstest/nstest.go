package nstest

import (
	"context"
	"reflect"
	"strings"

	"github.com/jig/lisp"
	"github.com/jig/lisp/lib/test"
	"github.com/jig/lisp/types"
)

type Here struct{}

var (
	__package_fullpath__ = strings.Split(reflect.TypeFor[Here]().PkgPath(), "/")
	_package_            = "$" + __package_fullpath__[len(__package_fullpath__)-1]
)

func Load(env types.EnvType) error {
	if err := test.Load(env); err != nil {
		return err
	}
	if _, err := lisp.REPL(context.Background(), env, test.HeaderTest(), types.NewCursorFile(_package_)); err != nil {
		return err
	}
	return nil
}
