package command

import (
	"fmt"
	"os"

	"github.com/jig/lisp/lib/integrity"
	"github.com/jig/lisp/lib/require"
)

// setupIntegrity validates the --integrity flags and enables integrity
// mode: the script is verified against the given Git ref before it
// runs, and the require loader is hooked so every module in the same
// repository is verified before it is evaluated (modules outside it are
// refused). No-op when --integrity was not given.
func setupIntegrity(a args) error {
	if a.Integrity == "" {
		if a.IntegritySigners != "" {
			return fmt.Errorf("--integrity-signers requires --integrity")
		}
		return nil
	}
	if a.Script == "" || a.Script == "-" {
		return fmt.Errorf("--integrity requires a script file")
	}
	if a.Eval != "" || a.Test != "" || a.Fmt || a.Version || a.BatSyntax ||
		a.DAP || a.DAPListen != "" || a.LSP || a.LSPListen != "" {
		return fmt.Errorf("--integrity only runs a script file; it cannot be combined with --eval, --test, --fmt or the server modes")
	}
	signers := ""
	if a.IntegritySigners != "" {
		b, err := os.ReadFile(a.IntegritySigners)
		if err != nil {
			return fmt.Errorf("--integrity-signers: %w", err)
		}
		signers = string(b)
	}
	if err := integrity.Enable(a.Script, a.Integrity, signers); err != nil {
		return err
	}
	require.VerifyModule = integrity.VerifyFile
	return nil
}
