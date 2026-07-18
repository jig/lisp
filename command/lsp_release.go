//go:build !debugger

package command

import (
	"errors"

	"github.com/jig/lisp/types"
)

// startLSP fails in release builds. The LSP server ships in the same
// artefact as the DAP server (the `debugger` build), keeping the
// release binary lean.
func startLSP(_ string, _ types.EnvType) error {
	return errors.New("--lsp requires a debug build: rebuild with -tags debugger")
}
