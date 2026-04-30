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

// Frame is a stub in release builds. The Frame contents only exist in
// `lispdebug` builds; this empty struct keeps consumer code (mal.go,
// debugadapter) compilable when wrapped in `if runtime.Enabled { ... }`.
type Frame struct{}

// MakeFrame returns nil in release builds. Dead code under
// `if runtime.Enabled`.
func MakeFrame(_ string, _ types.MalType, _ types.EnvType, _ *types.Position) *Frame {
	return nil
}

// UpdateFrame is a no-op in release builds.
func UpdateFrame(_ *Frame, _ types.MalType, _ types.EnvType, _ *types.Position) {}

// PushFrame is a no-op in release builds.
func PushFrame(_ context.Context, _ *Frame) bool { return false }

// PopFrame is a no-op in release builds.
func PopFrame(_ context.Context) {}

// FrameID always returns 0 in release builds — Frame has no fields.
func FrameID(_ *Frame) int64 { return 0 }
