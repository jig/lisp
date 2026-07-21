# integrity

Builtins to attest and verify lisp source text: canonical formatting,
hashing and digital signatures. Typical use: compute a stable digest of
a `.lisp` file (formatting first, so cosmetic layout differences do not
change the digest) and sign or verify it.

The package also implements the interpreter's [integrity
mode](#integrity-mode---integrity) (`lisp --integrity <ref>`).

## Functions

| Function | Returns |
|---|---|
| `(fmt s)` | source `s` in canonical form (as `lisp --fmt`); errors if `s` does not parse |
| `(sha2-256 s)` | SHA2-256 digest of `s`, lowercase hex |
| `(ed25519-generate)` | key pair `{:public "…" :private "…"}`, base64 |
| `(ed25519-sign private s)` | base64 signature of `s` |
| `(ed25519-verify public s signature)` | `true` or `false` |
| `(assert-integrity)` | the verified commit hash; **throws** unless running under `--integrity` |
| `(state-save name value)` | writes `value` as canonical lisp data to `.state/name.lisp` and commits it (SSH-signed with the ssh-agent key under `--integrity-keys`); returns the commit hash |
| `(state-load name & [default])` | the state read back as pure data (READ, never EVAL); `default` (or throws) when absent |

Ed25519 signing is deterministic: the same key and message always yield
the same signature bytes, so signatures are reproducible and
diff-friendly.

## Example

```clojure
(def keys (ed25519-generate))

(def source (fmt (slurp "program.lisp")))
(def digest (sha2-256 source))

(def signature (ed25519-sign (get keys :private) digest))
(ed25519-verify (get keys :public) digest signature) ;; => true
```

## Integrity mode (--integrity)

`lisp --integrity <ref> script.lisp` runs `script.lisp` *if and only
if* it matches what is committed in its enclosing Git repository at
`<ref>` (a commit hash, tag or branch):

1. `HEAD` must be exactly the commit `<ref>` resolves to, or a linear
   chain of `.state/`-only commits above it (the ones `state-save`
   creates), so the same ref stays valid across restarts;
2. the script must byte-match the blob committed at `<ref>`;
3. in cascade, every file evaluated as code — `require` modules and
   `load-file`/`load-file-once` targets — must resolve inside the same
   repository and byte-match its committed blob; a file resolving
   outside the repository (an `-i` directory elsewhere,
   `~/.config/lisp/`, …) is refused.

`(assert-integrity)` lets committed code demand the mode: it throws
unless the run is verified, and returns the verified commit hash.
`state-save`/`state-load` give a verified program a way to persist
state without leaving the integrity envelope (see
[INTEGRITY.md](../../INTEGRITY.md), the full specification).

With `--integrity-keys FILE` the ref must additionally carry an SSH
signature made by one of the public keys in `FILE` (authorized_keys /
`.pub` format, one key per line, as `git-verify-commit` — **not** git's
`allowed_signers` format): the tag signature for annotated tags, the
commit signature otherwise. The trust anchor then becomes the key list
instead of the local repository state, so verification survives cloning
the repository elsewhere.

What integrity mode is — and is not: it is an operational assurance
for the operator launching the script (no accidental drift, no
uncommitted edits, optionally "signed by a trusted key"). It is not a
security boundary against an attacker who can rewrite the repository,
the signers file or the `lisp` binary. `eval` over strings obtained by
other means (`slurp`, network) is not covered, and uncommitted files
that are never interpreted do not affect the check.

## Loading

Go embedders load the namespace with:

```go
import "github.com/jig/lisp/lib/integrity/nsintegrity"

nsintegrity.Load(env)
```

The lisp-level functions have no dependencies beyond `core` (the
example above uses `slurp` and `get` from core); the integrity mode
machinery builds on go-git and `lib/git`'s SSH signature verification.
The `lisp` binary loads the namespace by default.
