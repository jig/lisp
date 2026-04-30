//go:build lispdebug

package lisp

import "github.com/jig/lisp/runtime"

// installLegacyDebugHook installs runtime.PrintEvalHook on the first EVAL
// iteration that observes `DebugEvalEnabled = true` and no other hook
// already active. Only present in `lispdebug` builds.
func installLegacyDebugHook() {
	if runtime.Hook == nil {
		runtime.Hook = runtime.PrintEvalHook{}
	}
}
