# Integrity mode

`lisp --integrity <ref>` runs a script *if and only if* the code being
executed matches what is committed in its Git repository at `<ref>`.
This document is both the user guide and the specification the
implementation is held to (`lib/integrity/mode.go`, `state.go`;
enforced by `lib/integrity/mode_test.go` and `command/integrity_test.go`).

Runnable mini-examples of every concept below — basic verification,
the require cascade, signed refs, the state store, and what each
failure looks like — live in
[examples-integrity/](./examples-integrity/), with a `demo.sh` that
replays all of them in throwaway repositories.

## Purpose and threat model

The goal is **operational assurance for the operator launching a
script**: what runs is exactly what was committed (and, with
signatures, exactly what a trusted key released) — no accidental
drift, no uncommitted edits, no locally patched copy.

It is **not a security boundary** against an attacker who can already
write to the repository, the signers file or the `lisp` binary. The
interpreter cannot protect itself from whoever controls what it reads;
that separation belongs to the operating system (see
[Deployment](#deployment-hardening-the-assurance-into-a-boundary)).

## The invariant

Definitions:

- **ref** — the argument of `--integrity`: a commit hash, tag or
  branch name, resolved in the repository enclosing the script.
  A commit hash is immutable and therefore the strongest choice; a
  signed tag is equivalent when `--integrity-signers` is used.
- **code file** — any file evaluated as code: the script, every module
  loaded through `require`, and every file loaded through `load-file` /
  `load-file-once` (which read via the `slurp-source` builtin).
- **state path** — any path under `.state/` at the repository root.

At startup (`--integrity <ref>`):

1. The script must lie inside a Git repository; `<ref>` must resolve
   to a commit `C` in it.
2. `HEAD` must be `C`, **or** a descendant of `C` through a linear
   chain of commits each touching only state paths (the commits
   `state-save` creates). Anything else fails.
3. With `--integrity-signers FILE`: the ref must carry an SSH
   signature by one of the public keys in `FILE` — the tag signature
   if the ref is an annotated tag, the commit signature otherwise.
4. The script must byte-match its blob in `C`'s tree.

At runtime, while the mode is active:

5. Every code file, when loaded, must lie inside the verified
   repository and byte-match its blob in `C`'s tree. A code file
   resolving outside the repository (an `-i` include dir elsewhere,
   `~/.config/lisp/`, `/usr/local/share/lisp/`) is refused.
6. Every state file, when read through `state-load`, must byte-match
   its blob at the **current** `HEAD` (the commit the last
   `state-save` created). Missing-but-committed, present-but-
   uncommitted, and differing files all fail closed.

Uncommitted repository files that are never interpreted do not affect
any check. Point 2 is what makes the **same `--integrity <ref>` valid
across restarts** no matter how many state commits have accumulated:
the operator keeps launching with the release ref (or signed tag) and
never needs to chase state-commit hashes.

Concepts 1–2 and 4–6 are demonstrated by
[examples-integrity/01-basic](./examples-integrity/01-basic) and
[02-requires](./examples-integrity/02-requires); concept 3 by
[03-signed](./examples-integrity/03-signed); the state paths of 2 and
6 by [04-state](./examples-integrity/04-state).

## CLI

```bash
lisp --integrity v1.4.2 service.lisp
lisp --integrity 9fceb02d service.lisp
lisp --integrity v1.4.2 --integrity-signers /etc/lisp/release-keys service.lisp
```

- `--integrity REF` — enable the mode. Requires a script file;
  incompatible with `-e`, `--test`, `--fmt`, `--debug`, stdin (`-`)
  and the DAP/LSP server modes.
- `--integrity-signers FILE` — additionally require the ref to be
  SSH-signed by a key listed in FILE. authorized_keys format, one
  public key per line (`ssh-ed25519 AAAA… comment`), the same format
  `git-verify-commit` takes. Requires `--integrity`.

On success one structured JSON line is logged to stderr for the audit
trail: `{"msg":"integrity verified","ref":…,"commit":…,"signer":…}`
(`signer` only when signers were required).

## Builtins

| Builtin | Behaviour |
|---|---|
| `(assert-integrity)` | Throws unless running under `--integrity`; returns the verified commit hash. Committed code uses it to demand the mode — effective as long as operators know the program is supposed to carry it. |
| `(state-save name value)` | Writes `value` as canonical lisp data to `.state/name.lisp` and **commits it in the same operation** (message `state: name`); returns the commit hash. Works with or without the mode; requires a Git repository. |
| `(state-load name)` / `(state-load name default)` | Reads the state back as pure data (READ, never EVAL — state cannot smuggle code). Returns `default`, or throws without one, when the state does not exist. Under the mode, enforces invariant 6. |
| `(slurp-source path)` | `slurp` for files about to be evaluated: identical, plus invariant 5 under the mode. `load-file` builds on it. |

## The state store

`.state/` sits at the repository root, sibling of `.lisp/`. It is the
sanctioned way for a verified program to persist state (a database as
a hash-map, counters, checkpoints) without stepping outside the
integrity envelope:

- **Canonical form** — values are printed readably and passed through
  the formatter, so state files diff cleanly and hash
  deterministically. Values the reader cannot round-trip (live
  handles, functions) are rejected at save time. State is data only.
- **Commit protocol** — write file → `git add` → `git commit`, all
  inside `state-save`. Committed state is therefore always the product
  of a completed save. State commits are authored `state-save
  <state-save@lisp>` and unsigned (v1).
- **Crash recovery** — a save interrupted between write and commit
  leaves the file differing from `HEAD`; the next `state-load` under
  the mode fails closed and the operator resolves it (commit the
  orphan or check it out). There is deliberately no auto-repair.
- **Concurrency** — one writer process per repository (in-process
  saves are serialized; git itself rejects concurrent index writes
  from other processes).
- `slurp` and `spit` remain available for plain data files, but for
  state that must be trustworthy they are **discouraged** in favour of
  `state-load`/`state-save`: they participate in no invariant.

## Signatures and trust anchors

Without `--integrity-signers` the trust anchor is the local repository
state: the mode proves consistency ("matches what is committed here"),
which stops drift but not history rewriting by whoever can write to
the repository.

With `--integrity-signers` the anchor becomes the key list plus the
binary: the ref must be signed by a trusted key, so verification
survives cloning the repository onto other machines and re-tagging by
someone without the key. Keep the signers file outside the repository
and outside the process user's write reach.

## Deployment: hardening the assurance into a boundary

The mode becomes a real boundary only when the OS guarantees the
attacker cannot write to what the interpreter reads:

- repository checkout, signers file and `lisp` binary owned by `root`
  (or a dedicated `deploy` user);
- the process running as an unprivileged user with **no write access**
  to any of the three;
- if `state-save` is used, grant the process user write access to
  `.state/` and `.git` only — or accept that state (unlike code) is
  writable by the process by design;
- binary provenance (signed releases, package manager verification) is
  outside the interpreter's scope but completes the chain.

## Known limitations

- `eval` over strings obtained by other means (`slurp`, network) is
  not covered — the verified code that chooses to do that is
  responsible for it.
- Preamble placeholders (`-P`) inject operator-supplied expressions;
  the operator is the trusted party in this model.
- A self-verifying binary is deliberately **not** attempted: an
  attacker who can replace the binary can also remove the check.

## Future revisions (not implemented)

- **Signed state commits** — sign `state-save` commits with a machine
  or process key (distinct from release keys) and verify membership on
  load, giving state authenticity, not just consistency.
- **Keys baked into the binary** — accept allowed signers via
  `-ldflags -X` at build time, shrinking the trust anchor to the
  binary alone.
- **Data-repository variant** — keep state in a separate repository
  for multi-writer or high-churn scenarios.
