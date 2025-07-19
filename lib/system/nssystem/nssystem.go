package nssystem

import (
	"reflect"

	"github.com/jig/lisp/debug"
	"github.com/jig/lisp/lib/coreextented"
	"github.com/jig/lisp/lib/system"
	"github.com/jig/lisp/types"
)

type Here struct{}

var _package_ = reflect.TypeOf(Here{}).PkgPath()

func Load(env types.EnvType, dbg debug.Debug) error {
	if dbg != nil {
		dbg.PushFile(_package_, coreextented.HeaderCoreExtended())
	}
	system.Load(env, dbg)
	return nil
}
