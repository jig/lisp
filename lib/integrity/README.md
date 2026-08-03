# integrity

Builtins to attest and verify lisp source text: canonical formatting,
hashing and digital signatures. Typical use: compute a stable digest of
a `.lisp` file (formatting first, so cosmetic layout differences do not
change the digest) and sign or verify it.

The package also implements the interpreter's [integrity
mode](#integrity-mode-lisp-integrity) (the `lisp-integrity` binary).

## Functions

| Function | Returns |
|---|---|
| `(fmt s)` | source `s` in canonical form (as `lisp --fmt`); errors if `s` does not parse |
| `(sha2-256 s)` | SHA2-256 digest of `s`, lowercase hex |
| `(ed25519-generate)` | key pair `{:public "…" :private "…"}`, base64 |
| `(ed25519-sign private s)` | base64 signature of `s` |
| `(ed25519-verify public s signature)` | `true` or `false` |
| `(assert-integrity & [:with-signature])` | the verified commit hash; **throws** unless running under `lisp-integrity` (and, with `:with-signature`, unless the signature rule was applied) |

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

## Integrity mode (lisp-integrity)

`lisp-integrity script.lisp` runs `script.lisp` *if and only if* it
matches what is committed in its enclosing Git repository at `HEAD`,
and attests the run to systemd-journald:

1. the script must byte-match the blob committed at `HEAD` (pin a
   release by checking it out: `git checkout --detach v1.4.2`);
2. in cascade, every file evaluated as code — `require` modules and
   `load-file`/`load-file-once` targets — must resolve inside the same
   repository and byte-match its committed blob; a file resolving
   outside the repository (an `-i` directory elsewhere,
   `~/.config/lisp/`, …) is refused;
3. the run leaves start/end records in the journal (commit, repo,
   trace id, argv, exit code) and every `log-*` record carries the
   same run fields.

`(assert-integrity)` lets committed code demand the mode: it throws
unless the run is verified, and returns the verified commit hash. The
code repository holds code and configuration only; mutable data lives
outside it (see [INTEGRITY.md](../../INTEGRITY.md), the full
specification).

When `/etc/lisp/allowed_signers` exists on the host (authorized_keys /
`.pub` format, one key per line, as `git-verify-commit` — **not** git's
`allowed_signers` format), `HEAD` must additionally be SSH-signed by a
listed key — itself or via a signed annotated tag pointing at it. The
trust anchor then becomes the key list instead of the local repository
state, so verification survives cloning the repository elsewhere.
`(assert-integrity :with-signature)` demands that rule from code.

What integrity mode is — and is not: it is an operational assurance
for the operator launching the script (no accidental drift, no
uncommitted edits, optionally "signed by a trusted key") plus an
append-only audit trail. It is not a security boundary against an
attacker who can rewrite the repository, the signers file or the
binary. `eval` over strings obtained by other means (`slurp`,
network) is not covered, and uncommitted files that are never
interpreted do not affect the check.

## Loading

Go embedders load the namespace with:

```go
import "github.com/jig/lisp/lib/integrity/nsintegrity"

nsintegrity.Load(env)
```

The lisp-level functions have no dependencies beyond `core` (the
example above uses `slurp` and `get` from core); the integrity mode
machinery builds on go-git and `lib/git`'s SSH signature verification.
Both the `lisp` and `lisp-integrity` binaries load the namespace by
default (under plain `lisp` the mode is never active, so
`assert-integrity` always throws there).
