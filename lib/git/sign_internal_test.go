package git

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
	gossh "golang.org/x/crypto/ssh"

	. "github.com/jig/lisp/types"
)

func newTestKey(t *testing.T) (gossh.Signer, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	sshPub, err := gossh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return signer, strings.TrimSpace(string(gossh.MarshalAuthorizedKey(sshPub)))
}

// signedCommit builds a repo with one commit signed through resignCommit.
func signedCommit(t *testing.T, signer gossh.Signer) (*Repo, *object.Commit) {
	t.Helper()
	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	r := &Repo{repo: repo, path: dir}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add("a.txt"); err != nil {
		t.Fatal(err)
	}
	who := &object.Signature{Name: "T", Email: "t@x", When: time.Now()}
	hash, err := wt.Commit("one", &gogit.CommitOptions{Author: who})
	if err != nil {
		t.Fatal(err)
	}
	if hash, err = resignCommit(r, hash, signer); err != nil {
		t.Fatal(err)
	}
	c, err := repo.CommitObject(hash)
	if err != nil {
		t.Fatal(err)
	}
	return r, c
}

// TestTamperedPayload transplants a valid signature onto a different
// commit: verification must fail because the signed payload differs.
func TestTamperedPayload(t *testing.T) {
	signer, authorized := newTestKey(t)
	_, victim := signedCommit(t, signer)

	tampered := *victim
	tampered.Message = "tampered message"
	if _, err := verifySignature(&tampered, victim.Signature, authorized); err == nil {
		t.Fatal("verification of a tampered payload succeeded")
	}
	// unaltered payload still verifies
	if _, err := verifySignature(victim, victim.Signature, authorized); err != nil {
		t.Fatalf("verification of the untampered commit failed: %v", err)
	}
	// a corrupted armored blob is rejected
	if _, err := verifySignature(victim, strings.Replace(victim.Signature, "A", "B", 1), authorized); err == nil {
		t.Fatal("verification with a corrupted signature blob succeeded")
	}
}

func TestClientOpts(t *testing.T) {
	opt := func(kv ...MalType) map[string]MalType {
		auth := HashMap{Val: map[string]MalType{}}
		for i := 0; i < len(kv); i += 2 {
			auth.Val[kv[i].(string)] = kv[i+1]
		}
		return map[string]MalType{NewKeyword("auth"): auth}
	}

	// absent :auth → no options
	if opts, err := clientOpts(map[string]MalType{}); err != nil || opts != nil {
		t.Fatalf("no auth: got %v, %v", opts, err)
	}
	// bad shapes error
	for name, o := range map[string]map[string]MalType{
		"non-map":    {NewKeyword("auth"): 42},
		"empty map":  opt(),
		"bad ssh":    opt(NewKeyword("ssh-key"), "not a pem"),
		"typed keys": opt(NewKeyword("username"), 7),
	} {
		if _, err := clientOpts(o); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	// valid shapes yield exactly one client option
	for name, o := range map[string]map[string]MalType{
		"basic":  opt(NewKeyword("username"), "u", NewKeyword("password"), "p"),
		"token":  opt(NewKeyword("token"), "t"),
		"bearer": opt(NewKeyword("bearer"), "t"),
	} {
		opts, err := clientOpts(o)
		if err != nil || len(opts) != 1 {
			t.Errorf("%s: got %d options, err %v", name, len(opts), err)
		}
	}
}
