//go:build !lispdebug

package lisp

// installLegacyDebugHook is a no-op in release builds. The call site in
// EVAL is dead code under `if runtime.Enabled` (compile-time false).
func installLegacyDebugHook() {}
