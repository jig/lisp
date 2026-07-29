package command

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/coreos/go-systemd/v22/journal"
	libgit "github.com/jig/lisp/lib/git"
	"github.com/jig/lisp/lib/integrity"
	liblog "github.com/jig/lisp/lib/log"
	"github.com/jig/lisp/lib/require"
	"github.com/jig/lisp/lib/system"
	libterm "github.com/jig/lisp/lib/term"
	"github.com/jig/lisp/types"
	"golang.org/x/term"
)

// ErrIntegrityReported signals that the integrity failure has already
// been shown to the user (the red block); main exits non-zero without
// re-printing it.
var ErrIntegrityReported = errors.New("integrity check failed")

// allowedSignersPath is the host's trust anchor for signature
// verification: its mere presence activates the signature rule for
// every lisp-integrity run. Overridable in tests.
var allowedSignersPath = "/etc/lisp/allowed_signers"

// Test seams: journald availability, record emission, and the
// confirmation prompt's TTY check.
var (
	journalEnabled  = journal.Enabled
	emitRecord      = liblog.Record
	stdinIsTerminal = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }
)

// integrityArgs is the lisp-integrity CLI: a script (or a test target),
// its arguments and the consent flag — nothing else, by design (no
// REPL, no -e, no environment overrides).
type integrityArgs struct {
	Yes      bool     `arg:"-y,--yes" help:"run without asking for confirmation (required when stdin is not a terminal)"`
	Test     string   `arg:"-t,--test" help:"verify and run the test suite from a directory or a single test file" placeholder:"DIR|FILE"`
	TestJSON string   `arg:"--test-json" help:"with --test, also write a JSON report to the given file" placeholder:"FILE"`
	Script   string   `arg:"positional" help:"lisp script to execute"`
	Args     []string `arg:"positional" help:"arguments to pass to the script"`
}

// PreParseIntegrityArgs extracts the script arguments before the
// libraries load, mirroring PreParseArgs for the lisp binary.
func PreParseIntegrityArgs(cmdArgs []string) []string {
	var parsedArgs integrityArgs
	parser, err := arg.NewParser(arg.Config{Program: "lisp-integrity"}, &parsedArgs)
	if err != nil {
		return []string{}
	}
	if len(cmdArgs) > 1 {
		if err := parser.Parse(cmdArgs[1:]); err != nil {
			return []string{}
		}
	}
	return parsedArgs.Args
}

// ExecuteIntegrity is the main function of the lisp-integrity binary:
// verify the script against HEAD (and, when the host has an
// allowed-signers set, the signature rule), report the audit block,
// ask for confirmation, attest the run to the journal and execute.
func ExecuteIntegrity(cmdArgs []string, repl_env types.EnvType) error {
	var parsedArgs integrityArgs
	parser, err := arg.NewParser(arg.Config{Program: "lisp-integrity"}, &parsedArgs)
	if err != nil {
		return err
	}
	if len(cmdArgs) > 1 {
		err = parser.Parse(cmdArgs[1:])
		if err == arg.ErrHelp {
			parser.WriteHelp(os.Stdout)
			return nil
		}
		if err != nil {
			return err
		}
	}
	testMode := parsedArgs.Test != ""
	switch {
	case parsedArgs.TestJSON != "" && !testMode:
		return fmt.Errorf("--test-json requires --test")
	case testMode && parsedArgs.Script != "":
		return fmt.Errorf("--test cannot be combined with a script file")
	case !testMode && (parsedArgs.Script == "" || parsedArgs.Script == "-"):
		return fmt.Errorf("lisp-integrity requires a script file or --test (stdin is not supported)")
	}

	// The audit trail must be protected before anything is attested:
	// journald on Linux — or refuse; the XDG state file only on macOS
	// (development convenience, explicitly weaker).
	protected := journalEnabled()
	if !protected && runtime.GOOS != "darwin" {
		return fmt.Errorf("lisp-integrity requires systemd-journald for the audit trail (journal socket unavailable)")
	}

	keys := ""
	if b, err := os.ReadFile(allowedSignersPath); err == nil {
		keys = string(b)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading %s: %w", allowedSignersPath, err)
	}

	// A script anchors on itself (verified at startup); a test target
	// anchors on its repository, and every test file is verified as it
	// loads (runTests reads through load-file, which is hooked).
	var enableErr error
	if testMode {
		enableErr = integrity.EnableDir(parsedArgs.Test, keys)
	} else {
		enableErr = integrity.Enable(parsedArgs.Script, keys)
	}
	if enableErr != nil {
		return reportIntegrity(false, "", "", "", enableErr)
	}
	require.VerifyModule = integrity.VerifyFile
	system.VerifySource = integrity.VerifyFile

	// With an allowed-signers set, git commits/tags and state-save made
	// during the run are SSH-signed with the ssh-agent key that is
	// listed in it. No private key ever enters the process; the
	// resolution is lazy and fails closed if no listed key is loaded.
	if keys != "" {
		libgit.SetSigningKeys(os.Getenv("SSH_AUTH_SOCK"), keys)
	}

	if err := reportIntegrity(true, integrity.RepoName(), integrity.CommitHash(), integrity.Signer(), nil); err != nil {
		return err
	}

	// Explicit consent: ask on the terminal, or require -y. Never run
	// on an unattended invocation that did not state its consent.
	if !parsedArgs.Yes {
		if !stdinIsTerminal() {
			return fmt.Errorf("refusing to run without confirmation: stdin is not a terminal (pass -y)")
		}
		fmt.Fprint(os.Stderr, "proceed? [y/N] ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
		default:
			return fmt.Errorf("run aborted by operator")
		}
	}

	// Attestation: constant run fields on every record, a start record
	// before evaluation and an end record from a deferred handler that
	// also covers error returns and panics.
	traceID := make([]byte, 16)
	if _, err := rand.Read(traceID); err != nil {
		return err
	}
	target := parsedArgs.Script
	argvList := append([]string{parsedArgs.Script}, parsedArgs.Args...)
	if testMode {
		target = parsedArgs.Test
		argvList = []string{"--test", parsedArgs.Test}
	}
	liblog.SetIdentifier(strings.TrimSuffix(filepath.Base(filepath.Clean(target)), ".lisp"))
	liblog.SetRunFields(map[string]string{
		"commit":   integrity.CommitHash(),
		"repo":     integrity.RepoName(),
		"trace_id": hex.EncodeToString(traceID),
	})
	argv, err := json.Marshal(argvList)
	if err != nil {
		return err
	}
	startFields := map[string]string{
		"argv":      string(argv),
		"protected": fmt.Sprintf("%t", protected),
	}
	if signer := integrity.Signer(); signer != "" {
		startFields["signer"] = signer
	}
	if err := emitRecord(slog.LevelInfo, "run started", startFields); err != nil {
		return err
	}

	exitCode := "1"
	endLevel := slog.LevelError
	defer func() {
		fields := map[string]string{"exit_code": exitCode}
		if r := recover(); r != nil {
			fields["panic"] = fmt.Sprint(r)
			_ = emitRecord(slog.LevelError, "run ended", fields)
			panic(r)
		}
		_ = emitRecord(endLevel, "run ended", fields)
	}()

	if testMode {
		if err := runTests(parsedArgs.Test, parsedArgs.TestJSON, repl_env); err != nil {
			return err
		}
	} else {
		result, err := runScript(context.Background(), repl_env, parsedArgs.Script, nil,
			types.NewCursorHere(parsedArgs.Script, -3, 1))
		if err != nil {
			return err
		}
		fmt.Println(result)
	}
	exitCode, endLevel = "0", slog.LevelInfo
	return nil
}

// reportIntegrity writes the audit outcome to stderr. On an interactive
// terminal it prints a human-readable block — green when integrity is
// satisfied, red when not; when stderr is redirected it emits a
// machine-readable JSON line instead. The block attests the anchor
// (and, in script mode, the entry script) only: each require/load-file
// — test files included — is verified as it loads and aborts the run
// on mismatch, so a green block does not mean the whole run is
// pre-verified. Returns nil on success, ErrIntegrityReported on
// a terminal failure (already shown), or the cause when redirected.
func reportIntegrity(ok bool, repo, commit, signer string, cause error) error {
	if !libterm.StderrIsTerminal() {
		if !ok {
			return cause // the normal error path reports it
		}
		attrs := []any{"repo", repo, "commit", commit}
		if signer != "" {
			attrs = append(attrs, "signer", signer)
		}
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).Info("integrity: verified at HEAD", attrs...)
		return nil
	}

	fmt.Fprint(os.Stderr, integrityBlock(ok, repo, commit, signer, cause))
	if ok {
		return nil
	}
	return ErrIntegrityReported
}

// integrityBlock renders the coloured, one-field-per-line audit block.
func integrityBlock(ok bool, repo, commit, signer string, cause error) string {
	field := func(label, value string) string {
		return fmt.Sprintf("    %-6s  %s\n", label, value)
	}
	if !ok {
		reason := strings.TrimPrefix(cause.Error(), "integrity: ")
		return libterm.StderrStyle("red", true, "✗ integrity check failed") + "\n" +
			libterm.StderrStyle("red", false, field("reason", reason))
	}
	body := field("repo", repo) + field("commit", commit)
	if signer != "" {
		body += field("signer", signer)
	}
	return libterm.StderrStyle("green", true, "✓ integrity verified") + "\n" +
		libterm.StderrStyle("green", false, body)
}
