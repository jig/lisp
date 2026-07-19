package command

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/lib/coreextended/nscoreextended"
	"github.com/jig/lisp/lib/integrity"
	"github.com/jig/lisp/lib/integrity/nsintegrity"
	"github.com/jig/lisp/lib/require"
	"github.com/jig/lisp/lib/require/nsrequire"
	"github.com/jig/lisp/types"
)

// newIntegrityEnv loads the namespaces an --integrity script needs:
// core, coreextended, require and integrity.
func newIntegrityEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	for _, load := range []func(types.EnvType) error{
		nscore.Load, nscore.LoadInput, nscoreextended.Load,
		nsrequire.Load("lisp"), nsintegrity.Load,
	} {
		if err := load(ns); err != nil {
			t.Fatal(err)
		}
	}
	return ns
}

// runGit runs a git command in dir and returns its trimmed output,
// failing the test on error. Shared by the integrity tests.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=Test", "-c", "user.email=test@example.com"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// gitRepoWithScript builds a repository containing script.lisp (which
// requires .lisp/util.lisp and asserts integrity) and returns its dir
// and HEAD hash. It uses the git CLI, chdirs into the repo (so require
// resolves `.lisp/` there) and registers cleanup of the integrity
// globals the Execute call under test mutates.
func gitRepoWithScript(t *testing.T) (dir, hash string) {
	t.Helper()
	dir = t.TempDir()
	git := func(args ...string) string { return runGit(t, dir, args...) }
	script := "(require \"util\")\n(load-file \"extra.lisp\")\n(str (assert-integrity) \" \" (util/hello) \" \" extra-val)\n"
	if err := os.WriteFile(filepath.Join(dir, "script.lisp"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "extra.lisp"), []byte(`(def extra-val "extra")`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, ".lisp"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".lisp", "util.lisp"), []byte(`(def hello (fn [] "hola"))`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("init", "-q")
	git("add", "-A")
	git("commit", "-q", "-m", "first")
	hash = git("rev-parse", "HEAD")

	t.Chdir(dir)
	t.Cleanup(func() {
		integrity.Disable()
		require.VerifyModule = nil
		core.VerifySource = nil
	})
	return dir, hash
}

func TestExecuteIntegrityRunsVerifiedScript(t *testing.T) {
	_, hash := gitRepoWithScript(t)
	ns := newIntegrityEnv(t)
	out, err := captureStdout(t, func() error {
		return Execute([]string{"lisp", "--integrity", hash, "script.lisp"}, ns)
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, hash) || !strings.Contains(out, "hola") || !strings.Contains(out, "extra") {
		t.Fatalf("output %q: want the commit hash, the module's and the load-file's values", out)
	}
}

func TestExecuteIntegrityRejectsTamperedLoadFile(t *testing.T) {
	dir, hash := gitRepoWithScript(t)
	if err := os.WriteFile(filepath.Join(dir, "extra.lisp"), []byte(`(def extra-val "evil")`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ns := newIntegrityEnv(t)
	_, err := captureStdout(t, func() error {
		return Execute([]string{"lisp", "--integrity", hash, "script.lisp"}, ns)
	})
	if err == nil || !strings.Contains(err.Error(), "integrity") {
		t.Fatalf("Execute with tampered load-file target = %v, want integrity error", err)
	}
}

func TestExecuteIntegrityRejectsTamperedModule(t *testing.T) {
	dir, hash := gitRepoWithScript(t)
	if err := os.WriteFile(filepath.Join(dir, ".lisp", "util.lisp"), []byte(`(def hello (fn [] "evil"))`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ns := newIntegrityEnv(t)
	_, err := captureStdout(t, func() error {
		return Execute([]string{"lisp", "--integrity", hash, "script.lisp"}, ns)
	})
	if err == nil || !strings.Contains(err.Error(), "integrity") {
		t.Fatalf("Execute with tampered module = %v, want integrity error", err)
	}
}

func TestExecuteAssertIntegrityWithoutFlag(t *testing.T) {
	gitRepoWithScript(t)
	ns := newIntegrityEnv(t)
	_, err := captureStdout(t, func() error {
		return Execute([]string{"lisp", "script.lisp"}, ns)
	})
	if err == nil || !strings.Contains(err.Error(), "assert-integrity") {
		t.Fatalf("Execute without --integrity = %v, want assert-integrity throw", err)
	}
}

func TestExecuteIntegrityFlagValidation(t *testing.T) {
	ns := newIntegrityEnv(t)
	for _, cmdline := range [][]string{
		{"lisp", "--integrity", "HEAD"},                          // no script
		{"lisp", "--integrity", "HEAD", "-"},                     // stdin
		{"lisp", "--integrity", "HEAD", "-e", "1", "x.lisp"},     // eval
		{"lisp", "--integrity-keys", "keys.txt", "x.lisp"},       // keys alone
		{"lisp", "--integrity", "HEAD", "--fmt", "x.lisp"},       // fmt
		{"lisp", "--integrity", "HEAD", "--test", ".", "x.lisp"}, // test
		{"lisp", "--integrity", "HEAD", "--debug", "x.lisp"},     // debugger hook
	} {
		if err := Execute(cmdline, ns); err == nil {
			t.Errorf("Execute(%v) did not fail", cmdline)
		}
	}
}
