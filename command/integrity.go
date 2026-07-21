package command

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/jig/lisp/lib/core"
	libgit "github.com/jig/lisp/lib/git"
	"github.com/jig/lisp/lib/integrity"
	"github.com/jig/lisp/lib/require"
	libterm "github.com/jig/lisp/lib/term"
)

// ErrIntegrityReported signals that the integrity failure has already
// been shown to the user (the red block); main exits non-zero without
// re-printing it.
var ErrIntegrityReported = errors.New("integrity check failed")

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
		return reportIntegrity(false, "", "", "", err)
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

	return reportIntegrity(true, integrity.Ref(), integrity.CommitHash(), integrity.Signer(), nil)
}

// reportIntegrity writes the audit outcome to stderr. On an interactive
// terminal it prints a human-readable block — green when integrity is
// satisfied, red when not; when stderr is redirected it emits the
// machine-readable JSON line instead (JSON is the norm for log
// ingestion). The block attests the pinned ref and the entry script
// only: each require/load-file is verified as it loads and aborts the
// run on mismatch, so a green block does not mean the whole run is
// pre-verified. Returns nil on success, ErrIntegrityReported on a
// terminal failure (already shown), or the cause when redirected.
func reportIntegrity(ok bool, ref, commit, signer string, cause error) error {
	if !libterm.StderrIsTerminal() {
		if !ok {
			return cause // the normal error path reports it
		}
		attrs := []any{"ref", ref, "commit", commit}
		if signer != "" {
			attrs = append(attrs, "signer", signer)
		}
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).Info("integrity: entry script and ref verified", attrs...)
		return nil
	}

	fmt.Fprint(os.Stderr, integrityBlock(ok, ref, commit, signer, cause))
	if ok {
		return nil
	}
	return ErrIntegrityReported
}

// integrityBlock renders the coloured, one-field-per-line audit block.
func integrityBlock(ok bool, ref, commit, signer string, cause error) string {
	field := func(label, value string) string {
		return fmt.Sprintf("    %-6s  %s\n", label, value)
	}
	if !ok {
		reason := strings.TrimPrefix(cause.Error(), "integrity: ")
		return libterm.StderrStyle("red", true, "✗ integrity check failed") + "\n" +
			libterm.StderrStyle("red", false, field("reason", reason))
	}
	body := field("ref", ref) + field("commit", commit)
	if signer != "" {
		body += field("signer", signer)
	}
	return libterm.StderrStyle("green", true, "✓ integrity verified") + "\n" +
		libterm.StderrStyle("green", false, body)
}
