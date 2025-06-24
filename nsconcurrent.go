package lisp

import (
	"context"
	"reflect"

	"github.com/jig/lisp/types"
)

type Here struct{}

var _package_ = reflect.TypeOf(Here{}).PkgPath()

func LoadNSConcurrent(env types.EnvType) error {
	LoadConcurrent(env)

	if _, err := REPL(context.Background(), env, HeaderConcurrent(), types.NewCursorFile(_package_)); err != nil {
		return err
	}

	return nil
}
