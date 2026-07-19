package command

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/jig/lisp/lib/core"
	"github.com/jig/lisp/lib/integrity"
	"github.com/jig/lisp/lib/require"
)

// setupIntegrity validates the --integrity flags and enables integrity
// mode: the script is verified against the given Git ref before it
// runs, and the require and load-file loaders are hooked so every file
// evaluated as code in the same repository is verified first (files
// outside it are refused). No-op when --integrity was not given.
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
	if a.Eval != "" || a.Test != "" || a.Fmt || a.Version || a.BatSyntax || a.Debug ||
		a.DAP || a.DAPListen != "" || a.LSP || a.LSPListen != "" {
		return fmt.Errorf("--integrity only runs a script file; it cannot be combined with --eval, --test, --fmt, --debug or the server modes")
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
	core.VerifySource = integrity.VerifyFile

	// One structured line to stderr for the operator's audit trail.
	logAttrs := []any{"ref", integrity.Ref(), "commit", integrity.CommitHash()}
	if signers != "" {
		logAttrs = append(logAttrs, "signer", integrity.Signer())
	}
	slog.New(slog.NewJSONHandler(os.Stderr, nil)).Info("integrity verified", logAttrs...)
	return nil
}
