//go:build debugger

package command

import "github.com/jig/lisp/runtime"

// setupDebugHook installs the legacy DEBUG-EVAL print hook. Only compiled
// into `debugger` builds.
func setupDebugHook() error {
	runtime.Hook = runtime.PrintEvalHook{}
	return nil
}
