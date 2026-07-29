package integrity_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gogit "github.com/go-git/go-git/v6"
	gitconfig "github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
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
// function that removes it. git-commit, git-tag and state-save then sign.
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

// evalErr evaluates src and requires a (catchable) lisp error.
func evalErr(t *testing.T, ns types.EnvType, src string) error {
	t.Helper()
	ast, err := lisp.READ(src, types.NewCursorFile(t.Name()), ns)
	if err != nil {
		t.Fatalf("READ %s: %v", src, err)
	}
	_, err = lisp.EVAL(context.Background(), ast, ns)
	if err == nil {
		t.Fatalf("%s did not throw", src)
	}
	return err
}

// expectTrue evaluates src and requires the result to be true.
func expectTrue(t *testing.T, ns types.EnvType, src string) {
	t.Helper()
	if v := evalLisp(t, ns, src); v != true {
		t.Fatalf("%s = %v, want true", src, v)
	}
}

func TestStateSaveLoadWithoutIntegrity(t *testing.T) {
	ns := newGitEnv(t)
	dir, _ := setupRepo(t, ns, "")
	t.Chdir(dir)

	state := `{:z "last" :n 1 :who "operador" :entries [` +
		`{:payload "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnop" :label "alpha"} ` +
		`{:payload "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnop" :label "beta"}]}`
	hash, ok := evalLisp(t, ns, `(state-save "db" `+state+`)`).(string)
	if !ok || hash == "" {
		t.Fatalf("state-save did not return a commit hash")
	}

	contentBytes, err := os.ReadFile(filepath.Join(dir, ".state", "db.lisp"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(contentBytes)
	keyIndexes := []int{
		strings.Index(content, ":entries"),
		strings.Index(content, ":n 1"),
		strings.Index(content, ":who"),
		strings.Index(content, ":z"),
	}
	for i, index := range keyIndexes {
		if index < 0 || i > 0 && index <= keyIndexes[i-1] {
			t.Fatalf("state keys are not in deterministic order: %q", content)
		}
	}
	if !strings.Contains(content, "\n           {:label \"beta\"") ||
		!strings.Contains(content, "\n            :payload ") {
		t.Fatalf("nested state value is not multiline:\n%s", content)
	}
	if !strings.HasSuffix(content, "\n") || strings.HasSuffix(content, "\n\n") {
		t.Fatalf("state file must have exactly one trailing newline: %q", content)
	}

	expectTrue(t, ns, `(= `+state+` (state-load "db"))`)
	expectTrue(t, ns, `(= 1 (get (state-load "db") :n))`)
	expectTrue(t, ns, `(= "operador" (get (state-load "db") :who))`)
	expectTrue(t, ns, `(= 42 (state-load "missing" 42))`)
	_ = evalErr(t, ns, `(state-load "missing")`)
	_ = evalErr(t, ns, `(state-save "../evil" 1)`)
	_ = evalErr(t, ns, `(state-save ".hidden" 1)`)

	// The state commit is a real commit at HEAD touching only .state/.
	expectTrue(t, ns, fmt.Sprintf(`(= %q (get (git-show r "HEAD") :hash))`, hash))
	expectTrue(t, ns, `(= "state: db" (get (git-show r "HEAD") :message))`)
}

func TestStateSaveIgnoresCommitGPGSign(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)

	ns := newGitEnv(t)
	dir, _ := setupRepo(t, ns, "")
	t.Chdir(dir)

	repo, err := gogit.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := repo.Config()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Commit.GpgSign = gitconfig.OptBoolTrue
	if err := repo.SetConfig(cfg); err != nil {
		t.Fatal(err)
	}

	hash := evalLisp(t, ns, `(state-save "db" {:n 1})`).(string)
	commit, err := repo.CommitObject(plumbing.NewHash(hash))
	if err != nil {
		t.Fatal(err)
	}
	if commit.Signature != "" || commit.SignatureSHA256 != "" {
		t.Fatal("state-save commit must remain unsigned")
	}
}

func TestStateSaveSignedCommit(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			ns := newGitEnv(t)
			privPEM, authorized := testKey(t, "state-writer")
			dir := t.TempDir()
			t.Chdir(dir)
			t.Cleanup(signWith(t, privPEM))
			evalLisp(t, ns, fmt.Sprintf(
				`(def r (git-init %q {:object-format %q}))`,
				dir,
				format,
			))

			for n := 1; n <= 2; n++ {
				hash := evalLisp(t, ns, fmt.Sprintf(
					`(state-save "db" {:n %d})`,
					n,
				)).(string)
				repo, err := gogit.PlainOpen(dir)
				if err != nil {
					t.Fatal(err)
				}
				commit, err := repo.CommitObject(plumbing.NewHash(hash))
				if err != nil {
					t.Fatal(err)
				}
				if _, err := libgit.VerifyCommitSSH(commit, authorized); err != nil {
					t.Fatalf("verify signed state commit: %v", err)
				}
				if commit.Author.Name != "state-save" || commit.Author.Email != "state-save@lisp" {
					t.Fatalf("state commit author = %s <%s>", commit.Author.Name, commit.Author.Email)
				}
			}
			repo, err := gogit.PlainOpen(dir)
			if err != nil {
				t.Fatal(err)
			}

			const statePath = ".state/db.lisp"
			worktreeData, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(statePath)))
			if err != nil {
				t.Fatal(err)
			}
			idx, err := repo.Storer.Index()
			if err != nil {
				t.Fatal(err)
			}
			entry, err := idx.Entry(statePath)
			if err != nil {
				t.Fatal(err)
			}
			indexBlob, err := repo.BlobObject(entry.Hash)
			if err != nil {
				t.Fatal(err)
			}
			indexReader, err := indexBlob.Reader()
			if err != nil {
				t.Fatal(err)
			}
			indexData, err := io.ReadAll(indexReader)
			if closeErr := indexReader.Close(); err == nil {
				err = closeErr
			}
			if err != nil {
				t.Fatal(err)
			}
			head, err := repo.Head()
			if err != nil {
				t.Fatal(err)
			}
			headCommit, err := repo.CommitObject(head.Hash())
			if err != nil {
				t.Fatal(err)
			}
			headTree, err := headCommit.Tree()
			if err != nil {
				t.Fatal(err)
			}
			headFile, err := headTree.File(statePath)
			if err != nil {
				t.Fatal(err)
			}
			headData, err := headFile.Contents()
			if err != nil {
				t.Fatal(err)
			}

			// go-git v6 alpha.4 hashes racy worktree files with SHA-1 even
			// in SHA-256 repositories, so Status can report a false change.
			if string(worktreeData) != string(indexData) || string(worktreeData) != headData {
				t.Fatal("state differs between worktree, index, and HEAD")
			}
		})
	}
}

// TestStateSaveIdempotentUnchanged verifies that saving a byte-identical
// value is a no-op returning the existing commit, not an ErrEmptyCommit
// failure. Deterministic serialization makes an unchanged value produce
// an unchanged file, so without idempotence a repeated save would error.
func TestStateSaveIdempotentUnchanged(t *testing.T) {
	ns := newGitEnv(t)
	dir, _ := setupRepo(t, ns, "")
	t.Chdir(dir)

	first := evalLisp(t, ns, `(state-save "db" {:a 1 :b 2 :c 3})`).(string)
	repo, err := gogit.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	countState := func() int {
		iter, err := repo.Log(&gogit.LogOptions{})
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		_ = iter.ForEach(func(c *object.Commit) error {
			if strings.HasPrefix(c.Message, "state: ") {
				n++
			}
			return nil
		})
		return n
	}
	if countState() != 1 {
		t.Fatalf("expected 1 state commit, got %d", countState())
	}

	// Re-saving the identical value must not error and must not add a commit.
	second := evalLisp(t, ns, `(state-save "db" {:a 1 :b 2 :c 3})`).(string)
	if second != first {
		t.Fatalf("idempotent save returned %q, want the existing commit %q", second, first)
	}
	if countState() != 1 {
		t.Fatalf("idempotent save created a new commit: %d state commits", countState())
	}

	// A changed value still commits.
	third := evalLisp(t, ns, `(state-save "db" {:a 1 :b 2 :c 4})`).(string)
	if third == first {
		t.Fatal("changed save should produce a new commit")
	}
	if countState() != 2 {
		t.Fatalf("expected 2 state commits after a change, got %d", countState())
	}
}

// TestStateSaveMessage covers the optional commit-message argument:
// default "state: name", a custom message when given, and a type error
// for a non-string message.
func TestStateSaveMessage(t *testing.T) {
	ns := newGitEnv(t)
	dir, _ := setupRepo(t, ns, "")
	t.Chdir(dir)

	evalLisp(t, ns, `(state-save "db" {:n 1})`)             // default message
	evalLisp(t, ns, `(state-save "db" {:n 2} "bump to 2")`) // custom message

	repo, err := gogit.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	c, err := repo.CommitObject(head.Hash())
	if err != nil {
		t.Fatal(err)
	}
	if c.Message != "bump to 2" {
		t.Fatalf("HEAD message = %q, want %q", c.Message, "bump to 2")
	}
	parent, err := c.Parent(0)
	if err != nil {
		t.Fatal(err)
	}
	if parent.Message != "state: db" {
		t.Fatalf("parent (default) message = %q, want %q", parent.Message, "state: db")
	}

	if err := evalErr(t, ns, `(state-save "db" {:n 3} 42)`); !strings.Contains(err.Error(), "message must be a string") {
		t.Fatalf("non-string message = %v, want a type error", err)
	}
}

// TestStateChainSignedUnderKeys verifies that with allowed signers every
// state commit between HEAD and the release commit must be SSH-signed by
// a listed key: a signed chain verifies (with the release commit as the
// reported signer), an unsigned state commit on top fails closed.
func TestStateChainSignedUnderKeys(t *testing.T) {
	ns := newGitEnv(t)
	privPEM, authorized := testKey(t, "release")

	// Signed release commit + one signed state commit; HEAD is the
	// state commit.
	clear := signWith(t, privPEM)
	dir, _ := setupRepo(t, ns, "")
	t.Chdir(dir)
	evalLisp(t, ns, `(state-save "db" {:n 1})`)
	clear()

	script := filepath.Join(dir, "script.lisp")
	if err := enable(t, script, authorized); err != nil {
		t.Fatalf("Enable(signed state chain): %v", err)
	}
	if integrity.Signer() != "release" {
		t.Fatalf("Signer() = %q, want the release commit's key comment", integrity.Signer())
	}
	integrity.Disable()

	// An unsigned state commit on top must fail closed.
	evalLisp(t, ns, `(state-save "db" {:n 2})`)
	if err := enable(t, script, authorized); err == nil ||
		!strings.Contains(err.Error(), "not signed by an allowed key") {
		t.Fatalf("Enable(unsigned state commit) = %v, want 'not signed by an allowed key'", err)
	}
}

func TestStateUnderIntegrity(t *testing.T) {
	ns := newGitEnv(t)
	dir, _ := setupRepo(t, ns, "")
	script := filepath.Join(dir, "script.lisp")
	if err := enable(t, script, ""); err != nil {
		t.Fatal(err)
	}

	evalLisp(t, ns, `(state-save "db" {:n 1})`)
	evalLisp(t, ns, `(state-save "db" {:n 2})`)
	expectTrue(t, ns, `(= 2 (get (state-load "db") :n))`)

	// State commits move HEAD; a restart re-anchors there and the code
	// (unchanged by state commits) still verifies.
	integrity.Disable()
	if err := integrity.Enable(script, ""); err != nil {
		t.Fatalf("Enable after state commits: %v", err)
	}

	// An out-of-band edit of the state file fails closed.
	stateFile := filepath.Join(dir, ".state", "db.lisp")
	if err := os.WriteFile(stateFile, []byte("{:n 999}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := evalErr(t, ns, `(state-load "db")`); !strings.Contains(err.Error(), "differs") {
		t.Fatalf("state-load after tamper = %v, want 'differs'", err)
	}

	// An uncommitted state file (interrupted save) fails closed too.
	if err := os.WriteFile(filepath.Join(dir, ".state", "orphan.lisp"), []byte("{:n 1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := evalErr(t, ns, `(state-load "orphan")`); !strings.Contains(err.Error(), "not committed") {
		t.Fatalf("state-load of orphan = %v, want 'not committed'", err)
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
