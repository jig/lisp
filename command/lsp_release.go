//go:build !lispdebug

package command

import (
	"errors"

	"github.com/jig/lisp/types"
)

// startLSP fails in release builds. The LSP server ships in the same
// artefact as the DAP server (the `lispdebug` build), keeping the
// release binary lean.
func startLSP(_ string, _ types.EnvType) error {
	return errors.New("--lsp requires a debug build: rebuild with -tags lispdebug")
}
