# Integrity mode — `lisp-integrity`

`lisp-integrity` is a separate binary that runs a script *if and only
if* the code being executed matches what is committed in its Git
repository at `HEAD`, and leaves an append-only audit trail of every
run in the system journal. There are no mode flags: integrity is
always on, in every execution, and cannot be disabled. The regular
`lisp` binary has no integrity mode at all.

This document is both the user guide and the specification the
implementation is held to (`lib/integrity/mode.go`, `state.go`;
enforced by `lib/integrity/mode_test.go` and
`command/integrity_test.go`).

Runnable mini-examples of every concept below — basic verification,
the require cascade, signatures, the state store, and what each
failure looks like — live in
[examples-integrity/](./examples-integrity/), with a `demo.sh` that
replays all of them in throwaway repositories. Reviewers trying to
break the mode should start from the adversarial review brief in
[INTEGRITY-REVIEW.md](./INTEGRITY-REVIEW.md).

## Purpose and threat model

Two guarantees, deliberately distinct:

- **Consistency** (prevention): what runs is exactly what is
  committed — no accidental drift, no uncommitted edits, no locally
  patched copy. Enforced at startup and on every code load;
  violations refuse to run.
- **Attestation** (detection): every run leaves start/end records in
  systemd-journald — append-only storage the executing user cannot
  alter or delete, with kernel-verified metadata (`_UID`, `_EXE`,
  `_PID`) the process cannot forge. An auditor can always answer
  *what code ran, when, as whom, with what arguments, and how it
  ended*.

It is **not a security boundary** against whoever controls the
repository checkout, `/etc/lisp/allowed_signers` or the binary; that
separation belongs to the operating system (see
[Deployment](#deployment-hardening-the-assurance-into-a-boundary)).
An attacker can always run modified code with some other tool — what
they cannot do is produce a *verified-looking* attestation for it.

## CLI

```bash
lisp-integrity script.lisp [args…]
lisp-integrity -y service.lisp [args…]
```

- Only script-file execution. No REPL, no `-e`, no stdin, no
  `--test` / `--fmt` / `--debug`, no DAP/LSP servers, no environment
  variables selecting behaviour (`LOG_LEVEL` for verbosity is the one
  exception, inherited from `lib/log`). Use `lisp` for everything
  else.
- After verification succeeds and the green block is printed,
  `lisp-integrity` asks for confirmation on the terminal
  (`proceed? [y/N]`) before evaluating anything.
  - `-y` skips the question.
  - If stdin is **not a TTY** and `-y` was not given, the run fails
    closed. A systemd unit must therefore state its consent
    explicitly: `ExecStart=/usr/local/bin/lisp-integrity -y …`.

There is no ref argument. **Pinning a release is a property of the
checkout, not of the invocation**: deploy with
`git checkout --detach v1.4.2` and `HEAD` stays on that commit across
restarts (`git pull` does not move a detached `HEAD`; only the
script's own `state-save` commits advance it, as children of it).

## The invariant

Definitions:

- **code file** — any file evaluated as code: the script, every module
  loaded through `require`, and every file loaded through `load-file` /
  `load-file-once` (which read via the `slurp-source` builtin).
- **state path** — any path under `.state/` at the repository root.

At startup:

1. The script must lie inside a Git repository with a resolvable
   `HEAD` commit `C`.
2. The script must byte-match its blob in `C`'s tree.
3. If `/etc/lisp/allowed_signers` exists: walking from `C` down
   through consecutive state-save commits (commits with one parent
   touching only state paths), each such commit must carry an SSH
   signature by a listed key, and the first non-state commit under
   them — the **release commit** — must be signed itself or via an
   annotated tag pointing at it. The release signer is reported in
   the green block and the start record.

At runtime, while the mode is active:

4. Every code file, when loaded, must lie inside the verified
   repository and byte-match its blob in `C`'s tree. A code file
   resolving outside the repository (an `-i` include dir elsewhere,
   `~/.config/lisp/`, `/usr/local/share/lisp/`) is refused.
5. Every state file, when read through `state-load`, must byte-match
   its blob at the **current** `HEAD` (the commit the last
   `state-save` created). Missing-but-committed, present-but-
   uncommitted, and differing files all fail closed.

Uncommitted repository files that are never interpreted do not affect
any check.

## Audit trail (journald)

On Linux the destination is **systemd-journald and nothing else** —
if the journal socket is unavailable, `lisp-integrity` refuses to
run. On macOS (development convenience; explicitly weaker) records
fall back to `~/.local/state/lisp/<script>.log` and carry
`PROTECTED=false`.

Every record shares three fields:

- `COMMIT` — the verified `HEAD` hash.
- `REPO` — the `origin` remote URL, or the repository root's basename
  when there is no remote.
- `TRACE_ID` — 128-bit random hex, constant for the whole run.

Records emitted by the runtime itself (exactly two, never more):

- **start** — after verification and confirmation, before evaluation:
  `MESSAGE="run started"`, `ARGV` (script and arguments, as JSON),
  `SIGNER` when rule 3 applied, `PROTECTED`.
- **end** — from a deferred handler covering normal return, error and
  panic: `MESSAGE="run ended"`, `EXIT_CODE` (and `PANIC` on one). A
  start record with no matching end record means abnormal termination
  (SIGKILL, power loss) — that asymmetry is signal, not defect.

Every `log-debug` / `log-info` / `log-warn` / `log-error` call from
the script carries the three shared fields in addition to its own.
`SYSLOG_IDENTIFIER` is the script basename; read a run back with:

```bash
journalctl -t <script> TRACE_ID=<id> -o json
```

> `ARGV` is recorded verbatim and the journal is append-only by
> design: **never pass secrets on the command line** (they would also
> land in shell history). Use files or the state store.

## Builtins

| Builtin | Behaviour |
|---|---|
| `(assert-integrity)` | Throws unless running under `lisp-integrity`; returns the verified commit hash. Committed code uses it to demand the mode — effective as long as operators know the program is supposed to carry it. |
| `(assert-integrity :with-signature)` | Additionally throws unless startup rule 3 was applied (an `/etc/lisp/allowed_signers` file existed and the chain verified). For code that must not run unsigned even on hosts lacking the keys file. |
| `(state-save name value & [message])` | Writes `value` as canonical lisp data to `.state/name.lisp` and **commits it in the same operation**; returns the commit hash. The commit `message` defaults to `state: name`. With an allowed-signers set active, the commit is SSH-signed via ssh-agent (no per-call key). Saving an unchanged value is a no-op returning the current commit. Works with or without the mode; requires a Git repository. |
| `(state-load name)` / `(state-load name default)` | Reads the state back as pure data (READ, never EVAL — state cannot smuggle code). Returns `default`, or throws without one, when the state does not exist. Under the mode, enforces invariant 5. |
| `(slurp-source path)` | `slurp` for files about to be evaluated: identical, plus invariant 4 under the mode. `load-file` builds on it. |

## Keys

`/etc/lisp/allowed_signers` — **authorized_keys / `.pub` format**:
one public key per line, `<type> <base64> [comment]`
(e.g. `ssh-ed25519 AAAA… alice`), blank lines and `#` comments
skipped. This is **not** git's `allowed_signers` format (which puts a
principal first); such a line is rejected, not silently accepted, so
the trust anchor is the set of keys, matched by key — no principal or
validity constraints (rotate keys by editing the file, not by
expiry).

The file's **mere presence activates rule 3** for every
`lisp-integrity` run on the host. It must be owned by root, outside
the repository and outside the process user's write reach. The same
key set also **drives signing**: while it is present, every
`git-commit`, annotated `git-tag` and `state-save` made during a run
is SSH-signed with the **ssh-agent** key whose public key is listed
(no private key ever enters the process; fails closed if no listed
key is loaded in the agent).

Future evolution: keys baked into the binary at build time
(`-ldflags -X`), shrinking the trust anchor to the binary alone.

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
  <state-save@lisp>`; they are Git-compatible SSH-signed when an
  allowed-signers set is active (ssh-agent key), unsigned otherwise.
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

## Deployment: hardening the assurance into a boundary

The mode becomes a real boundary only when the OS guarantees the
attacker cannot write to what the interpreter reads:

- repository checkout, `/etc/lisp/allowed_signers` and the
  `lisp-integrity` binary owned by `root` (or a dedicated `deploy`
  user);
- the process running as an unprivileged user with **no write access**
  to any of the three;
- if `state-save` is used, grant the process user write access to
  `.state/` and `.git` only — or accept that state (unlike code) is
  writable by the process by design;
- persistent journald (`/var/log/journal/` present) on hosts where
  the audit trail must survive reboots; journald FSS sealing
  (`journalctl --setup-keys`) adds cryptographic tamper-evidence —
  orthogonal to this spec, with its own keys;
- binary provenance (signed releases, package manager verification) is
  outside the interpreter's scope but completes the chain.

## Known limitations

- `eval` over strings obtained by other means (`slurp`, network) is
  not covered — the verified code that chooses to do that is
  responsible for it.
- A self-verifying binary is deliberately **not** attempted: an
  attacker who can replace the binary can also remove the check.
- Anyone with commit access to the checkout can run code that
  verifies green: without an allowed-signers set the anchor is
  consistency, not authorization. The attestation records what ran;
  signatures (rule 3) add who released it.
- Verification compares the **worktree bytes** against the **raw
  committed blob**. A repository that applies EOL normalization or a
  clean/smudge filter (`.gitattributes`: `text eol=crlf`, `filter=…`)
  to a verified file makes the two differ, so verification fails
  closed — it is not a bypass, but such repositories must keep their
  `.lisp` and `.state/` files unfiltered. On case-insensitive or
  unicode-normalizing filesystems (macOS), a committed path and the
  on-disk path that resolves to it must match exactly; a mismatch
  fails closed rather than verifying the wrong file.

## Changes from v1 (`lisp --integrity <ref>`)

- `--integrity REF` and `--integrity-keys FILE` are **removed**; the
  `lisp` binary loses the mode entirely. The new `lisp-integrity`
  binary is always-on and verifies against `HEAD`; pinning moved to
  the checkout (`git checkout --detach <tag>`).
- Keys moved from a flag to the fixed path
  `/etc/lisp/allowed_signers`; presence activates the signature rule.
- Interactive confirmation (`[y/N]`; `-y` to skip; non-TTY without
  `-y` fails closed).
- Mandatory journald attestation: start/end records with
  `COMMIT` / `REPO` / `TRACE_ID` / `ARGV` / `EXIT_CODE`; `log-*`
  records enriched with the same fields; macOS falls back to the XDG
  state file with `PROTECTED=false`.
- `(assert-integrity)` gained the `:with-signature` variant.
- The preamble flags (`-P`) are not part of `lisp-integrity`.
