// Package nsgit loads the git namespace into a jig/lisp environment.
package nsgit

import (
	"context"
	"reflect"
	"strings"

	"github.com/jig/lisp"
	"github.com/jig/lisp/lib/git"
	"github.com/jig/lisp/types"
)

type Here struct{}

var (
	__package_fullpath__ = strings.Split(reflect.TypeFor[Here]().PkgPath(), "/")
	_package_            = "$" + __package_fullpath__[len(__package_fullpath__)-1]
)

func Load(env types.EnvType) error {
	git.Load(env)

	if _, err := lisp.REPL(context.Background(), env, git.HeaderGit(), types.NewCursorFile(_package_)); err != nil {
		return err
	}

	return nil
}
