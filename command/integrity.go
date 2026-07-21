package command

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/jig/lisp/lib/core"
	libgit "github.com/jig/lisp/lib/git"
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
		if a.IntegrityKeys != "" {
			return fmt.Errorf("--integrity-keys requires --integrity")
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
	keys := ""
	if a.IntegrityKeys != "" {
		b, err := os.ReadFile(a.IntegrityKeys)
		if err != nil {
			return fmt.Errorf("--integrity-keys: %w", err)
		}
		keys = string(b)
	}
	if err := integrity.Enable(a.Script, a.Integrity, keys); err != nil {
		return err
	}
	require.VerifyModule = integrity.VerifyFile
	core.VerifySource = integrity.VerifyFile

	// With --integrity-keys, git commits/tags and state-save made during
	// the run are SSH-signed with the ssh-agent key that is listed in the
	// keys file. No private key ever enters the process; the resolution
	// is lazy and fails closed if no listed key is loaded in the agent.
	if keys != "" {
		libgit.SetSigningKeys(os.Getenv("SSH_AUTH_SOCK"), keys)
	}

	// One structured line to stderr for the operator's audit trail. It
	// attests the pinned ref and the entry script only; each require /
	// load-file is verified as it loads and aborts the run on mismatch,
	// so a green line here does not mean the whole run is pre-verified.
	logAttrs := []any{"ref", integrity.Ref(), "commit", integrity.CommitHash()}
	if keys != "" {
		logAttrs = append(logAttrs, "signer", integrity.Signer())
	}
	slog.New(slog.NewJSONHandler(os.Stderr, nil)).Info("integrity: entry script and ref verified", logAttrs...)
	return nil
}
