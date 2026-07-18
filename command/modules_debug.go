//go:build debugger

package command

import "github.com/jig/lisp/runtime"

// registerModule maps a module identifier (typically a script filename
// passed to NewCursorFile / NewCursorHere) to its absolute filesystem
// path. The DAP server consults this map when emitting `source.path` and
// when matching `setBreakpoints` requests against incoming Cursors.
func registerModule(module, absPath string) {
	runtime.Modules.Register(module, absPath)
}
