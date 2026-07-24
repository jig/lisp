// Package git exposes go-git to jig/lisp as a set of builtins covering
// local repository operations (init/open, add, commit, log, status,
// branches, tags) and remote ones (clone, push, pull, fetch), with
// first-class support for SSH-signed commits and tags (ed25519).
//
// Signatures use the SSH signature format (what git produces with
// gpg.format=ssh), so commits signed here verify with `git verify-commit`
// and show as "Verified" on the usual forges, and vice versa. Both sha1
// and sha256 object-format repositories are supported; signing places the
// signature in the header git expects for the repo's format (gpgsig or
// gpgsig-sha256). Options are passed as a trailing hashmap and results
// come back as maps and vectors of plain lisp values.
package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-billy/v6/osfs"
	gogit "github.com/go-git/go-git/v6"
	gitcfg "github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	formatcfg "github.com/go-git/go-git/v6/plumbing/format/config"
	"github.com/go-git/go-git/v6/plumbing/format/gitignore"
	"github.com/go-git/go-git/v6/plumbing/object"

	_ "embed"

	"github.com/jig/lisp/internal/gogitutil"
	"github.com/jig/lisp/lib/call"
	. "github.com/jig/lisp/types"
)

//go:embed header-git.lisp
var headerGit string

// HeaderGit returns the lisp-defined part of the namespace (the with-repo
// macro and the verified? helpers), loaded by nsgit.
func HeaderGit() string { return headerGit }

// Repo is an open repository handle. It remembers the repository's object
// format so signatures land in the header git expects (gpgsig for sha1,
// gpgsig-sha256 for sha256).
type Repo struct {
	repo   *gogit.Repository
	path   string
	sha256 bool
}

func (r *Repo) LispPrint(_ func(MalType, bool) string) string { return "«git-repo " + r.path + "»" }

// asRepo unwraps a repo handle, with a typed error naming the builtin.
func asRepo(name string, v MalType) (*Repo, error) {
	r, ok := v.(*Repo)
	if !ok {
		return nil, fmt.Errorf("%s: expected a git repo handle, got %T", name, v)
	}
	return r, nil
}

// Load registers the git builtins in env.
func Load(env EnvType) {
	call.CallOverrideFN(env, "git-init", gitInit, 1, 2)
	call.CallOverrideFN(env, "git-open", gitOpen)
	call.CallOverrideFN(env, "git-clone", gitClone, 3, 4)
	call.CallOverrideFN(env, "git-close", gitClose)
	call.CallOverrideFN(env, "git-add", gitAdd, 2, 3)
	call.CallOverrideFN(env, "git-commit", gitCommit, 2, 3)
	call.CallOverrideFN(env, "git-log", gitLog, 1, 2)
	call.CallOverrideFN(env, "git-show", gitShow)
	call.CallOverrideFN(env, "git-status", gitStatus)
	call.CallOverrideFN(env, "git-head", gitHead)
	call.CallOverrideFN(env, "git-branch", gitBranch, 2, 3)
	call.CallOverrideFN(env, "git-branches", gitBranches)
	call.CallOverrideFN(env, "git-checkout", gitCheckout, 2, 3)
	call.CallOverrideFN(env, "git-tag", gitTag, 2, 3)
	call.CallOverrideFN(env, "git-tags", gitTags)
	call.CallOverrideFN(env, "git-remote-add", gitRemoteAdd)
	call.CallOverrideFN(env, "git-remotes", gitRemotes)
	call.CallOverrideFN(env, "git-push", gitPush, 2, 3)
	call.CallOverrideFN(env, "git-pull", gitPull, 2, 3)
	call.CallOverrideFN(env, "git-fetch", gitFetch, 2, 3)
	call.CallOverrideFN(env, "git-verify-commit", gitVerifyCommit)
	call.CallOverrideFN(env, "git-verify-tag", gitVerifyTag)

	call.Doc(env, "git-init", "[path & {:bare :object-format}]",
		"Creates a repository at path and returns a handle; :object-format \"sha256\" for a SHA-256 repo.")
	call.Doc(env, "git-open", "[path]",
		"Opens an existing repository and returns a handle.")
	call.Doc(env, "git-clone", "[url path & {:auth :branch :depth :single-branch :bare}]",
		"Clones url into path and returns a handle; see git-push for the :auth map.")
	call.Doc(env, "git-close", "[repo]",
		"Closes a repository handle.")
	call.Doc(env, "git-add", "[repo path & {:all :glob}]",
		"Stages path (\".\" for everything); :all true stages all modified/deleted files, :glob true treats path as a glob pattern.")
	call.Doc(env, "git-commit", "[repo msg & {:author {:name :email} :committer :all :allow-empty :amend}]",
		"Commits staged changes and returns the commit map. Under --integrity-keys the commit is SSH-signed with the ssh-agent key listed there; otherwise it is unsigned (there is no per-call key option).")
	call.Doc(env, "git-log", "[repo & {:max :from :all :path}]",
		"Returns a vector of commit maps from HEAD (or :from rev), newest first.")
	call.Doc(env, "git-show", "[repo rev]",
		"Returns the commit map for rev (hash, \"HEAD\", branch or tag name).")
	call.Doc(env, "git-status", "[repo]",
		"Returns {:clean bool :files {path {:staging kw :worktree kw}}} for the worktree.")
	call.Doc(env, "git-head", "[repo]",
		"Returns {:name :branch :hash} for HEAD.")
	call.Doc(env, "git-branch", "[repo name & {:checkout :at}]",
		"Creates branch name at HEAD (or :at rev); :checkout true switches to it.")
	call.Doc(env, "git-branches", "[repo]",
		"Returns a vector of {:name :hash :head} for local branches.")
	call.Doc(env, "git-checkout", "[repo ref & {:create :force}]",
		"Checks out a branch, tag or revision; :create true creates the branch first.")
	call.Doc(env, "git-tag", "[repo name & {:at :message :tagger {:name :email}}]",
		"Creates a tag at HEAD (or :at rev); :message makes it annotated. Under --integrity-keys an annotated tag is SSH-signed with the ssh-agent key listed there.")
	call.Doc(env, "git-tags", "[repo]",
		"Returns a vector of {:name :hash :target :annotated} for all tags.")
	call.Doc(env, "git-remote-add", "[repo name url]",
		"Adds a remote.")
	call.Doc(env, "git-remotes", "[repo]",
		"Returns a vector of {:name :urls} for the configured remotes.")
	call.Doc(env, "git-push", "[repo & {:auth :remote :refspecs :force :prune :follow-tags}]",
		"Pushes to :remote (default origin); returns :ok or :up-to-date. :auth is :ssh-agent, {:ssh-key pem :passphrase p :user u :known-hosts path :insecure-host-key bool}, {:username u :password p}, {:token t} or {:bearer t}.")
	call.Doc(env, "git-pull", "[repo & {:auth :remote :branch :depth :force}]",
		"Pulls into the current branch; returns :ok or :up-to-date.")
	call.Doc(env, "git-fetch", "[repo & {:auth :remote :refspecs :depth :prune :force}]",
		"Fetches from :remote (default origin); returns :ok or :up-to-date.")
	call.Doc(env, "git-verify-commit", "[repo rev allowed-keys]",
		"Verifies the SSH signature of rev against allowed-keys (authorized_keys-format lines); returns {:valid true :key-type :fingerprint :hash-algorithm :signer} or throws.")
	call.Doc(env, "git-verify-tag", "[repo name allowed-keys]",
		"Verifies the SSH signature of annotated tag name; same contract as git-verify-commit.")
}

// globalIgnore loads the system and user-global gitignore patterns once.
// go-git only honors core.excludesfile declared in ~/.gitconfig; git's
// XDG default ($XDG_CONFIG_HOME/git/ignore, usually ~/.config/git/ignore,
// used when core.excludesFile is unset) is loaded here explicitly so
// git-status agrees with git about what is ignored.
var globalIgnore = sync.OnceValue(func() []gitignore.Pattern {
	fs := osfs.New("/")
	var ps []gitignore.Pattern
	if p, err := gitignore.LoadSystemPatterns(fs); err == nil {
		ps = append(ps, p...)
	}
	if p, err := gitignore.LoadGlobalPatterns(fs); err == nil && len(p) > 0 {
		return append(ps, p...)
	}
	cfgDir := os.Getenv("XDG_CONFIG_HOME")
	if cfgDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ps
		}
		cfgDir = filepath.Join(home, ".config")
	}
	data, err := os.ReadFile(filepath.Join(cfgDir, "git", "ignore"))
	if err != nil {
		return ps
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ps = append(ps, gitignore.ParsePattern(line, nil))
	}
	return ps
})

// worktree returns the repo's worktree with the global ignore patterns
// attached, so ignore handling matches the git CLI.
func worktree(r *Repo) (*gogit.Worktree, error) {
	wt, err := r.repo.Worktree()
	if err != nil {
		return nil, err
	}
	wt.Excludes = globalIgnore()
	return wt, nil
}

// refreshIndex rewrites the index so its on-disk timestamp becomes
// strictly newer than the worktree files. go-git trusts index metadata
// only in that case (the racy-git rule) and otherwise re-hashes worktree
// files — with a hardcoded sha1 hasher (alpha.4,
// utils/merkletrie/filesystem/node.go), so in sha256 repositories every
// freshly written file is spuriously reported as modified, which breaks
// status and pull. Rewriting the index after operations that touch both
// the worktree and the index restores metadata trust. sha1 repositories
// re-hash correctly and need no refresh.
func refreshIndex(r *Repo) error {
	if !r.sha256 {
		return nil
	}
	idx, err := r.repo.Storer.Index()
	if err != nil {
		return err
	}
	return r.repo.Storer.SetIndex(idx)
}

func newRepo(repo *gogit.Repository, path string) (*Repo, error) {
	cfg, err := repo.Config()
	if err != nil {
		return nil, err
	}
	return &Repo{
		repo:   repo,
		path:   path,
		sha256: cfg.Extensions.ObjectFormat == formatcfg.SHA256,
	}, nil
}

func gitInit(path string, params ...MalType) (MalType, error) {
	o, err := trailingOpts(params)
	if err != nil {
		return nil, err
	}
	var initOpts []gogit.InitOption
	if s, ok, err := optString(o, "object-format"); err != nil {
		return nil, err
	} else if ok {
		switch s {
		case "sha1":
			initOpts = append(initOpts, gogit.WithObjectFormat(formatcfg.SHA1))
		case "sha256":
			initOpts = append(initOpts, gogit.WithObjectFormat(formatcfg.SHA256))
		default:
			return nil, fmt.Errorf("git-init: :object-format must be \"sha1\" or \"sha256\", got %q", s)
		}
	}
	repo, err := gogit.PlainInit(path, optBool(o, "bare"), initOpts...)
	if err != nil {
		return nil, err
	}
	return newRepo(repo, path)
}

func gitOpen(path string) (MalType, error) {
	repo, err := gogit.PlainOpen(path)
	if err != nil {
		return nil, err
	}
	return newRepo(repo, path)
}

func gitClose(rv MalType) (MalType, error) {
	r, err := asRepo("git-close", rv)
	if err != nil {
		return nil, err
	}
	return nil, r.repo.Close()
}

func gitAdd(rv MalType, path string, params ...MalType) (MalType, error) {
	r, err := asRepo("git-add", rv)
	if err != nil {
		return nil, err
	}
	o, err := trailingOpts(params)
	if err != nil {
		return nil, err
	}
	wt, err := worktree(r)
	if err != nil {
		return nil, err
	}
	switch {
	case optBool(o, "glob"):
		err = wt.AddGlob(path)
	case optBool(o, "all"):
		err = wt.AddWithOptions(&gogit.AddOptions{All: true})
	default:
		_, err = wt.Add(path)
	}
	if err != nil {
		return nil, err
	}
	return nil, refreshIndex(r)
}

func gitCommit(rv MalType, msg string, params ...MalType) (MalType, error) {
	r, err := asRepo("git-commit", rv)
	if err != nil {
		return nil, err
	}
	o, err := trailingOpts(params)
	if err != nil {
		return nil, err
	}
	commitOpts := &gogit.CommitOptions{
		All:               optBool(o, "all"),
		AllowEmptyCommits: optBool(o, "allow-empty"),
		Amend:             optBool(o, "amend"),
		Signer:            gogitutil.NoSign{},
	}
	if commitOpts.Author, err = optSignature(o, "author"); err != nil {
		return nil, err
	}
	if commitOpts.Committer, err = optSignature(o, "committer"); err != nil {
		return nil, err
	}
	wt, err := worktree(r)
	if err != nil {
		return nil, err
	}
	hash, err := wt.Commit(msg, commitOpts)
	if err != nil {
		return nil, err
	}
	// Signing is driven by the installed policy (--integrity-keys, via
	// the ssh-agent), never a per-call key. It is done after the fact
	// (see resignCommit) so the signature lands in the header matching
	// the repo's object format; go-git's own Signer always writes gpgsig.
	if hash, err = signCommitIfPolicy(r, hash); err != nil {
		return nil, err
	}
	c, err := r.repo.CommitObject(hash)
	if err != nil {
		return nil, err
	}
	if err := refreshIndex(r); err != nil {
		return nil, err
	}
	return commitMap(c), nil
}

func gitLog(rv MalType, params ...MalType) (MalType, error) {
	r, err := asRepo("git-log", rv)
	if err != nil {
		return nil, err
	}
	o, err := trailingOpts(params)
	if err != nil {
		return nil, err
	}
	logOpts := &gogit.LogOptions{
		All:   optBool(o, "all"),
		Order: gogit.LogOrderCommitterTime,
	}
	if rev, ok, err := optString(o, "from"); err != nil {
		return nil, err
	} else if ok {
		hash, err := resolve(r, rev)
		if err != nil {
			return nil, err
		}
		logOpts.From = *hash
	}
	if path, ok, err := optString(o, "path"); err != nil {
		return nil, err
	} else if ok {
		logOpts.PathFilter = func(p string) bool {
			return p == path || strings.HasPrefix(p, path+"/")
		}
	}
	max, err := optInt(o, "max", -1)
	if err != nil {
		return nil, err
	}
	iter, err := r.repo.Log(logOpts)
	if err != nil {
		return nil, err
	}
	defer iter.Close()
	out := []MalType{}
	for max < 0 || len(out) < max {
		c, err := iter.Next()
		if err != nil {
			break
		}
		out = append(out, commitMap(c))
	}
	return Vector{Val: out}, nil
}

func gitShow(rv MalType, rev string) (MalType, error) {
	r, err := asRepo("git-show", rv)
	if err != nil {
		return nil, err
	}
	c, err := commitAt(r, rev)
	if err != nil {
		return nil, err
	}
	return commitMap(c), nil
}

func gitStatus(rv MalType) (MalType, error) {
	r, err := asRepo("git-status", rv)
	if err != nil {
		return nil, err
	}
	wt, err := worktree(r)
	if err != nil {
		return nil, err
	}
	status, err := wt.Status()
	if err != nil {
		return nil, err
	}
	files := HashMap{Items: map[MalType]MalType{}}
	for path, fs := range status {
		files.Items[path] = HashMap{Items: map[MalType]MalType{
			NewKeyword("staging"):  statusKeyword(fs.Staging),
			NewKeyword("worktree"): statusKeyword(fs.Worktree),
		}}
	}
	return HashMap{Items: map[MalType]MalType{
		NewKeyword("clean"): status.IsClean(),
		NewKeyword("files"): files,
	}}, nil
}

func gitHead(rv MalType) (MalType, error) {
	r, err := asRepo("git-head", rv)
	if err != nil {
		return nil, err
	}
	head, err := r.repo.Head()
	if err != nil {
		return nil, err
	}
	return HashMap{Items: map[MalType]MalType{
		NewKeyword("name"):   head.Name().String(),
		NewKeyword("branch"): head.Name().Short(),
		NewKeyword("hash"):   head.Hash().String(),
	}}, nil
}

func gitBranch(rv MalType, name string, params ...MalType) (MalType, error) {
	r, err := asRepo("git-branch", rv)
	if err != nil {
		return nil, err
	}
	o, err := trailingOpts(params)
	if err != nil {
		return nil, err
	}
	at := "HEAD"
	if rev, ok, err := optString(o, "at"); err != nil {
		return nil, err
	} else if ok {
		at = rev
	}
	hash, err := resolve(r, at)
	if err != nil {
		return nil, err
	}
	ref := plumbing.NewHashReference(plumbing.NewBranchReferenceName(name), *hash)
	if err := r.repo.Storer.SetReference(ref); err != nil {
		return nil, err
	}
	if optBool(o, "checkout") {
		wt, err := worktree(r)
		if err != nil {
			return nil, err
		}
		if err := wt.Checkout(&gogit.CheckoutOptions{Branch: ref.Name()}); err != nil {
			return nil, err
		}
		return nil, refreshIndex(r)
	}
	return nil, nil
}

func gitBranches(rv MalType) (MalType, error) {
	r, err := asRepo("git-branches", rv)
	if err != nil {
		return nil, err
	}
	head, err := r.repo.Head()
	if err != nil {
		head = nil // empty repo or detached HEAD resolution failure
	}
	iter, err := r.repo.Branches()
	if err != nil {
		return nil, err
	}
	defer iter.Close()
	out := []MalType{}
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		out = append(out, HashMap{Items: map[MalType]MalType{
			NewKeyword("name"): ref.Name().Short(),
			NewKeyword("hash"): ref.Hash().String(),
			NewKeyword("head"): head != nil && ref.Name() == head.Name(),
		}})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return Vector{Val: out}, nil
}

func gitCheckout(rv MalType, ref string, params ...MalType) (MalType, error) {
	r, err := asRepo("git-checkout", rv)
	if err != nil {
		return nil, err
	}
	o, err := trailingOpts(params)
	if err != nil {
		return nil, err
	}
	wt, err := worktree(r)
	if err != nil {
		return nil, err
	}
	checkoutOpts := &gogit.CheckoutOptions{
		Create: optBool(o, "create"),
		Force:  optBool(o, "force"),
	}
	branchRef := plumbing.NewBranchReferenceName(ref)
	if _, err := r.repo.Reference(branchRef, false); err == nil || checkoutOpts.Create {
		checkoutOpts.Branch = branchRef
	} else {
		hash, err := resolve(r, ref)
		if err != nil {
			return nil, err
		}
		checkoutOpts.Hash = *hash // tag or revision: detached HEAD
	}
	if err := wt.Checkout(checkoutOpts); err != nil {
		return nil, err
	}
	return nil, refreshIndex(r)
}

func gitRemoteAdd(rv MalType, name, url string) (MalType, error) {
	r, err := asRepo("git-remote-add", rv)
	if err != nil {
		return nil, err
	}
	_, err = r.repo.CreateRemote(&gitcfg.RemoteConfig{Name: name, URLs: []string{url}})
	return nil, err
}

func gitRemotes(rv MalType) (MalType, error) {
	r, err := asRepo("git-remotes", rv)
	if err != nil {
		return nil, err
	}
	remotes, err := r.repo.Remotes()
	if err != nil {
		return nil, err
	}
	out := []MalType{}
	for _, remote := range remotes {
		urls := []MalType{}
		for _, u := range remote.Config().URLs {
			urls = append(urls, u)
		}
		out = append(out, HashMap{Items: map[MalType]MalType{
			NewKeyword("name"): remote.Config().Name,
			NewKeyword("urls"): Vector{Val: urls},
		}})
	}
	return Vector{Val: out}, nil
}

func gitTag(rv MalType, name string, params ...MalType) (MalType, error) {
	r, err := asRepo("git-tag", rv)
	if err != nil {
		return nil, err
	}
	o, err := trailingOpts(params)
	if err != nil {
		return nil, err
	}
	at := "HEAD"
	if rev, ok, err := optString(o, "at"); err != nil {
		return nil, err
	} else if ok {
		at = rev
	}
	hash, err := resolve(r, at)
	if err != nil {
		return nil, err
	}
	message, annotated, err := optString(o, "message")
	if err != nil {
		return nil, err
	}
	var tagOpts *gogit.CreateTagOptions
	if annotated {
		tagOpts = &gogit.CreateTagOptions{Message: message, Signer: gogitutil.NoSign{}}
		if tagOpts.Tagger, err = optSignature(o, "tagger"); err != nil {
			return nil, err
		}
	}
	ref, err := r.repo.CreateTag(name, *hash, tagOpts)
	if err != nil {
		return nil, err
	}
	// Only annotated tags carry a signature; a lightweight tag is just a
	// ref and is left unsigned even when a signing policy is active.
	if annotated {
		if ref, err = signTagIfPolicy(r, ref); err != nil {
			return nil, err
		}
	}
	return HashMap{Items: map[MalType]MalType{
		NewKeyword("name"):      name,
		NewKeyword("hash"):      ref.Hash().String(),
		NewKeyword("target"):    hash.String(),
		NewKeyword("annotated"): annotated,
	}}, nil
}

func gitTags(rv MalType) (MalType, error) {
	r, err := asRepo("git-tags", rv)
	if err != nil {
		return nil, err
	}
	iter, err := r.repo.Tags()
	if err != nil {
		return nil, err
	}
	defer iter.Close()
	out := []MalType{}
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		target := ref.Hash()
		annotated := false
		if tag, err := r.repo.TagObject(ref.Hash()); err == nil {
			target = tag.Target
			annotated = true
		}
		out = append(out, HashMap{Items: map[MalType]MalType{
			NewKeyword("name"):      ref.Name().Short(),
			NewKeyword("hash"):      ref.Hash().String(),
			NewKeyword("target"):    target.String(),
			NewKeyword("annotated"): annotated,
		}})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return Vector{Val: out}, nil
}

// resolve turns a revision string (hash, HEAD, branch, tag, ...) into a hash.
func resolve(r *Repo, rev string) (*plumbing.Hash, error) {
	return r.repo.ResolveRevision(plumbing.Revision(rev))
}

// commitAt resolves rev and loads its commit object.
func commitAt(r *Repo, rev string) (*object.Commit, error) {
	hash, err := resolve(r, rev)
	if err != nil {
		return nil, err
	}
	return r.repo.CommitObject(*hash)
}

// commitMap converts a commit to its lisp map representation.
func commitMap(c *object.Commit) MalType {
	parents := []MalType{}
	for _, p := range c.ParentHashes {
		parents = append(parents, p.String())
	}
	return HashMap{Items: map[MalType]MalType{
		NewKeyword("hash"):      c.Hash.String(),
		NewKeyword("message"):   c.Message,
		NewKeyword("author"):    signatureMap(c.Author),
		NewKeyword("committer"): signatureMap(c.Committer),
		NewKeyword("parents"):   Vector{Val: parents},
		NewKeyword("signed"):    c.Signature != "" || c.SignatureSHA256 != "",
	}}
}

func signatureMap(s object.Signature) MalType {
	return HashMap{Items: map[MalType]MalType{
		NewKeyword("name"):  s.Name,
		NewKeyword("email"): s.Email,
		NewKeyword("when"):  s.When.Format(time.RFC3339),
	}}
}

func statusKeyword(code gogit.StatusCode) MalType {
	switch code {
	case gogit.Unmodified:
		return NewKeyword("unmodified")
	case gogit.Untracked:
		return NewKeyword("untracked")
	case gogit.Modified:
		return NewKeyword("modified")
	case gogit.Added:
		return NewKeyword("added")
	case gogit.Deleted:
		return NewKeyword("deleted")
	case gogit.Renamed:
		return NewKeyword("renamed")
	case gogit.Copied:
		return NewKeyword("copied")
	case gogit.UpdatedButUnmerged:
		return NewKeyword("unmerged")
	default:
		return NewKeyword("unknown")
	}
}

// trailingOpts extracts the optional trailing options hashmap.
func trailingOpts(params []MalType) (map[MalType]MalType, error) {
	switch len(params) {
	case 0:
		return nil, nil
	case 1:
		hm, ok := params[0].(HashMap)
		if !ok {
			return nil, fmt.Errorf("options must be a map, got %T", params[0])
		}
		return hm.Items, nil
	default:
		return nil, fmt.Errorf("expected a single options map, got %d arguments", len(params))
	}
}

// optGet finds an option, accepting a keyword key (:name, the idiomatic
// form) or a plain string key ("name").
func optGet(o map[MalType]MalType, name string) (MalType, bool) {
	if v, ok := o[NewKeyword(name)]; ok {
		return v, true
	}
	v, ok := o[name]
	return v, ok
}

func optString(o map[MalType]MalType, name string) (string, bool, error) {
	v, ok := optGet(o, name)
	if !ok || v == nil {
		return "", false, nil
	}
	s, ok := v.(string)
	if !ok {
		return "", false, fmt.Errorf(":%s must be a string, got %T", name, v)
	}
	return s, true, nil
}

func optBool(o map[MalType]MalType, name string) bool {
	v, ok := optGet(o, name)
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func optInt(o map[MalType]MalType, name string, def int) (int, error) {
	v, ok := optGet(o, name)
	if !ok || v == nil {
		return def, nil
	}
	n, ok := v.(int)
	if !ok {
		return 0, fmt.Errorf(":%s must be an integer, got %T", name, v)
	}
	return n, nil
}

func optHashMap(o map[MalType]MalType, name string) (map[MalType]MalType, bool, error) {
	v, ok := optGet(o, name)
	if !ok || v == nil {
		return nil, false, nil
	}
	hm, ok := v.(HashMap)
	if !ok {
		return nil, false, fmt.Errorf(":%s must be a map, got %T", name, v)
	}
	return hm.Items, true, nil
}

// optSignature builds an author/committer signature from {:name :email}.
func optSignature(o map[MalType]MalType, name string) (*object.Signature, error) {
	m, ok, err := optHashMap(o, name)
	if err != nil || !ok {
		return nil, err
	}
	sigName, _, err := optString(m, "name")
	if err != nil {
		return nil, err
	}
	email, _, err := optString(m, "email")
	if err != nil {
		return nil, err
	}
	return &object.Signature{Name: sigName, Email: email, When: time.Now()}, nil
}
