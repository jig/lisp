package git_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/lib/git/nsgit"
	"github.com/jig/lisp/types"
	gossh "golang.org/x/crypto/ssh"
)

// newEnv loads core + the git namespace.
func newEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	if err := nscore.Load(ns); err != nil {
		t.Fatal(err)
	}
	if err := nscore.LoadInput(ns); err != nil {
		t.Fatal(err)
	}
	if err := nsgit.Load(ns); err != nil {
		t.Fatal(err)
	}
	return ns
}

// eval evaluates src and returns its value.
func eval(t *testing.T, ns types.EnvType, src string) types.MalType {
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

// expectTrue evaluates src and requires the result to be true.
func expectTrue(t *testing.T, ns types.EnvType, src string) {
	t.Helper()
	if v := eval(t, ns, src); v != true {
		t.Fatalf("%s = %v, want true", src, v)
	}
}

// expectThrow evaluates src and requires a catchable lisp error.
func expectThrow(t *testing.T, ns types.EnvType, src string) {
	t.Helper()
	ast, err := lisp.READ(src, types.NewCursorFile(t.Name()), ns)
	if err != nil {
		t.Fatalf("READ %s: %v", src, err)
	}
	if _, err := lisp.EVAL(context.Background(), ast, ns); err == nil {
		t.Fatalf("%s did not throw", src)
	}
}

// testKey generates an ed25519 key pair, returning the OpenSSH private key
// PEM and the authorized_keys line ("ssh-ed25519 AAAA... comment").
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

// objectFormats parametrizes tests over sha1 and sha256 repositories.
var objectFormats = []string{"sha1", "sha256"}

const author = `{:name "Test" :email "test@example.com"}`

func TestInitAddCommitLog(t *testing.T) {
	for _, format := range objectFormats {
		t.Run(format, func(t *testing.T) {
			ns := newEnv(t)
			dir := t.TempDir()
			eval(t, ns, fmt.Sprintf(`(def r (git/init %q {:object-format %q}))`, dir, format))
			if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hola\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			eval(t, ns, `(git/add r "a.txt")`)
			eval(t, ns, `(def c (git/commit r "first" {:author `+author+`}))`)
			expectTrue(t, ns, `(= false (get c :signed))`)
			expectTrue(t, ns, `(= "first" (get c :message))`)
			expectTrue(t, ns, `(= "Test" (get-in c [:author :name]))`)
			expectTrue(t, ns, `(= [] (get c :parents))`)
			wantLen := map[string]int{"sha1": 40, "sha256": 64}[format]
			hash, ok := eval(t, ns, `(get c :hash)`).(string)
			if !ok || len(hash) != wantLen {
				t.Fatalf("hash %q: want a %d-char hex string", hash, wantLen)
			}
			expectTrue(t, ns, `(= 1 (count (git/log r)))`)
			expectTrue(t, ns, `(= (get c :hash) (get (git/show r "HEAD") :hash))`)
			expectTrue(t, ns, `(get (git/status r) :clean)`)
			expectTrue(t, ns, `(= "master" (get (git/head r) :branch))`)
			eval(t, ns, `(git/close r)`)
		})
	}
}

func TestSignedCommitVerify(t *testing.T) {
	for _, format := range objectFormats {
		t.Run(format, func(t *testing.T) {
			ns := newEnv(t)
			dir := t.TempDir()
			privPEM, authorized := testKey(t, "alice")
			eval(t, ns, fmt.Sprintf(`(def r (git/init %q {:object-format %q}))`, dir, format))
			if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hola\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			eval(t, ns, `(git/add r "a.txt")`)
			eval(t, ns, fmt.Sprintf(`(def c (git/commit r "signed" {:author %s :sign {:key %q}}))`, author, privPEM))
			expectTrue(t, ns, `(get c :signed)`)
			eval(t, ns, fmt.Sprintf(`(def v (git/verify-commit r "HEAD" %q))`, authorized))
			expectTrue(t, ns, `(get v :valid)`)
			expectTrue(t, ns, `(= "ssh-ed25519" (get v :key-type))`)
			expectTrue(t, ns, `(= "sha512" (get v :hash-algorithm))`)
			expectTrue(t, ns, `(= "alice" (get v :signer))`)
			// log and show see the signed commit at HEAD
			expectTrue(t, ns, `(get (get (git/log r) 0) :signed)`)
			eval(t, ns, `(git/close r)`)
		})
	}
}

func TestVerifyFailClosed(t *testing.T) {
	ns := newEnv(t)
	dir := t.TempDir()
	privPEM, authorized := testKey(t, "alice")
	_, otherAuthorized := testKey(t, "mallory")
	eval(t, ns, fmt.Sprintf(`(def r (git/init %q))`, dir))
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	eval(t, ns, `(git/add r "a.txt")`)
	eval(t, ns, `(git/commit r "unsigned" {:author `+author+`})`)

	// unsigned commit: verify throws, verified? is false
	expectThrow(t, ns, fmt.Sprintf(`(git/verify-commit r "HEAD" %q)`, authorized))
	expectTrue(t, ns, fmt.Sprintf(`(= false (git/verified? r "HEAD" %q))`, authorized))

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	eval(t, ns, `(git/add r "a.txt")`)
	eval(t, ns, fmt.Sprintf(`(git/commit r "signed" {:author %s :sign {:key %q}})`, author, privPEM))

	// wrong key throws; a multi-line allowed-keys with the right key passes
	expectThrow(t, ns, fmt.Sprintf(`(git/verify-commit r "HEAD" %q)`, otherAuthorized))
	multi := "# team keys\n" + otherAuthorized + "\n" + authorized + "\n"
	expectTrue(t, ns, fmt.Sprintf(`(get (git/verify-commit r "HEAD" %q) :valid)`, multi))
	expectTrue(t, ns, fmt.Sprintf(`(= "alice" (get (git/verify-commit r "HEAD" %q) :signer))`, multi))
	expectTrue(t, ns, fmt.Sprintf(`(git/verified? r "HEAD" %q)`, authorized))
	eval(t, ns, `(git/close r)`)
}

func TestBranchesTagsStatus(t *testing.T) {
	ns := newEnv(t)
	dir := t.TempDir()
	privPEM, authorized := testKey(t, "alice")
	eval(t, ns, fmt.Sprintf(`(def r (git/init %q))`, dir))
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	expectTrue(t, ns, `(= false (get (git/status r) :clean))`)
	expectTrue(t, ns, `(= :untracked (get-in (git/status r) [:files "a.txt" :worktree]))`)
	eval(t, ns, `(git/add r ".")`)
	eval(t, ns, `(git/commit r "first" {:author `+author+`})`)
	expectTrue(t, ns, `(get (git/status r) :clean)`)

	// branches
	eval(t, ns, `(git/branch r "feature" {:checkout true})`)
	expectTrue(t, ns, `(= "feature" (get (git/head r) :branch))`)
	expectTrue(t, ns, `(= 2 (count (git/branches r)))`)
	eval(t, ns, `(git/checkout r "master")`)
	expectTrue(t, ns, `(= "master" (get (git/head r) :branch))`)

	// tags: lightweight, annotated, signed
	eval(t, ns, `(git/tag r "light")`)
	eval(t, ns, `(git/tag r "annotated" {:message "v1" :tagger `+author+`})`)
	eval(t, ns, fmt.Sprintf(`(git/tag r "signed" {:message "v2" :tagger %s :sign {:key %q}})`, author, privPEM))
	expectTrue(t, ns, `(= 3 (count (git/tags r)))`)
	expectTrue(t, ns, fmt.Sprintf(`(get (git/verify-tag r "signed" %q) :valid)`, authorized))
	expectThrow(t, ns, fmt.Sprintf(`(git/verify-tag r "annotated" %q)`, authorized))
	expectThrow(t, ns, fmt.Sprintf(`(git/verify-tag r "light" %q)`, authorized))
	expectTrue(t, ns, fmt.Sprintf(`(git/tag-verified? r "signed" %q)`, authorized))
	expectTrue(t, ns, fmt.Sprintf(`(= false (git/tag-verified? r "light" %q))`, authorized))

	// :sign without :message is rejected
	expectThrow(t, ns, fmt.Sprintf(`(git/tag r "bad" {:sign {:key %q}})`, privPEM))
	eval(t, ns, `(git/close r)`)
}

func TestPushPullFetchLocalRemote(t *testing.T) {
	for _, format := range objectFormats {
		t.Run(format, func(t *testing.T) {
			ns := newEnv(t)
			base := t.TempDir()
			work := filepath.Join(base, "work")
			bare := filepath.Join(base, "bare")
			clone := filepath.Join(base, "clone")

			eval(t, ns, fmt.Sprintf(`(def bare (git/init %q {:bare true :object-format %q}))`, bare, format))
			eval(t, ns, fmt.Sprintf(`(def r (git/init %q {:object-format %q}))`, work, format))
			if err := os.WriteFile(filepath.Join(work, "a.txt"), []byte("1\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			eval(t, ns, `(git/add r "a.txt")`)
			eval(t, ns, `(git/commit r "first" {:author `+author+`})`)
			eval(t, ns, fmt.Sprintf(`(git/remote-add r "origin" %q)`, bare))
			expectTrue(t, ns, `(= 1 (count (git/remotes r)))`)
			expectTrue(t, ns, `(= :ok (git/push r))`)
			expectTrue(t, ns, `(= :up-to-date (git/push r))`)

			eval(t, ns, fmt.Sprintf(`(def r2 (git/clone %q %q))`, bare, clone))
			expectTrue(t, ns, `(= (get (git/head r) :hash) (get (git/head r2) :hash))`)

			// new commit upstream, then pull and fetch downstream
			if err := os.WriteFile(filepath.Join(work, "a.txt"), []byte("2\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			eval(t, ns, `(git/add r "a.txt")`)
			eval(t, ns, `(git/commit r "second" {:author `+author+`})`)
			expectTrue(t, ns, `(= :ok (git/push r))`)
			expectTrue(t, ns, `(= :ok (git/pull r2))`)
			expectTrue(t, ns, `(= :up-to-date (git/pull r2))`)
			expectTrue(t, ns, `(= (get (git/head r) :hash) (get (git/head r2) :hash))`)
			expectTrue(t, ns, `(= 2 (count (git/log r2)))`)
			expectTrue(t, ns, `(= :up-to-date (git/fetch r2))`)
			eval(t, ns, `(do (git/close r) (git/close r2) (git/close bare))`)
		})
	}
}

// TestGitCLIInterop proves byte-level sshsig interoperability: commits and
// tags signed by this library verify with the system git.
func TestGitCLIInterop(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git CLI not available")
	}
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not available (git needs it to verify SSH signatures)")
	}
	for _, format := range objectFormats {
		t.Run(format, func(t *testing.T) {
			ns := newEnv(t)
			base := t.TempDir()
			dir := filepath.Join(base, "repo")
			privPEM, authorized := testKey(t, "")
			allowed := filepath.Join(base, "allowed_signers")
			if err := os.WriteFile(allowed, []byte("* "+authorized+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			eval(t, ns, fmt.Sprintf(`(def r (git/init %q {:object-format %q}))`, dir, format))
			if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hola\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			eval(t, ns, `(git/add r "a.txt")`)
			eval(t, ns, fmt.Sprintf(`(git/commit r "signed" {:author %s :sign {:key %q}})`, author, privPEM))
			eval(t, ns, fmt.Sprintf(`(git/tag r "v1" {:message "v1" :tagger %s :sign {:key %q}})`, author, privPEM))
			eval(t, ns, `(git/close r)`)

			for _, args := range [][]string{
				{"-C", dir, "fsck"},
				{"-C", dir, "-c", "gpg.ssh.allowedSignersFile=" + allowed, "verify-commit", "HEAD"},
				{"-C", dir, "-c", "gpg.ssh.allowedSignersFile=" + allowed, "verify-tag", "v1"},
			} {
				out, err := exec.Command("git", args...).CombinedOutput()
				if err != nil {
					t.Errorf("git %v: %v\n%s", args, err, out)
				}
			}
		})
	}
}
