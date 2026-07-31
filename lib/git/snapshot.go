package git

// RepoSnapshot captures HEAD and the index so an operation that
// commits before signing (git-commit, state-save) can roll back to the
// pre-operation state when the signing step fails — instead of leaving
// an unsigned commit at HEAD, which an allowed-signers policy would
// then refuse on every subsequent verified run.

import (
	"errors"
	"fmt"

	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	gitindex "github.com/go-git/go-git/v6/plumbing/format/index"
)

// RepoSnapshot is the pre-operation HEAD and index of a repository.
type RepoSnapshot struct {
	head     *plumbing.Reference
	headHash plumbing.Hash
	hasHead  bool
	index    *gitindex.Index
}

// NewRepoSnapshot captures the repository's current HEAD (symbolic,
// detached or unborn) and a copy of its index.
func NewRepoSnapshot(repo *gogit.Repository) (*RepoSnapshot, error) {
	directHead, err := repo.Storer.Reference(plumbing.HEAD)
	if err != nil && !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return nil, err
	}
	resolvedHead, err := repo.Head()
	hasHead := err == nil
	if err != nil && !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return nil, err
	}
	var headHash plumbing.Hash
	if hasHead {
		headHash = resolvedHead.Hash()
	}
	idx, err := repo.Storer.Index()
	if err != nil {
		return nil, err
	}
	return &RepoSnapshot{
		head:     directHead,
		headHash: headHash,
		hasHead:  hasHead,
		index:    cloneIndex(idx),
	}, nil
}

// Restore puts HEAD and the index back to the snapshot. The rolled-back
// commit object stays in the object store, unreachable.
func (s *RepoSnapshot) Restore(repo *gogit.Repository) error {
	var errs []error
	if err := s.restoreHead(repo); err != nil {
		errs = append(errs, fmt.Errorf("HEAD: %w", err))
	}
	if err := repo.Storer.SetIndex(s.index); err != nil {
		errs = append(errs, fmt.Errorf("index: %w", err))
	}
	return errors.Join(errs...)
}

func (s *RepoSnapshot) restoreHead(repo *gogit.Repository) error {
	if s.head == nil {
		return nil
	}
	name := s.head.Name()
	if s.head.Type() == plumbing.SymbolicReference {
		name = s.head.Target()
		if err := repo.Storer.SetReference(s.head); err != nil {
			return err
		}
	}
	if s.hasHead {
		return repo.Storer.SetReference(plumbing.NewHashReference(name, s.headHash))
	}
	return repo.Storer.RemoveReference(name)
}

// cloneIndex copies the index entries; the cached-tree extension is
// dropped rather than shared, so a restore never writes a stale TREE
// extension mutated by the rolled-back operation.
func cloneIndex(idx *gitindex.Index) *gitindex.Index {
	if idx == nil {
		return nil
	}
	clone := *idx
	clone.Cache = nil
	clone.Entries = make([]*gitindex.Entry, len(idx.Entries))
	for i, entry := range idx.Entries {
		if entry == nil {
			continue
		}
		entryClone := *entry
		clone.Entries[i] = &entryClone
	}
	return &clone
}
