// Package nsweb loads the web (Ring-style HTTP server) namespace into a
// jig/lisp environment: the Go builtins (web-serve, web-router,
// web-verify-jwt) plus the response helpers and middleware
// defined in header-web.lisp.
package nsweb

import (
	"context"
	"reflect"
	"strings"

	"github.com/jig/lisp"
	"github.com/jig/lisp/lib/web"
	"github.com/jig/lisp/types"
)

type Here struct{}

var (
	__package_fullpath__ = strings.Split(reflect.TypeFor[Here]().PkgPath(), "/")
	_package_            = "$" + __package_fullpath__[len(__package_fullpath__)-1]
)

func Load(env types.EnvType) error {
	web.Load(env)
	if _, err := lisp.REPL(context.Background(), env, web.HeaderWeb(), types.NewCursorFile(_package_)); err != nil {
		return err
	}
	return nil
}
