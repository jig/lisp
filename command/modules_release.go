//go:build !lispdebug

package command

// registerModule is a no-op in release builds. The module-to-path map
// only exists in `lispdebug` builds where the DAP server consumes it.
func registerModule(_, _ string) {}
