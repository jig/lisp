# Integrity mode — adversarial review brief

A guide for reviewers trying to **break** the integrity mode specified
in [INTEGRITY.md](./INTEGRITY.md). That document is the contract; a
finding is anything that violates one of its six numbered invariants
without stepping outside the threat model below.

## Objective and assets

The mode promises the operator launching
`lisp --integrity <ref> script.lisp` that every byte evaluated as code
matches the repository content at `<ref>`, and that state read through
`state-load` matches what a completed `state-save` committed. A
successful attack makes the interpreter **run code (or accept state)
that differs from the committed content while the run is reported as
verified** — including the startup audit line and a successful
`(assert-integrity)`.

## Out of scope

Declared non-goals — findings here are not interesting:

- An attacker who can rewrite the repository/refs, the keys file or
  the `lisp` binary, or who controls the operator's command line
  (including `-P` preamble injection).
- Verified code that *chooses* to evaluate unverified input
  (`slurp` + `eval`, network input): the committed code is trusted.
- Denial of service (making a verified run fail is fail-closed
  behaviour, not a finding — unless it masks a bypass).

In scope, explicitly: an attacker who can write **worktree files**
(not `.git`), craft repository *content* (committed by a careless
release manager), influence the filesystem (symlinks, case folding,
unicode normalization), or run concurrently with the verified process.

## Code map

Everything enforcing the invariants, with what to probe:

| File | Role — suggested attack angles |
|---|---|
| `lib/integrity/mode.go` | `Enable`: ref resolution (`ResolveRevision`), annotated-tag peeling, HEAD rule. `verifyStateOnlyDescent`: linear walk, merge/root commits, rename `From`/`To` names, pathological histories. `modeState.verify`: `filepath.Rel` containment, byte-compare vs `tree.File` — symlinked files or parent directories, case-insensitive filesystems (macOS), NFC/NFD unicode paths, CRLF/`.gitattributes` filters (does go-git apply them to blobs? the worktree does), SHA-1 collision surface on sha1 repos. |
| `lib/integrity/state.go` | `statePath` name validation (unicode tricks, `.lisp` suffix collisions with code files — can a state name shadow a module?). `stateRepo` cwd-based resolution when the mode is off. Commit protocol atomicity; cross-process races against `state-save`; `headStateBlob` freshness. |
| `lib/integrity/integrity.go` | Builtin registration; can a verified script's environment rebind `assert-integrity` / hooks in a way that matters? |
| `command/integrity.go` | Flag validation completeness (mode combinations, `PreParseArgs` double-parse quirks); hook installation (`require.VerifyModule`, `core.VerifySource`) — anything evaluating code without passing through a hook? |
| `command/command.go` + `command/preamble.go` | **The script is read twice**: `Enable` reads and verifies it, `runScript` re-reads it for evaluation — examine the TOCTOU window between the two reads. |
| `lib/require/require.go` | `resolve_require` sanitization vs `VerifyFile` containment (defence in depth — do they agree?); module cache keyed by abs path. |
| `lib/core/core.go` (`slurp_source`) + `lib/core/header-load-file.lisp` | `load-file` funnels through `slurp-source`; is there any other code-evaluating path (`read-program` callers, `eval` helpers) that skips it? Same double-read question as the script. |
| `lib/git/sign.go` | `matchAllowedKey` authorized_keys parsing; `verifySignature` (sshsig namespace, hash algorithm downgrade); `Signature` vs `SignatureSHA256` field selection per object format. |

## Harness — demonstrate, don't speculate

```bash
go build -o /tmp/lisp ./cmd/lisp
go test ./lib/integrity/ ./command/          # the invariant tests
LISP=/tmp/lisp examples-integrity/demo.sh    # the documented walkthroughs
```

A finding should come as a PoC in the style of `demo.sh` /
`examples-integrity/README.md`: a throwaway repository, the exact
commands, the invariant number it violates, and the output showing a
verified run executing non-committed content. Findings that reproduce
become tests in `lib/integrity/mode_test.go`.
