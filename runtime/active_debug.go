//go:build lispdebug

package runtime

import (
	"context"
	"fmt"

	"github.com/jig/lisp/printer"
	"github.com/jig/lisp/types"
)

// Enabled reports whether hook dispatching is compiled in.
// Always true in debug builds (`-tags lispdebug`).
const Enabled = true

// Hook is the active EvalHook. nil disables hooking.
//
// Set this once before kicking off EVAL goroutines. Concurrent reads are
// safe; concurrent writes are not synchronised — callers must coordinate.
var Hook EvalHook

// Dispatch invokes the active hook if any. EVAL calls this once per
// iteration when Enabled is true.
func Dispatch(ctx context.Context, ast types.MalType, env types.EnvType, cursor *types.Position) error {
	h := Hook
	if h == nil {
		return nil
	}
	return h.OnEval(ctx, EvalEvent{AST: ast, Env: env, Cursor: cursor})
}

// PrintEvalHook reproduces the legacy DEBUG-EVAL printing behaviour:
// when the symbol DEBUG-EVAL is bound to true in the active env, it prints
// the source position followed by the form being evaluated.
type PrintEvalHook struct{}

// OnEval implements EvalHook.
func (PrintEvalHook) OnEval(_ context.Context, ev EvalEvent) error {
	if ev.Env == nil {
		return nil
	}
	dbg, err := ev.Env.Get(types.Symbol{Val: "DEBUG-EVAL"})
	if err != nil {
		return nil
	}
	b, ok := dbg.(bool)
	if !ok || !b {
		return nil
	}
	if ev.Cursor != nil {
		fmt.Printf("\033[38;5;208m%s\033[0m: %s\n", ev.Cursor, printer.Pr_str(ev.AST, true))
	}
	return nil
}
