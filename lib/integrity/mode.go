package integrity

// This file implements integrity mode (`lisp --integrity <ref>`): the
// interpreter runs a script if and only if it matches what is committed
// in its Git repository at <ref>.
//
// Enable pins <ref> (a commit hash, tag or branch), requires HEAD to be
// exactly that commit, and byte-compares the script against the blob
// committed at it. The command package then installs VerifyFile as the
// require loader's vetting hook, so the check cascades: every module a
// verified script requires must live in the same repository and match
// its committed blob too; anything outside the repository is refused.
// With an allowed-signers file, Enable additionally requires <ref> to
// carry an SSH signature by one of the listed keys (the tag's signature
// for annotated tags, the commit's otherwise), turning the guarantee
// from "matches the local repository" into "matches what a trusted key
// released".
//
// This is an operational assurance for the operator launching the
// script — no accidental drift, no uncommitted edits — not a security
// boundary against an attacker who can already write to the repository
// or replace the binary. Code the verified script chooses to load
// outside the require mechanism (load-file, slurp+eval) is not covered.

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

// modeState is the pinned verification context of an --integrity run.
type modeState struct {
	repo     *gogit.Repository
	repoRoot string
	ref      string
	signer   string
	commit   *object.Commit
	tree     *object.Tree
}

// mode is nil when the interpreter does not run under --integrity. It
// is written once by Enable before any lisp code is evaluated and only
// read afterwards.
var mode *modeState

// Active reports whether the interpreter runs under --integrity.
func Active() bool { return mode != nil }

// CommitHash returns the verified commit hash, or "" when inactive.
func CommitHash() string {
	if mode == nil {
		return ""
	}
	return mode.commit.Hash.String()
}

// Ref returns the ref --integrity was given, or "" when inactive.
func Ref() string {
	if mode == nil {
		return ""
	}
	return mode.ref
}

// Signer returns the comment of the allowed key that signed the
// verified ref, or "" when inactive or no signers were required.
func Signer() string {
	if mode == nil {
		return ""
	}
	return mode.signer
}

// Disable turns integrity mode off. Exported for tests.
func Disable() { mode = nil }

// Enable verifies scriptPath against ref in its enclosing Git
// repository and turns integrity mode on. allowedSigners, when
// non-empty, holds authorized_keys-format public keys and makes Enable
// additionally require ref to be SSH-signed by one of them.
func Enable(scriptPath, ref, allowedSigners string) error {
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

	hash, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return fmt.Errorf("integrity: cannot resolve %q: %w", ref, err)
	}
	// An annotated tag resolves to the tag object; peel it to its
	// target commit and keep it for the signature check.
	tag, err := repo.TagObject(*hash)
	commitHash := *hash
	if err == nil {
		commitHash = tag.Target
	} else {
		tag = nil
	}
	commit, err := repo.CommitObject(commitHash)
	if err != nil {
		return fmt.Errorf("integrity: %q does not resolve to a commit: %w", ref, err)
	}

	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("integrity: %w", err)
	}
	if head.Hash() != commit.Hash {
		// HEAD may sit above the ref: state-save commits its writes,
		// moving HEAD, and the code ref stays valid across restarts as
		// long as every commit in between touches only .state/.
		if err := verifyStateOnlyDescent(repo, head.Hash(), commit.Hash); err != nil {
			return fmt.Errorf("integrity: HEAD is at %s, not at %q (%s): %w", head.Hash(), ref, commit.Hash, err)
		}
	}

	signer := ""
	if allowedSigners != "" {
		signer, err = verifyRefSignature(repo, ref, tag, commit, allowedSigners)
		if err != nil {
			return fmt.Errorf("integrity: %w", err)
		}
	}

	tree, err := commit.Tree()
	if err != nil {
		return fmt.Errorf("integrity: %w", err)
	}
	m := &modeState{repo: repo, repoRoot: root, ref: ref, signer: signer, commit: commit, tree: tree}
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

// verifyRefSignature requires ref to be SSH-signed by an allowed key:
// the tag signature when ref is an annotated tag, the commit signature
// otherwise (plain commits, branches and lightweight tags). It returns
// the signing key's comment.
func verifyRefSignature(repo *gogit.Repository, ref string, tag *object.Tag, commit *object.Commit, allowedSigners string) (string, error) {
	if tag == nil {
		// ResolveRevision may already have peeled an annotated tag
		// named ref to its commit; look the tag object up explicitly
		// so its signature — not the commit's — is what gets checked.
		if tref, err := repo.Reference(plumbing.NewTagReferenceName(ref), true); err == nil {
			if t, err := repo.TagObject(tref.Hash()); err == nil {
				tag = t
			}
		}
	}
	if tag != nil {
		return libgit.VerifyTagSSH(tag, allowedSigners)
	}
	return libgit.VerifyCommitSSH(commit, allowedSigners)
}

// verifyStateOnlyDescent checks that ref is an ancestor of head through
// a linear chain of commits that touch only .state/ paths — the commits
// state-save creates. Any other divergence is an error.
func verifyStateOnlyDescent(repo *gogit.Repository, head, ref plumbing.Hash) error {
	cur, err := repo.CommitObject(head)
	if err != nil {
		return err
	}
	for cur.Hash != ref {
		if cur.NumParents() != 1 {
			return fmt.Errorf("commit %s is not part of a linear state-only descent from the ref", cur.Hash)
		}
		parent, err := cur.Parent(0)
		if err != nil {
			return err
		}
		curTree, err := cur.Tree()
		if err != nil {
			return err
		}
		parentTree, err := parent.Tree()
		if err != nil {
			return err
		}
		changes, err := object.DiffTree(parentTree, curTree)
		if err != nil {
			return err
		}
		for _, ch := range changes {
			for _, name := range []string{ch.From.Name, ch.To.Name} {
				if name != "" && !strings.HasPrefix(name, stateDir+"/") {
					return fmt.Errorf("commit %s modifies %s outside %s/", cur.Hash, name, stateDir)
				}
			}
		}
		cur = parent
	}
	return nil
}

// VerifyFile checks that absPath lies inside the verified repository
// and that content matches the blob committed at the pinned ref. It is
// a no-op when integrity mode is off; the command package installs it
// as the require loader's vetting hook.
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
		return fmt.Errorf("integrity: %s is not committed at %q (%s)", absPath, m.ref, m.commit.Hash)
	}
	committed, err := f.Contents()
	if err != nil {
		return fmt.Errorf("integrity: %w", err)
	}
	if committed != string(content) {
		return fmt.Errorf("integrity: %s differs from its committed version at %q (%s)", absPath, m.ref, m.commit.Hash)
	}
	return nil
}
