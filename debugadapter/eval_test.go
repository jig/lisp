//go:build lispdebug

package debugadapter

import (
	"context"

	"github.com/jig/lisp"
	"github.com/jig/lisp/types"
)

// evalSimple parses and evaluates source under the given env in the
// given context. Used by tests to drive the debugger.
func evalSimple(ctx context.Context, env types.EnvType, source string) (types.MalType, error) {
	ast, err := lisp.READ(source, types.NewCursorFile("test.lisp"), env)
	if err != nil {
		return nil, err
	}
	return lisp.EVAL(ctx, ast, env)
}
