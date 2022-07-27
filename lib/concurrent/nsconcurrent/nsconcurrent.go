package nsconcurrent

import (
	"context"
	"reflect"

	"github.com/jig/lisp"
	"github.com/jig/lisp/lib/concurrent"
	"github.com/jig/lisp/types"
)

var future = "(defmacro future (fn [& body] `(^{:once true} future-call (fn [] ~@body))))"

func Load(env types.EnvType) error {
	concurrent.Load(env)

	ctx := context.Background()
	for _, symbols := range []string{
		future,
	} {
		if _, err := lisp.REPL(ctx, env, `(eval (read-string (str "(do "`+symbols+`" nil)")))`, types.NewCursorFile(reflect.TypeOf(&symbols).PkgPath())); err != nil {
			return err
		}
	}

	return nil
}
