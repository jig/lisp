package command

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jig/lisp/lib/integrity"
	"github.com/jig/lisp/lib/require"
	"github.com/jig/lisp/lib/system"
	"github.com/jig/lisp/types"
)

// TestRunScriptPreambleTOCTOU is a regression test for the double-read
// TOCTOU on runScript's in-file preamble branch: integrity.Enable reads
// and verifies the script once, then runScript reads it again and — on
// the preamble branch — evaluates that second buffer directly. If the
// file changes between the two reads (a worktree write, in scope), the
// evaluated bytes must still be verified, or a "verified" run executes
// non-committed code.
//
// The window is modelled deterministically, without a filesystem race:
// enable the mode on the committed script, then overwrite the file with
// non-committed bytes before calling runScript. The run must fail closed.
func TestRunScriptPreambleTOCTOU(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "script.lisp")

	// A leading ;; $WHO line forces the preamble branch (not the
	// load-file fast path, which reads and verifies the same bytes).
	committed := ";; $WHO \"world\"\n(println \"hello\" $WHO)\n"
	payload := ";; $WHO \"world\"\n(println \"PWNED: non-committed code\")\n"

	if err := os.WriteFile(scriptPath, []byte(committed), 0o644); err != nil {
		t.Fatal(err)
	}
	hash := gitInitCommit(t, dir)
	t.Chdir(dir)
	t.Cleanup(func() {
		integrity.Disable()
		require.VerifyModule = nil
		system.VerifySource = nil
	})

	// Enable verifies the committed script; wire the hooks exactly as
	// setupIntegrity does.
	if err := integrity.Enable(scriptPath, hash, ""); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	require.VerifyModule = integrity.VerifyFile
	system.VerifySource = integrity.VerifyFile

	// The attacker swaps the file after verification, before evaluation.
	if err := os.WriteFile(scriptPath, []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}

	ns := newIntegrityEnv(t)
	out, err := captureStdout(t, func() error {
		_, e := runScript(context.Background(), ns, "script.lisp", nil, types.NewCursorFile("script.lisp"))
		return e
	})
	if strings.Contains(out, "PWNED") {
		t.Fatalf("non-committed code executed under a verified run: %q", out)
	}
	if err == nil || !strings.Contains(err.Error(), "integrity") {
		t.Fatalf("preamble TOCTOU: got err=%v, want a fail-closed integrity error", err)
	}
}

// gitInitCommit initialises a repo in dir, commits everything and
// returns the HEAD hash. Uses the git CLI like the other tests here.
func gitInitCommit(t *testing.T, dir string) string {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"-c", "user.name=T", "-c", "user.email=t@e", "commit", "-qm", "release"},
	} {
		runGit(t, dir, args...)
	}
	return strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
}
