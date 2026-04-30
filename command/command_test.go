package command

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/types"
)

// captureStdout runs fn while redirecting os.Stdout, returning what was written.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = w

	errCh := make(chan error, 1)
	go func() { errCh <- fn() }()

	// Wait for fn to finish, then close writer so Read returns EOF.
	fnErr := <-errCh
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	return string(out), fnErr
}

func newTestEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	if err := nscore.Load(ns); err != nil {
		t.Fatalf("nscore.Load: %v", err)
	}
	if err := nscore.LoadInput(ns); err != nil {
		t.Fatalf("nscore.LoadInput: %v", err)
	}
	return ns
}

func TestExecute_EvalFlag(t *testing.T) {
	ns := newTestEnv(t)
	out, err := captureStdout(t, func() error {
		return Execute([]string{"lisp", "-e", "(+ 1 2)"}, ns)
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(out, "3") {
		t.Errorf("expected stdout to contain %q, got %q", "3", out)
	}
}

func TestExecute_EvalConflictsWithVersion(t *testing.T) {
	ns := newTestEnv(t)
	_, err := captureStdout(t, func() error {
		return Execute([]string{"lisp", "-e", "(+ 1 2)", "--version"}, ns)
	})
	if err == nil {
		t.Fatal("expected error when -e is combined with --version, got nil")
	}
	if !strings.Contains(err.Error(), "-e cannot be used") {
		t.Errorf("expected conflict error, got %v", err)
	}
}

func TestExecute_EvalConflictsWithTest(t *testing.T) {
	ns := newTestEnv(t)
	_, err := captureStdout(t, func() error {
		return Execute([]string{"lisp", "-e", "(+ 1 2)", "--test", "tests/"}, ns)
	})
	if err == nil {
		t.Fatal("expected error when -e is combined with --test, got nil")
	}
	if !strings.Contains(err.Error(), "-e cannot be used") {
		t.Errorf("expected conflict error, got %v", err)
	}
}

// TestExecute_ScriptAndEval verifies that when a script file and -e are
// combined the script's printed result is suppressed and only the -e result
// reaches stdout (see command.go: parsedArgs.Eval == "" guards in the script
// branch).
func TestExecute_ScriptAndEval(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "side.lisp")
	if err := os.WriteFile(scriptPath, []byte("(def shared 100)\n"), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	ns := newTestEnv(t)
	out, err := captureStdout(t, func() error {
		return Execute([]string{"lisp", "-e", "(+ shared 23)", scriptPath}, ns)
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(out, "123") {
		t.Errorf("expected eval result %q in stdout, got %q", "123", out)
	}
	// The script defines `shared` but does not print anything, so stdout
	// must not contain a stray "100" line from a re-printed script result.
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "100" {
			t.Errorf("script result leaked to stdout: %q", out)
		}
	}
}
