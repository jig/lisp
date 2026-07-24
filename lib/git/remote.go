package git

import (
	"context"
	"errors"
	"fmt"

	gogit "github.com/go-git/go-git/v6"
	gitcfg "github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/client"
	transporthttp "github.com/go-git/go-git/v6/plumbing/transport/http"
	transportssh "github.com/go-git/go-git/v6/plumbing/transport/ssh"
	gossh "golang.org/x/crypto/ssh"

	. "github.com/jig/lisp/types"
)

// clientOpts maps the :auth option to go-git transport client options.
// Supported shapes: the keyword :ssh-agent (user "git"), {:ssh-agent
// true :user u}, {:ssh-key pem :passphrase p :user u :known-hosts path
// :insecure-host-key bool}, {:username u :password p}, {:token t}
// (GitHub/GitLab PATs over basic auth) and {:bearer t} (true Bearer
// servers). Absent :auth means anonymous for HTTP and local paths, while
// for SSH URLs go-git itself falls back to agent auth with the URL's
// user. When no host-key option is given go-git uses the default
// known_hosts files — the secure default.
func clientOpts(o map[MalType]MalType) ([]client.Option, error) {
	v, ok := optGet(o, "auth")
	if !ok || v == nil {
		return nil, nil
	}
	if v == NewKeyword("ssh-agent") {
		auth, err := transportssh.NewSSHAgentAuth(transportssh.DefaultUsername)
		if err != nil {
			return nil, err
		}
		return []client.Option{client.WithSSHAuth(auth)}, nil
	}
	hm, ok := v.(HashMap)
	if !ok {
		return nil, fmt.Errorf(":auth must be :ssh-agent or a map, got %T", v)
	}
	auth := hm.Items
	if optBool(auth, "ssh-agent") {
		user, _, err := optString(auth, "user")
		if err != nil {
			return nil, err
		}
		if user == "" {
			user = transportssh.DefaultUsername
		}
		agentAuth, err := transportssh.NewSSHAgentAuth(user)
		if err != nil {
			return nil, err
		}
		return []client.Option{client.WithSSHAuth(agentAuth)}, nil
	}
	if pem, ok, err := optString(auth, "ssh-key"); err != nil {
		return nil, err
	} else if ok {
		user, _, err := optString(auth, "user")
		if err != nil {
			return nil, err
		}
		if user == "" {
			user = transportssh.DefaultUsername
		}
		passphrase, _, err := optString(auth, "passphrase")
		if err != nil {
			return nil, err
		}
		keys, err := transportssh.NewPublicKeys(user, []byte(pem), passphrase)
		if err != nil {
			return nil, err
		}
		if path, ok, err := optString(auth, "known-hosts"); err != nil {
			return nil, err
		} else if ok {
			cb, err := transportssh.NewKnownHostsCallback(path)
			if err != nil {
				return nil, err
			}
			keys.HostKeyCallback = cb
		} else if optBool(auth, "insecure-host-key") {
			keys.HostKeyCallback = gossh.InsecureIgnoreHostKey()
		}
		return []client.Option{client.WithSSHAuth(keys)}, nil
	}
	if username, ok, err := optString(auth, "username"); err != nil {
		return nil, err
	} else if ok {
		password, _, err := optString(auth, "password")
		if err != nil {
			return nil, err
		}
		return []client.Option{client.WithHTTPAuth(&transporthttp.BasicAuth{Username: username, Password: password})}, nil
	}
	if token, ok, err := optString(auth, "token"); err != nil {
		return nil, err
	} else if ok {
		return []client.Option{client.WithHTTPAuth(&transporthttp.BasicAuth{Username: "x-access-token", Password: token})}, nil
	}
	if bearer, ok, err := optString(auth, "bearer"); err != nil {
		return nil, err
	} else if ok {
		return []client.Option{client.WithHTTPAuth(&transporthttp.TokenAuth{Token: bearer})}, nil
	}
	return nil, fmt.Errorf(":auth map must contain :ssh-key, :ssh-agent, :username, :token or :bearer")
}

// refSpecs converts a :refspecs vector of strings.
func refSpecs(o map[MalType]MalType, name string) ([]gitcfg.RefSpec, error) {
	v, ok := optGet(o, name)
	if !ok || v == nil {
		return nil, nil
	}
	vec, ok := v.(Vector)
	if !ok {
		return nil, fmt.Errorf(":%s must be a vector of strings, got %T", name, v)
	}
	specs := make([]gitcfg.RefSpec, 0, len(vec.Val))
	for _, item := range vec.Val {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf(":%s must be a vector of strings, got %T element", name, item)
		}
		spec := gitcfg.RefSpec(s)
		if err := spec.Validate(); err != nil {
			return nil, fmt.Errorf("invalid refspec %q: %w", s, err)
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

// upToDate maps go-git's already-up-to-date sentinel to a normal value.
func upToDate(err error) (MalType, error) {
	if err == nil {
		return NewKeyword("ok"), nil
	}
	if errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return NewKeyword("up-to-date"), nil
	}
	return nil, err
}

func gitClone(ctx context.Context, url, path string, params ...MalType) (MalType, error) {
	o, err := trailingOpts(params)
	if err != nil {
		return nil, err
	}
	copts, err := clientOpts(o)
	if err != nil {
		return nil, err
	}
	depth, err := optInt(o, "depth", 0)
	if err != nil {
		return nil, err
	}
	cloneOpts := &gogit.CloneOptions{
		URL:           url,
		ClientOptions: copts,
		Depth:         depth,
		SingleBranch:  optBool(o, "single-branch"),
		Bare:          optBool(o, "bare"),
	}
	if branch, ok, err := optString(o, "branch"); err != nil {
		return nil, err
	} else if ok {
		cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(branch)
	}
	repo, err := gogit.PlainCloneContext(ctx, path, cloneOpts)
	if err != nil {
		return nil, err
	}
	r, err := newRepo(repo, path)
	if err != nil {
		return nil, err
	}
	if !cloneOpts.Bare {
		if err := refreshIndex(r); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func gitPush(ctx context.Context, rv MalType, params ...MalType) (MalType, error) {
	r, err := asRepo("git-push", rv)
	if err != nil {
		return nil, err
	}
	o, err := trailingOpts(params)
	if err != nil {
		return nil, err
	}
	copts, err := clientOpts(o)
	if err != nil {
		return nil, err
	}
	remote, _, err := optString(o, "remote")
	if err != nil {
		return nil, err
	}
	specs, err := refSpecs(o, "refspecs")
	if err != nil {
		return nil, err
	}
	return upToDate(r.repo.PushContext(ctx, &gogit.PushOptions{
		RemoteName:    remote,
		RefSpecs:      specs,
		ClientOptions: copts,
		Force:         optBool(o, "force"),
		Prune:         optBool(o, "prune"),
		FollowTags:    optBool(o, "follow-tags"),
	}))
}

func gitPull(ctx context.Context, rv MalType, params ...MalType) (MalType, error) {
	r, err := asRepo("git-pull", rv)
	if err != nil {
		return nil, err
	}
	o, err := trailingOpts(params)
	if err != nil {
		return nil, err
	}
	copts, err := clientOpts(o)
	if err != nil {
		return nil, err
	}
	remote, _, err := optString(o, "remote")
	if err != nil {
		return nil, err
	}
	depth, err := optInt(o, "depth", 0)
	if err != nil {
		return nil, err
	}
	pullOpts := &gogit.PullOptions{
		RemoteName:    remote,
		ClientOptions: copts,
		Depth:         depth,
		Force:         optBool(o, "force"),
	}
	if branch, ok, err := optString(o, "branch"); err != nil {
		return nil, err
	} else if ok {
		pullOpts.ReferenceName = plumbing.NewBranchReferenceName(branch)
	}
	wt, err := worktree(r)
	if err != nil {
		return nil, err
	}
	result, err := upToDate(wt.PullContext(ctx, pullOpts))
	if err != nil {
		return nil, err
	}
	return result, refreshIndex(r)
}

func gitFetch(ctx context.Context, rv MalType, params ...MalType) (MalType, error) {
	r, err := asRepo("git-fetch", rv)
	if err != nil {
		return nil, err
	}
	o, err := trailingOpts(params)
	if err != nil {
		return nil, err
	}
	copts, err := clientOpts(o)
	if err != nil {
		return nil, err
	}
	remote, _, err := optString(o, "remote")
	if err != nil {
		return nil, err
	}
	specs, err := refSpecs(o, "refspecs")
	if err != nil {
		return nil, err
	}
	depth, err := optInt(o, "depth", 0)
	if err != nil {
		return nil, err
	}
	return upToDate(r.repo.FetchContext(ctx, &gogit.FetchOptions{
		RemoteName:    remote,
		RefSpecs:      specs,
		ClientOptions: copts,
		Depth:         depth,
		Prune:         optBool(o, "prune"),
		Force:         optBool(o, "force"),
	}))
}
