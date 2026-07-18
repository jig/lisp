# git

Git operations for jig/lisp, backed by [go-git](https://github.com/go-git/go-git) v6, with first-class SSH-signed commits and tags (ed25519). Signatures are interoperable with `git` itself: commits signed here pass `git verify-commit` (and show as "Verified" on the usual forges), and commits signed with `git commit -S` under `gpg.format=ssh` verify here. Both sha1 and sha256 object-format repositories are supported.

## Loading

The namespace is loaded by the `lisp` binary. Embedders load it with:

```go
import "github.com/jig/lisp/lib/git/nsgit"

nsgit.Load(env)
```

## Quick example

```clojure
(def key (slurp "/home/me/.ssh/id_ed25519"))
(def pub (slurp "/home/me/.ssh/id_ed25519.pub"))

(git/with-repo [r (git/init "/tmp/demo" {:object-format "sha256"})]
  (spit "/tmp/demo/a.txt" "hello\n")
  (git/add r "a.txt")
  (git/commit r "first" {:author {:name "Me" :email "me@example.com"}
                         :sign   {:key key}})
  (git/verify-commit r "HEAD" pub))
;; => {:valid true :key-type "ssh-ed25519" :fingerprint "SHA256:…"
;;     :hash-algorithm "sha512" :signer "me@laptop"}
```

## Signed commits and tags

`:sign {:key pem :passphrase p}` takes an OpenSSH private key as a PEM string (read it with `slurp`; `:passphrase` only for encrypted keys). The signature uses the SSH signature format with namespace `git` and SHA-512, exactly what `ssh-keygen -Y sign` and `git commit -S` produce with `gpg.format=ssh`. Generate a key with:

```
ssh-keygen -t ed25519 -f signing-key
```

In sha256 repositories the commit signature is stored under the `gpgsig-sha256` header, as git expects; tag signatures are appended to the tag body in both formats.

## Verification — fail closed

`git/verify-commit` and `git/verify-tag` take the allowed public keys as a string of authorized_keys-format lines (`ssh-ed25519 AAAA… comment`, one per line; blank lines and `#` comments are skipped — a `.pub` file or an `allowed_signers`-style list both work). They return a result map **only** when a listed key produced a valid signature over the object's exact payload; every other outcome — unsigned object, no matching key, altered content, corrupt signature — throws a catchable error. `git/verified?` and `git/tag-verified?` wrap them when only a boolean is wanted.

## Remotes and authentication

`git/clone`, `git/push`, `git/pull` and `git/fetch` accept an `:auth` option:

| `:auth` | Meaning |
|---|---|
| absent | Anonymous; also for local path remotes |
| `:ssh-agent` | Keys from the running SSH agent |
| `{:ssh-key pem :passphrase p :user u}` | SSH private key (user defaults to `git`) |
| `{:username u :password p}` | HTTP basic auth |
| `{:token t}` | GitHub/GitLab personal access token |
| `{:bearer t}` | HTTP `Authorization: Bearer` token |

SSH host keys are checked against the default `known_hosts` files. Override with `:known-hosts "path"` in the `:ssh-key` map, or disable checking (only for tests) with `:insecure-host-key true`.

## API

| Builtin | Arguments | Returns |
|---|---|---|
| `git/init` | `[path & {:bare :object-format}]` | repo handle; `:object-format "sha256"` for a SHA-256 repo |
| `git/open` | `[path]` | repo handle |
| `git/clone` | `[url path & {:auth :branch :depth :single-branch :bare}]` | repo handle |
| `git/close` | `[repo]` | nil |
| `git/with-repo` | `[[r expr] & body]` | body value; guarantees `git/close` |
| `git/add` | `[repo path & {:all :glob}]` | nil |
| `git/commit` | `[repo msg & {:author :committer :sign :all :allow-empty :amend}]` | commit map |
| `git/log` | `[repo & {:max :from :all :path}]` | vector of commit maps, newest first |
| `git/show` | `[repo rev]` | commit map |
| `git/status` | `[repo]` | `{:clean bool :files {path {:staging kw :worktree kw}}}` |
| `git/head` | `[repo]` | `{:name :branch :hash}` |
| `git/branch` | `[repo name & {:checkout :at}]` | nil |
| `git/branches` | `[repo]` | vector of `{:name :hash :head}` |
| `git/checkout` | `[repo ref & {:create :force}]` | nil |
| `git/tag` | `[repo name & {:at :message :tagger :sign}]` | `{:name :hash :target :annotated}` |
| `git/tags` | `[repo]` | vector of tag maps |
| `git/remote-add` | `[repo name url]` | nil |
| `git/remotes` | `[repo]` | vector of `{:name :urls}` |
| `git/push` | `[repo & {:auth :remote :refspecs :force :prune :follow-tags}]` | `:ok` or `:up-to-date` |
| `git/pull` | `[repo & {:auth :remote :branch :depth :force}]` | `:ok` or `:up-to-date` |
| `git/fetch` | `[repo & {:auth :remote :refspecs :depth :prune :force}]` | `:ok` or `:up-to-date` |
| `git/verify-commit` | `[repo rev allowed-keys]` | `{:valid :key-type :fingerprint :hash-algorithm :signer}` or throws |
| `git/verify-tag` | `[repo name allowed-keys]` | same, for annotated tags |
| `git/verified?` | `[repo rev allowed-keys]` | boolean |
| `git/tag-verified?` | `[repo name allowed-keys]` | boolean |

Commit maps look like:

```clojure
{:hash "…" :message "…"
 :author    {:name "…" :email "…" :when "2026-07-18T10:00:00Z"}
 :committer {:name "…" :email "…" :when "…"}
 :parents ["…"] :signed true}
```

Revisions (`rev`, `:from`, `:at`) accept anything `git rev-parse` style: a hash, `"HEAD"`, a branch or a tag name.

## Limitations (v1)

- go-git v6 is pinned to a pre-release (`v6.0.0-alpha.4`); it is the first version with sha256 support. Signing works around its current signer plumbing (which targets the wrong header in sha256 repos) by signing after commit creation, so a signed commit briefly leaves one unsigned dangling object behind — harmless, and `git fsck` stays clean.
- Only SSH signatures (any key type ssh-keygen supports; ed25519 recommended). No PGP/X.509 signing or verification.
- Dual sha1+sha256 compatibility-mode repositories (both signature headers at once) are not supported.
- go-git needs an author identity: pass `:author {:name … :email …}` (or have `user.name`/`user.email` in the repo or global config).
- No merge/rebase/stash; no submodules.
