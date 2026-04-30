//go:build !lispdebug

package runtime

import (
	"context"

	"github.com/jig/lisp/types"
)

// Enabled reports whether hook dispatching is compiled in.
// Always false in release builds. Callers can guard with
// `if runtime.Enabled { ... }` and the compiler will eliminate the branch.
const Enabled = false

// Dispatch is a no-op in release builds. The constant `Enabled = false`
// guarantees this is dead code and the compiler removes the call site.
func Dispatch(_ context.Context, _ types.MalType, _ types.EnvType, _ *types.Position) error {
	return nil
}
