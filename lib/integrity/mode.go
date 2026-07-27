package integrity

// This file implements integrity mode (the `lisp-integrity` binary):
// the interpreter runs a script if and only if it matches what is
// committed in its Git repository at HEAD.
//
// Enable anchors on HEAD and byte-compares the script against the blob
// committed there. The command package then installs VerifyFile as the
// require loader's vetting hook, so the check cascades: every module a
// verified script requires must live in the same repository and match
// its committed blob too; anything outside the repository is refused.
// Pinning a release is a property of the checkout, not of the
// invocation: `git checkout --detach <tag>` keeps HEAD (and therefore
// every restart) on that exact commit; only the script's own
// state-save commits advance it, as children of it.
//
// With an allowed-signers file, Enable additionally requires the
// anchor to carry an SSH signature by one of the listed keys: HEAD's
// state-save commits (if any) must each be signed, and the release
// commit under them must be signed itself or via an annotated tag
// pointing at it — turning the guarantee from "matches the local
// repository" into "matches what a trusted key released".
//
// This is an operational assurance for the operator launching the
// script — no accidental drift, no uncommitted edits — not a security
// boundary against an attacker who can already write to the repository
// or replace the binary. Code the verified script chooses to load
// outside the require mechanism (slurp+eval) is not covered.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	libgit "github.com/jig/lisp/lib/git"
)

// modeState is the pinned verification context of a lisp-integrity run.
type modeState struct {
	repo     *gogit.Repository
	repoRoot string
	repoName string
	signer   string
	signed   bool
	commit   *object.Commit
	tree     *object.Tree
}

// mode is nil when the interpreter does not run under integrity. It
// is written once by Enable before any lisp code is evaluated and only
// read afterwards.
var mode *modeState

// Active reports whether the interpreter runs under integrity mode.
func Active() bool { return mode != nil }

// CommitHash returns the verified commit hash, or "" when inactive.
func CommitHash() string {
	if mode == nil {
		return ""
	}
	return mode.commit.Hash.String()
}

// RepoName returns the verified repository's origin remote URL, or the
// basename of its root directory when it has no origin; "" when
// inactive.
func RepoName() string {
	if mode == nil {
		return ""
	}
	return mode.repoName
}

// Signer returns the comment of the allowed key that signed the
// verified anchor, or "" when inactive or no signers were required.
func Signer() string {
	if mode == nil {
		return ""
	}
	return mode.signer
}

// Signed reports whether signature verification was performed (an
// allowed-signers set was present and the anchor verified against it).
func Signed() bool { return mode != nil && mode.signed }

// Disable turns integrity mode off. Exported for tests.
func Disable() { mode = nil }

// repoName derives the attestation repository name: the origin remote
// URL, or the repository root's basename without one.
func repoName(repo *gogit.Repository, root string) string {
	if remote, err := repo.Remote("origin"); err == nil {
		if urls := remote.Config().URLs; len(urls) > 0 && urls[0] != "" {
			return urls[0]
		}
	}
	return filepath.Base(root)
}

// Enable verifies scriptPath against HEAD of its enclosing Git
// repository and turns integrity mode on. allowedSigners, when
// non-empty, holds authorized_keys-format public keys and makes Enable
// additionally require the HEAD chain to be SSH-signed by them (see
// verifyHeadSignature).
func Enable(scriptPath, allowedSigners string) error {
	abs, err := filepath.Abs(scriptPath)
	if err != nil {
		return fmt.Errorf("integrity: %w", err)
	}
	repo, err := gogit.PlainOpenWithOptions(filepath.Dir(abs), &gogit.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return fmt.Errorf("integrity: %s is not inside a git repository", abs)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("integrity: %w", err)
	}
	root := wt.Filesystem().Root()

	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("integrity: %w", err)
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return fmt.Errorf("integrity: HEAD does not resolve to a commit: %w", err)
	}

	signer := ""
	if allowedSigners != "" {
		signer, err = verifyHeadSignature(repo, commit, allowedSigners)
		if err != nil {
			return fmt.Errorf("integrity: %w", err)
		}
	}

	tree, err := commit.Tree()
	if err != nil {
		return fmt.Errorf("integrity: %w", err)
	}
	m := &modeState{
		repo:     repo,
		repoRoot: root,
		repoName: repoName(repo, root),
		signer:   signer,
		signed:   allowedSigners != "",
		commit:   commit,
		tree:     tree,
	}
	content, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Errorf("integrity: %w", err)
	}
	if err := m.verify(abs, content); err != nil {
		return err
	}
	mode = m
	return nil
}

// isStateOnly reports whether commit has exactly one parent and
// touches only .state/ paths — the shape of a state-save commit.
func isStateOnly(commit *object.Commit) (bool, error) {
	if commit.NumParents() != 1 {
		return false, nil
	}
	parent, err := commit.Parent(0)
	if err != nil {
		return false, err
	}
	curTree, err := commit.Tree()
	if err != nil {
		return false, err
	}
	parentTree, err := parent.Tree()
	if err != nil {
		return false, err
	}
	changes, err := object.DiffTree(parentTree, curTree)
	if err != nil {
		return false, err
	}
	for _, ch := range changes {
		for _, name := range []string{ch.From.Name, ch.To.Name} {
			if name != "" && !strings.HasPrefix(name, stateDir+"/") {
				return false, nil
			}
		}
	}
	return true, nil
}

// verifyHeadSignature enforces the signature rule on the HEAD chain:
// every state-save commit from HEAD down to the release commit under
// them must be SSH-signed by an allowed key, and the release commit
// must be signed itself or via a signed annotated tag pointing at it.
// It returns the release signer's key comment.
func verifyHeadSignature(repo *gogit.Repository, head *object.Commit, allowedSigners string) (string, error) {
	cur := head
	for {
		stateOnly, err := isStateOnly(cur)
		if err != nil {
			return "", err
		}
		if !stateOnly {
			break
		}
		if _, err := libgit.VerifyCommitSSH(cur, allowedSigners); err != nil {
			return "", fmt.Errorf("state commit %s is not signed by an allowed key: %w", cur.Hash, err)
		}
		cur, err = cur.Parent(0)
		if err != nil {
			return "", err
		}
	}
	// cur is the release commit: its own signature, or a signed
	// annotated tag targeting it.
	signer, commitErr := libgit.VerifyCommitSSH(cur, allowedSigners)
	if commitErr == nil {
		return signer, nil
	}
	tags, err := repo.TagObjects()
	if err != nil {
		return "", commitErr
	}
	defer tags.Close()
	var tagErr error
	for {
		tag, err := tags.Next()
		if err != nil {
			break
		}
		if tag.Target != cur.Hash || tag.TargetType != plumbing.CommitObject {
			continue
		}
		s, err := libgit.VerifyTagSSH(tag, allowedSigners)
		if err == nil {
			return s, nil
		}
		tagErr = err
	}
	if tagErr != nil {
		return "", fmt.Errorf("commit %s: %w (and no annotated tag pointing at it is signed by an allowed key: %v)", cur.Hash, commitErr, tagErr)
	}
	return "", fmt.Errorf("commit %s: %w", cur.Hash, commitErr)
}

// VerifyFile checks that absPath lies inside the verified repository
// and that content matches the blob committed at the verified HEAD. It
// is a no-op when integrity mode is off; the command package installs
// it as the require loader's vetting hook.
func VerifyFile(absPath string, content []byte) error {
	if mode == nil {
		return nil
	}
	return mode.verify(absPath, content)
}

func (m *modeState) verify(absPath string, content []byte) error {
	rel, err := filepath.Rel(m.repoRoot, absPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("integrity: %s is outside the verified repository %s", absPath, m.repoRoot)
	}
	f, err := m.tree.File(filepath.ToSlash(rel))
	if err != nil {
		return fmt.Errorf("integrity: %s is not committed at HEAD (%s)", absPath, m.commit.Hash)
	}
	committed, err := f.Contents()
	if err != nil {
		return fmt.Errorf("integrity: %w", err)
	}
	if committed != string(content) {
		return fmt.Errorf("integrity: %s differs from its committed version at HEAD (%s)", absPath, m.commit.Hash)
	}
	return nil
}
