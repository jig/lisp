// Package runtime exposes hooks that external tooling (debuggers, LSP
// servers) can install to observe or intercept Lisp evaluation.
//
// The package is intentionally tiny: it provides a single pre-eval callback
// that is invoked once per EVAL iteration. Consumers are expected to keep
// any auxiliary state (call stacks, breakpoint tables, pause channels) in
// their own implementation of EvalHook.
//
// When Hook is nil, EVAL pays no overhead beyond a single nil check.
package runtime

import (
	"context"
	"fmt"

	"github.com/jig/lisp/printer"
	"github.com/jig/lisp/types"
)

// EvalEvent describes a single EVAL step about to happen.
type EvalEvent struct {
	AST    types.MalType
	Env    types.EnvType
	Cursor *types.Position
}

// EvalHook is invoked by EVAL before each iteration of its TCO loop.
// Returning a non-nil error aborts evaluation and propagates the error.
type EvalHook interface {
	OnEval(ctx context.Context, ev EvalEvent) error
}

// Hook is the active EvalHook. nil disables hooking entirely.
//
// Set this once before kicking off EVAL goroutines. Concurrent reads are
// safe (a single pointer assignment is atomic on supported platforms);
// concurrent writes are not synchronised — callers must coordinate.
var Hook EvalHook

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
