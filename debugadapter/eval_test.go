//go:build debugger

package debugadapter

import (
	"context"

	"github.com/jig/lisp"
	"github.com/jig/lisp/runtime"
	"github.com/jig/lisp/types"
)

// evalSimple parses and evaluates source under the given env in the
// given context. Used by tests to drive the debugger.
//
// The fake module name "test.lisp" is registered with runtime.Modules
// so the StepHook treats it as user code (otherwise the hook would
// skip every form, having decided no real source file exists).
func evalSimple(ctx context.Context, env types.EnvType, source string) (types.MalType, error) {
	const moduleName = "test.lisp"
	runtime.Modules.Register(moduleName, "/tmp/"+moduleName)
	ast, err := lisp.READ(source, types.NewCursorFile(moduleName), env)
	if err != nil {
		return nil, err
	}
	return lisp.EVAL(ctx, ast, env)
}
