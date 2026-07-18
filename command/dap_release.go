//go:build !debugger

package command

import (
	"errors"

	"github.com/jig/lisp/types"
)

// startDAP fails in release builds. The DAP server is gated behind the
// `debugger` build tag for the same reasons --debug is: zero hot-path
// overhead and no in-process surface to install a hook.
func startDAP(_ string, _ string, _ []string, _ string, _ types.EnvType) error {
	return errors.New("--dap requires a debug build: rebuild with -tags debugger")
}
