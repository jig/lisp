// Package nssql loads the SQL namespace into a jig/lisp environment,
// bundling pure-Go drivers for SQLite (modernc.org/sqlite, driver name
// "sqlite") and PostgreSQL (jackc/pgx, driver name "pgx"). Both are cgo-free
// so cross-compiled binaries need no C toolchain. An embedder that needs
// another driver just blank-imports it alongside this package.
package nssql

import (
	"context"
	"reflect"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"github.com/jig/lisp"
	"github.com/jig/lisp/lib/sql"
	"github.com/jig/lisp/types"
)

type Here struct{}

var (
	__package_fullpath__ = strings.Split(reflect.TypeFor[Here]().PkgPath(), "/")
	_package_            = "$" + __package_fullpath__[len(__package_fullpath__)-1]
)

func Load(env types.EnvType) error {
	sql.Load(env)

	if _, err := lisp.REPL(context.Background(), env, sql.HeaderSQL(), types.NewCursorFile(_package_)); err != nil {
		return err
	}

	return nil
}
