package nssystem

import (
	"reflect"

	"github.com/jig/lisp/lib/system"
	"github.com/jig/lisp/types"
)

type Here struct{}

var _package_ = reflect.TypeOf(Here{}).PkgPath()

func Load(env types.EnvType) error {
	system.Load(env)
	return nil
}
