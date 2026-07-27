package command

import (
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/lib/coreextended/nscoreextended"
	"github.com/jig/lisp/lib/integrity"
	"github.com/jig/lisp/lib/integrity/nsintegrity"
	"github.com/jig/lisp/lib/require"
	"github.com/jig/lisp/lib/require/nsrequire"
	"github.com/jig/lisp/lib/system"
	"github.com/jig/lisp/lib/system/nssystem"
	"github.com/jig/lisp/types"
)

// newIntegrityEnv loads the namespaces a lisp-integrity script needs:
// core, coreextended, require and integrity.
func newIntegrityEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	for _, load := range []func(types.EnvType) error{
		nscore.Load, nscore.LoadInput, nssystem.Load, nscoreextended.Load,
		nsrequire.Load("lisp"), nsintegrity.Load,
	} {
		if err := load(ns); err != nil {
			t.Fatal(err)
		}
	}
	return ns
}

// record is one attestation record captured by stubAttestation.
type record struct {
	level  slog.Level
	msg    string
	fields map[string]string
}

// stubAttestation makes ExecuteIntegrity hermetic: journald is reported
// available, records are captured instead of sent, stdin is reported as
// a non-terminal, and the allowed-signers path points nowhere. Restores
// everything on cleanup and returns the captured records.
func stubAttestation(t *testing.T) *[]record {
	t.Helper()
	var records []record
	origJournal, origEmit, origStdin, origPath := journalEnabled, emitRecord, stdinIsTerminal, allowedSignersPath
	journalEnabled = func() bool { return true }
	emitRecord = func(l slog.Level, msg string, fields map[string]string) error {
		records = append(records, record{level: l, msg: msg, fields: fields})
		return nil
	}
	stdinIsTerminal = func() bool { return false }
	allowedSignersPath = filepath.Join(t.TempDir(), "no-signers")
	t.Cleanup(func() {
		journalEnabled = origJournal
		emitRecord = origEmit
		stdinIsTerminal = origStdin
		allowedSignersPath = origPath
	})
	return &records
}

// runGit runs a git command in dir and returns its trimmed output,
// failing the test on error. Shared by the integrity tests. Signing is
// disabled so the test is hermetic regardless of the runner's global
// commit.gpgsign / tag.gpgsign config.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	base := []string{
		"-c", "user.name=Test", "-c", "user.email=test@example.com",
		"-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false",
	}
	cmd := exec.Command("git", append(base, args...)...)
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
// globals the ExecuteIntegrity call under test mutates.
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
		system.VerifySource = nil
	})
	return dir, hash
}

func TestExecuteIntegrityRunsVerifiedScript(t *testing.T) {
	records := stubAttestation(t)
	_, hash := gitRepoWithScript(t)
	ns := newIntegrityEnv(t)
	out, err := captureStdout(t, func() error {
		return ExecuteIntegrity([]string{"lisp-integrity", "-y", "script.lisp", "one", "two"}, ns)
	})
	if err != nil {
		t.Fatalf("ExecuteIntegrity: %v", err)
	}
	if !strings.Contains(out, hash) || !strings.Contains(out, "hola") || !strings.Contains(out, "extra") {
		t.Fatalf("output %q: want the commit hash, the module's and the load-file's values", out)
	}

	// The run is attested: a start record with argv, an end record with
	// exit_code 0.
	if len(*records) != 2 {
		t.Fatalf("expected start+end records, got %d: %+v", len(*records), *records)
	}
	start, end := (*records)[0], (*records)[1]
	if start.msg != "run started" || !strings.Contains(start.fields["argv"], `"script.lisp"`) ||
		!strings.Contains(start.fields["argv"], `"two"`) {
		t.Fatalf("unexpected start record: %+v", start)
	}
	if start.fields["protected"] != "true" || start.fields["signer"] != "" {
		t.Fatalf("unexpected start record fields: %+v", start.fields)
	}
	if end.msg != "run ended" || end.fields["exit_code"] != "0" || end.level != slog.LevelInfo {
		t.Fatalf("unexpected end record: %+v", end)
	}
}

func TestExecuteIntegrityRejectsTamperedLoadFile(t *testing.T) {
	records := stubAttestation(t)
	dir, _ := gitRepoWithScript(t)
	if err := os.WriteFile(filepath.Join(dir, "extra.lisp"), []byte(`(def extra-val "evil")`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ns := newIntegrityEnv(t)
	_, err := captureStdout(t, func() error {
		return ExecuteIntegrity([]string{"lisp-integrity", "-y", "script.lisp"}, ns)
	})
	if err == nil || !strings.Contains(err.Error(), "integrity") {
		t.Fatalf("ExecuteIntegrity with tampered load-file target = %v, want integrity error", err)
	}
	// The failure happened mid-run: the end record reports exit_code 1.
	end := (*records)[len(*records)-1]
	if end.msg != "run ended" || end.fields["exit_code"] != "1" || end.level != slog.LevelError {
		t.Fatalf("unexpected end record after failure: %+v", end)
	}
}

func TestExecuteIntegrityRejectsTamperedModule(t *testing.T) {
	stubAttestation(t)
	dir, _ := gitRepoWithScript(t)
	if err := os.WriteFile(filepath.Join(dir, ".lisp", "util.lisp"), []byte(`(def hello (fn [] "evil"))`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ns := newIntegrityEnv(t)
	_, err := captureStdout(t, func() error {
		return ExecuteIntegrity([]string{"lisp-integrity", "-y", "script.lisp"}, ns)
	})
	if err == nil || !strings.Contains(err.Error(), "integrity") {
		t.Fatalf("ExecuteIntegrity with tampered module = %v, want integrity error", err)
	}
}

func TestExecuteAssertIntegrityUnderPlainLisp(t *testing.T) {
	gitRepoWithScript(t)
	ns := newIntegrityEnv(t)
	_, err := captureStdout(t, func() error {
		return Execute([]string{"lisp", "script.lisp"}, ns)
	})
	if err == nil || !strings.Contains(err.Error(), "assert-integrity") {
		t.Fatalf("Execute under plain lisp = %v, want assert-integrity throw", err)
	}
}

func TestExecuteIntegrityRefusesWithoutConsent(t *testing.T) {
	stubAttestation(t) // stdin reported as non-terminal
	gitRepoWithScript(t)
	ns := newIntegrityEnv(t)
	_, err := captureStdout(t, func() error {
		return ExecuteIntegrity([]string{"lisp-integrity", "script.lisp"}, ns)
	})
	if err == nil || !strings.Contains(err.Error(), "confirmation") {
		t.Fatalf("ExecuteIntegrity without -y on non-terminal = %v, want refusal", err)
	}
}

func TestExecuteIntegrityRequiresJournald(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("darwin falls back to the XDG state file")
	}
	stubAttestation(t)
	journalEnabled = func() bool { return false }
	gitRepoWithScript(t)
	ns := newIntegrityEnv(t)
	_, err := captureStdout(t, func() error {
		return ExecuteIntegrity([]string{"lisp-integrity", "-y", "script.lisp"}, ns)
	})
	if err == nil || !strings.Contains(err.Error(), "journald") {
		t.Fatalf("ExecuteIntegrity without journald = %v, want refusal", err)
	}
}

func TestExecuteIntegrityArgValidation(t *testing.T) {
	stubAttestation(t)
	ns := newIntegrityEnv(t)
	for _, cmdline := range [][]string{
		{"lisp-integrity"},                        // no script
		{"lisp-integrity", "-y", "-"},             // stdin
		{"lisp-integrity", "-e", "1", "x.lisp"},   // unknown flag
		{"lisp-integrity", "--fmt", "x.lisp"},     // unknown flag
		{"lisp-integrity", "--test", ".", "x.l"},  // unknown flag
		{"lisp-integrity", "--integrity", "HEAD"}, // v1 flag is gone
	} {
		if err := ExecuteIntegrity(cmdline, ns); err == nil {
			t.Errorf("ExecuteIntegrity(%v) did not fail", cmdline)
		}
	}
}

func TestExecuteRejectsRemovedIntegrityFlags(t *testing.T) {
	ns := newIntegrityEnv(t)
	for _, cmdline := range [][]string{
		{"lisp", "--integrity", "HEAD", "x.lisp"},
		{"lisp", "--integrity-keys", "keys.txt", "x.lisp"},
	} {
		if err := Execute(cmdline, ns); err == nil {
			t.Errorf("Execute(%v) did not fail", cmdline)
		}
	}
}

// TestIntegrityBlockFormat pins the human-readable audit block: a green
// header plus one field per line on success, a red header plus reason on
// failure. Asserts the text (robust to whether color is on).
func TestIntegrityBlockFormat(t *testing.T) {
	ok := integrityBlock(true, "git@github.com:jig/example.git", "abc123def", "alice", nil)
	for _, want := range []string{"integrity verified", "repo", "example", "commit", "abc123def", "signer", "alice"} {
		if !strings.Contains(ok, want) {
			t.Errorf("success block missing %q:\n%s", want, ok)
		}
	}
	if n := strings.Count(strings.TrimRight(ok, "\n"), "\n"); n != 3 {
		t.Errorf("expected header + 3 fields (3 newlines), got %d:\n%s", n, ok)
	}

	noSigner := integrityBlock(true, "example", "abc", "", nil)
	if strings.Contains(noSigner, "signer") {
		t.Errorf("no signer line expected when signer is empty:\n%s", noSigner)
	}

	fail := integrityBlock(false, "", "", "", errors.New("integrity: HEAD moved"))
	for _, want := range []string{"integrity check failed", "reason", "HEAD moved"} {
		if !strings.Contains(fail, want) {
			t.Errorf("failure block missing %q:\n%s", want, fail)
		}
	}
	if strings.Contains(fail, "integrity: HEAD moved") {
		t.Errorf("expected the redundant 'integrity: ' prefix stripped:\n%s", fail)
	}
}
