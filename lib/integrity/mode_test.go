package integrity_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gogit "github.com/go-git/go-git/v6"
	gitconfig "github.com/go-git/go-git/v6/config"
	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core/nscore"
	libgit "github.com/jig/lisp/lib/git"
	"github.com/jig/lisp/lib/git/nsgit"
	"github.com/jig/lisp/lib/integrity"
	"github.com/jig/lisp/lib/integrity/nsintegrity"
	"github.com/jig/lisp/types"
	gossh "golang.org/x/crypto/ssh"
)

const author = `{:name "Test" :email "test@example.com"}`

// newGitEnv loads core plus the git and integrity namespaces.
func newGitEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	for _, load := range []func(types.EnvType) error{nscore.Load, nscore.LoadInput, nsgit.Load, nsintegrity.Load} {
		if err := load(ns); err != nil {
			t.Fatal(err)
		}
	}
	return ns
}

func evalLisp(t *testing.T, ns types.EnvType, src string) types.MalType {
	t.Helper()
	ast, err := lisp.READ(src, types.NewCursorFile(t.Name()), ns)
	if err != nil {
		t.Fatalf("READ %s: %v", src, err)
	}
	v, err := lisp.EVAL(context.Background(), ast, ns)
	if err != nil {
		t.Fatalf("EVAL %s: %v", src, err)
	}
	return v
}

// testKey generates an ed25519 key pair, returning the OpenSSH private
// key PEM and the authorized_keys line.
func testKey(t *testing.T, comment string) (privPEM, authorized string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := gossh.MarshalPrivateKey(priv, comment)
	if err != nil {
		t.Fatal(err)
	}
	sshPub, err := gossh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(string(gossh.MarshalAuthorizedKey(sshPub)))
	if comment != "" {
		line += " " + comment
	}
	return string(pem.EncodeToMemory(block)), line
}

// setupRepo creates a repository holding script.lisp and .lisp/util.lisp,
// commits both (with commitOpts merged into the git-commit options) and
// returns the repo dir and the commit hash. The lisp variable r remains
// bound to the open repo.
func setupRepo(t *testing.T, ns types.EnvType, commitOpts string) (dir, hash string) {
	t.Helper()
	dir = t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "script.lisp"), []byte("(assert-integrity)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, ".lisp"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".lisp", "util.lisp"), []byte(`(def hello (fn [] "hola"))`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	evalLisp(t, ns, fmt.Sprintf(`(def r (git-init %q))`, dir))
	evalLisp(t, ns, `(git-add r "script.lisp")`)
	evalLisp(t, ns, `(git-add r ".lisp/util.lisp")`)
	h, ok := evalLisp(t, ns, `(get (git-commit r "first" {:author `+author+` `+commitOpts+`}) :hash)`).(string)
	if !ok {
		t.Fatal("git-commit did not return a hash")
	}
	return dir, h
}

// signWith installs a signing policy from a PEM private key (the test
// stand-in for the allowed signers' ssh-agent signer) and returns the
// function that removes it. git-commit and annotated git-tag then sign.
func signWith(t *testing.T, privPEM string) func() {
	t.Helper()
	signer, err := gossh.ParsePrivateKey([]byte(privPEM))
	if err != nil {
		t.Fatal(err)
	}
	libgit.SetSigner(func() (gossh.Signer, error) { return signer, nil })
	return libgit.ClearSigner
}

// enable calls integrity.Enable and registers cleanup of the global mode.
func enable(t *testing.T, script, signers string) error {
	t.Helper()
	t.Cleanup(integrity.Disable)
	return integrity.Enable(script, signers)
}

func TestEnableAndVerify(t *testing.T) {
	ns := newGitEnv(t)
	dir, hash := setupRepo(t, ns, "")
	script := filepath.Join(dir, "script.lisp")

	if err := enable(t, script, ""); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !integrity.Active() {
		t.Fatal("Active() = false after Enable")
	}
	if got := integrity.CommitHash(); got != hash {
		t.Fatalf("CommitHash() = %q, want %q", got, hash)
	}

	module := filepath.Join(dir, ".lisp", "util.lisp")
	content, err := os.ReadFile(module)
	if err != nil {
		t.Fatal(err)
	}
	if err := integrity.VerifyFile(module, content); err != nil {
		t.Fatalf("VerifyFile(committed module): %v", err)
	}
	if err := integrity.VerifyFile(module, append(content, "(def evil 1)\n"...)); err == nil {
		t.Fatal("VerifyFile(modified module) did not fail")
	}
	if err := integrity.VerifyFile(filepath.Join(t.TempDir(), "out.lisp"), nil); err == nil ||
		!strings.Contains(err.Error(), "outside") {
		t.Fatalf("VerifyFile(outside repo) = %v, want 'outside' error", err)
	}
	if err := integrity.VerifyFile(filepath.Join(dir, "other.lisp"), nil); err == nil ||
		!strings.Contains(err.Error(), "not committed") {
		t.Fatalf("VerifyFile(uncommitted file) = %v, want 'not committed' error", err)
	}
}

func TestVerifyFileNoOpWhenInactive(t *testing.T) {
	if err := integrity.VerifyFile("/nowhere/x.lisp", []byte("whatever")); err != nil {
		t.Fatalf("VerifyFile with mode off: %v", err)
	}
}

func TestEnableModifiedScript(t *testing.T) {
	ns := newGitEnv(t)
	dir, _ := setupRepo(t, ns, "")
	script := filepath.Join(dir, "script.lisp")
	if err := os.WriteFile(script, []byte("(assert-integrity) (def evil 1)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := enable(t, script, ""); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("Enable(modified script) = %v, want 'differs' error", err)
	}
	if integrity.Active() {
		t.Fatal("Active() = true after failed Enable")
	}
}

// TestEnableAfterLaterCommit pins the HEAD-anchored semantics: a later
// commit moves HEAD, and verification simply anchors there — the pin
// is the checkout, not an argument.
func TestEnableAfterLaterCommit(t *testing.T) {
	ns := newGitEnv(t)
	dir, hash := setupRepo(t, ns, "")
	if err := os.WriteFile(filepath.Join(dir, "later.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	evalLisp(t, ns, `(git-add r "later.txt")`)
	evalLisp(t, ns, `(git-commit r "second" {:author `+author+`})`)
	if err := enable(t, filepath.Join(dir, "script.lisp"), ""); err != nil {
		t.Fatalf("Enable(after later commit): %v", err)
	}
	if integrity.CommitHash() == hash {
		t.Fatal("CommitHash() still reports the old commit; want the new HEAD")
	}
}

// TestEnableDir anchors on a directory without an entry script; files
// still verify (or fail) through VerifyFile as they load.
func TestEnableDir(t *testing.T) {
	ns := newGitEnv(t)
	dir, hash := setupRepo(t, ns, "")

	t.Cleanup(integrity.Disable)
	if err := integrity.EnableDir(filepath.Join(dir, ".lisp"), ""); err != nil {
		t.Fatalf("EnableDir: %v", err)
	}
	if got := integrity.CommitHash(); got != hash {
		t.Fatalf("CommitHash() = %q, want %q", got, hash)
	}

	module := filepath.Join(dir, ".lisp", "util.lisp")
	content, err := os.ReadFile(module)
	if err != nil {
		t.Fatal(err)
	}
	if err := integrity.VerifyFile(module, content); err != nil {
		t.Fatalf("VerifyFile(committed module): %v", err)
	}
	if err := integrity.VerifyFile(module, append(content, "(def evil 1)\n"...)); err == nil {
		t.Fatal("VerifyFile(modified module) did not fail")
	}
}

func TestEnableDirOutsideRepo(t *testing.T) {
	t.Cleanup(integrity.Disable)
	if err := integrity.EnableDir(t.TempDir(), ""); err == nil ||
		!strings.Contains(err.Error(), "not inside a git repository") {
		t.Fatalf("EnableDir(no repo) = %v, want 'not inside a git repository'", err)
	}
}

func TestEnableOutsideRepo(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "script.lisp")
	if err := os.WriteFile(script, []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := enable(t, script, ""); err == nil ||
		!strings.Contains(err.Error(), "not inside a git repository") {
		t.Fatalf("Enable(no repo) = %v, want 'not inside a git repository'", err)
	}
}

func TestEnableSignedCommit(t *testing.T) {
	ns := newGitEnv(t)
	privPEM, authorized := testKey(t, "alice")
	_, otherAuthorized := testKey(t, "mallory")
	t.Cleanup(signWith(t, privPEM))
	dir, _ := setupRepo(t, ns, "")
	script := filepath.Join(dir, "script.lisp")

	if err := enable(t, script, authorized); err != nil {
		t.Fatalf("Enable(signed commit, allowed key): %v", err)
	}
	if !integrity.Signed() || integrity.Signer() != "alice" {
		t.Fatalf("Signed()/Signer() = %v/%q, want true/alice", integrity.Signed(), integrity.Signer())
	}
	if fp := integrity.SignerFingerprint(); !strings.HasPrefix(fp, "SHA256:") {
		t.Fatalf("SignerFingerprint() = %q, want an SSH SHA256 fingerprint", fp)
	}
	integrity.Disable()
	if err := enable(t, script, otherAuthorized); err == nil {
		t.Fatal("Enable(signed commit, wrong key) did not fail")
	}
}

func TestEnableSignedTag(t *testing.T) {
	// An unsigned HEAD commit verifies through a signed annotated tag
	// pointing at it.
	ns := newGitEnv(t)
	privPEM, authorized := testKey(t, "alice")
	dir, _ := setupRepo(t, ns, "")
	script := filepath.Join(dir, "script.lisp")
	clear := signWith(t, privPEM)
	evalLisp(t, ns, `(git-tag r "v1" {:message "release" :tagger `+author+`})`)
	clear()

	if err := enable(t, script, authorized); err != nil {
		t.Fatalf("Enable(signed tag, allowed key): %v", err)
	}
}

func TestEnableUnsignedTagWithSigners(t *testing.T) {
	// An unsigned annotated tag does not rescue an unsigned commit.
	ns := newGitEnv(t)
	_, authorized := testKey(t, "alice")
	dir, _ := setupRepo(t, ns, "")
	evalLisp(t, ns, `(git-tag r "v2" {:message "release" :tagger `+author+`})`)
	if err := enable(t, filepath.Join(dir, "script.lisp"), authorized); err == nil ||
		!strings.Contains(err.Error(), "not signed") {
		t.Fatalf("Enable(unsigned tag with signers) = %v, want 'not signed'", err)
	}
}

func TestEnableUnsignedCommitWithSigners(t *testing.T) {
	ns := newGitEnv(t)
	_, authorized := testKey(t, "alice")
	dir, _ := setupRepo(t, ns, "")
	if err := enable(t, filepath.Join(dir, "script.lisp"), authorized); err == nil ||
		!strings.Contains(err.Error(), "not signed") {
		t.Fatalf("Enable(unsigned commit with signers) = %v, want 'not signed'", err)
	}
}

func TestRepoName(t *testing.T) {
	ns := newGitEnv(t)
	dir, _ := setupRepo(t, ns, "")
	script := filepath.Join(dir, "script.lisp")
	if err := enable(t, script, ""); err != nil {
		t.Fatal(err)
	}
	// No origin remote: the repository root's basename.
	if got := integrity.RepoName(); got != filepath.Base(dir) {
		t.Fatalf("RepoName() = %q, want %q", got, filepath.Base(dir))
	}
	integrity.Disable()

	repo, err := gogit.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{
		Name: "origin", URLs: []string{"git@github.com:jig/example.git"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := enable(t, script, ""); err != nil {
		t.Fatal(err)
	}
	if got := integrity.RepoName(); got != "git@github.com:jig/example.git" {
		t.Fatalf("RepoName() = %q, want the origin URL", got)
	}
}

func TestAssertIntegrityBuiltin(t *testing.T) {
	ns := newGitEnv(t)
	dir, hash := setupRepo(t, ns, "")

	// Without integrity mode the builtin throws (catchable).
	if got := evalLisp(t, ns, `(try (assert-integrity) (catch e "caught"))`); got != "caught" {
		t.Fatalf("assert-integrity without mode = %v, want caught throw", got)
	}

	if err := enable(t, filepath.Join(dir, "script.lisp"), ""); err != nil {
		t.Fatal(err)
	}
	if got := evalLisp(t, ns, `(assert-integrity)`); got != hash {
		t.Fatalf("assert-integrity = %v, want %q", got, hash)
	}

	// :with-signature demands the signature rule; this run had no
	// allowed signers, so it throws (catchable), as does any unknown
	// option.
	if got := evalLisp(t, ns, `(try (assert-integrity :with-signature) (catch e "caught"))`); got != "caught" {
		t.Fatalf("assert-integrity :with-signature without signers = %v, want caught throw", got)
	}
	if got := evalLisp(t, ns, `(try (assert-integrity :nonsense) (catch e "caught"))`); got != "caught" {
		t.Fatalf("assert-integrity with unknown option = %v, want caught throw", got)
	}
}

func TestAssertIntegrityWithSignature(t *testing.T) {
	ns := newGitEnv(t)
	privPEM, authorized := testKey(t, "alice")
	t.Cleanup(signWith(t, privPEM))
	dir, hash := setupRepo(t, ns, "")

	if err := enable(t, filepath.Join(dir, "script.lisp"), authorized); err != nil {
		t.Fatal(err)
	}
	if got := evalLisp(t, ns, `(assert-integrity :with-signature)`); got != hash {
		t.Fatalf("assert-integrity :with-signature = %v, want %q", got, hash)
	}
}
