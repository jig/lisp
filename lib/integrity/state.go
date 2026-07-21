package integrity

// This file implements the .state/ store: state-save writes a value as
// canonical lisp data under <repo-root>/.state/<name>.lisp and commits
// it in the same operation, so committed state is always the product of
// a completed save; state-load reads it back as pure data (READ, never
// EVAL). Under --integrity, state-load additionally requires the file
// to byte-match its blob at the current HEAD — the commit the last
// state-save created — so out-of-band edits fail closed. The mode's
// startup check accepts these state-only commits above the code ref
// (see verifyStateOnlyDescent), which keeps the original --integrity
// ref valid across restarts.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/jig/lisp/internal/gogitutil"
	libgit "github.com/jig/lisp/lib/git"
	"github.com/jig/lisp/printer"
	"github.com/jig/lisp/reader"
	. "github.com/jig/lisp/types"
)

const stateDir = ".state"

// stateMu serializes state-save operations within the process (git
// itself rejects concurrent index writes from other processes).
var stateMu sync.Mutex

// stateRepo returns the repository and worktree root the state store
// lives in: the verified repository under --integrity, the repository
// enclosing the working directory otherwise.
func stateRepo(fnName string) (*gogit.Repository, string, error) {
	if mode != nil {
		return mode.repo, mode.repoRoot, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", fnName, err)
	}
	repo, err := gogit.PlainOpenWithOptions(wd, &gogit.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, "", fmt.Errorf("%s: not inside a git repository", fnName)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", fnName, err)
	}
	return repo, wt.Filesystem().Root(), nil
}

// statePath validates name (relative slash-separated segments, no
// hidden or dot-dot parts, as require module names) and returns the
// state file's repo-relative slash path.
func statePath(fnName, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("%s: empty state name", fnName)
	}
	if strings.ContainsAny(name, " \\:@") || strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("%s: invalid state name %q", fnName, name)
	}
	for part := range strings.SplitSeq(name, "/") {
		if part == "" || strings.HasPrefix(part, ".") {
			return "", fmt.Errorf("%s: invalid state name %q", fnName, name)
		}
	}
	return stateDir + "/" + name + ".lisp", nil
}

func state_save(name string, value MalType, params ...MalType) (MalType, error) {
	stateMu.Lock()
	defer stateMu.Unlock()

	options := HashMap{Val: map[string]MalType{}}
	switch len(params) {
	case 0:
	case 1:
		var ok bool
		options, ok = params[0].(HashMap)
		if !ok {
			return nil, fmt.Errorf("state-save: options must be a map, got %T", params[0])
		}
	default:
		return nil, fmt.Errorf("state-save: expected one options map, got %d arguments", len(params))
	}
	signer, err := libgit.SSHSignerFromOptions(options)
	if err != nil {
		return nil, fmt.Errorf("state-save: %w", err)
	}

	rel, err := statePath("state-save", name)
	if err != nil {
		return nil, err
	}
	repo, root, err := stateRepo("state-save")
	if err != nil {
		return nil, err
	}

	// Canonical form: Pr_data is the single source of truth for state
	// layout — it prints readable data deterministically (sorted map/set
	// keys) at a stable width, so state files diff cleanly and hash
	// deterministically. The lisp source formatter is not run: on
	// Pr_data output it only appends this trailing newline. Validate that
	// the text round-trips — a value the reader cannot read back (a live
	// handle, a function) fails here.
	canon := []byte(printer.Pr_data(value, 100) + "\n")
	if _, err := reader.Read_str(string(canon), NewCursorFile(rel), nil); err != nil {
		return nil, fmt.Errorf("state-save: value is not serializable lisp data: %w", err)
	}

	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, fmt.Errorf("state-save: %w", err)
	}
	if err := os.WriteFile(abs, canon, 0o644); err != nil {
		return nil, fmt.Errorf("state-save: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("state-save: %w", err)
	}
	if err := wt.AddWithOptions(&gogit.AddOptions{
		Path:       rel,
		SkipStatus: true,
	}); err != nil {
		return nil, fmt.Errorf("state-save: %w", err)
	}
	hash, err := wt.Commit("state: "+name, &gogit.CommitOptions{
		Author: &object.Signature{Name: "state-save", Email: "state-save@lisp", When: time.Now()},
		Signer: gogitutil.NoSign{},
	})
	if err != nil {
		// Deterministic serialization means an unchanged value produces
		// a byte-identical file, so re-saving the same state leaves the
		// worktree clean. That is a no-op, not an error: the state is
		// already committed, so return the existing HEAD commit. (A
		// :sign option is ignored here — there is nothing new to sign;
		// sign when the value actually changes.)
		if errors.Is(err, gogit.ErrEmptyCommit) {
			head, herr := repo.Head()
			if herr != nil {
				return nil, fmt.Errorf("state-save: %w", herr)
			}
			return head.Hash().String(), nil
		}
		return nil, fmt.Errorf("state-save: %w", err)
	}
	if signer != nil {
		hash, err = libgit.SignCommitSSH(repo, hash, signer)
		if err != nil {
			return nil, fmt.Errorf("state-save: sign commit: %w", err)
		}
	}
	return hash.String(), nil
}

func state_load(name string, defaultValue ...MalType) (MalType, error) {
	rel, err := statePath("state-load", name)
	if err != nil {
		return nil, err
	}
	_, root, err := stateRepo("state-load")
	if err != nil {
		return nil, err
	}
	abs := filepath.Join(root, filepath.FromSlash(rel))
	content, err := os.ReadFile(abs)
	missing := errors.Is(err, fs.ErrNotExist)
	if err != nil && !missing {
		return nil, fmt.Errorf("state-load: %w", err)
	}

	if mode != nil {
		committed, atHead, err := headStateBlob(rel)
		if err != nil {
			return nil, fmt.Errorf("state-load: %w", err)
		}
		switch {
		case missing && atHead:
			return nil, fmt.Errorf("state-load: %s is committed at HEAD but missing from the worktree", rel)
		case !missing && !atHead:
			return nil, fmt.Errorf("state-load: %s is not committed at HEAD (interrupted state-save?)", rel)
		case !missing && committed != string(content):
			return nil, fmt.Errorf("state-load: %s differs from its committed version at HEAD", rel)
		}
	}

	if missing {
		if len(defaultValue) == 1 {
			return defaultValue[0], nil
		}
		return nil, fmt.Errorf("state-load: state %q not found (%s)", name, rel)
	}
	return reader.Read_str(string(content), NewCursorFile(abs), nil)
}

// headStateBlob returns the contents of rel at the current HEAD of the
// verified repository, and whether it exists there.
func headStateBlob(rel string) (string, bool, error) {
	head, err := mode.repo.Head()
	if err != nil {
		return "", false, err
	}
	commit, err := mode.repo.CommitObject(head.Hash())
	if err != nil {
		return "", false, err
	}
	tree, err := commit.Tree()
	if err != nil {
		return "", false, err
	}
	f, err := tree.File(rel)
	if err != nil {
		return "", false, nil
	}
	contents, err := f.Contents()
	if err != nil {
		return "", false, err
	}
	return contents, true, nil
}
