//go:build !debugger

package command

// registerModule is a no-op in release builds. The module-to-path map
// only exists in `debugger` builds where the DAP server consumes it.
func registerModule(_, _ string) {}
