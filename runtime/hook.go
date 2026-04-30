// Package runtime exposes hooks that external tooling (debuggers, LSP
// servers) can install to observe or intercept Lisp evaluation.
//
// # Build modes
//
// The hook machinery is gated by the build tag `lispdebug`:
//
//   - **release** (default, no tag): runtime.Enabled == false. The hook
//     dispatch in EVAL is dead code and the compiler removes it. There is
//     no way to install a hook in this build, no performance overhead in
//     the hot path, and no in-process attack surface for an unwanted
//     observer of evaluation.
//
//   - **debug** (`-tags lispdebug`): runtime.Enabled == true. The exported
//     `Hook` variable accepts an EvalHook implementation. EVAL invokes
//     `runtime.Dispatch` once per iteration. Use this build for the
//     `--debug` CLI flag and for the upcoming DAP debugger and LSP
//     server.
//
// Build the production binary with `go build ./cmd/lisp`. Build the
// debug binary with `go build -tags lispdebug ./cmd/lisp`.
package runtime

import (
	"context"

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
//
// Only meaningful in `lispdebug` builds; see package doc.
type EvalHook interface {
	OnEval(ctx context.Context, ev EvalEvent) error
}
