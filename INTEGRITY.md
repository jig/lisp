# Integrity mode — `lisp-integrity`

`lisp-integrity` is a separate binary that runs a script *if and only
if* the code being executed matches what is committed in its Git
repository at `HEAD`, and leaves an append-only audit trail of every
run in the system journal. There are no mode flags: integrity is
always on, in every execution, and cannot be disabled. The regular
`lisp` binary has no integrity mode at all.

This document is both the user guide and the specification the
implementation is held to (`lib/integrity/mode.go`; enforced by
`lib/integrity/mode_test.go` and `command/integrity_test.go`).

Runnable mini-examples of every concept below — basic verification,
the require cascade, signatures, and what each failure looks like —
live in
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
lisp-integrity -y --test ./tests [--test-json report.json]
```

- `--version` reports version information (and accepts no other
  argument). Otherwise: only script-file execution and the verified
  test runner. No REPL,
  no `-e`, no stdin, no `--fmt` / `--debug`, no DAP/LSP servers, no
  environment variables selecting behaviour (`LOG_LEVEL` for
  verbosity is the one exception, inherited from `lib/log`). Use
  `lisp` for everything else.
- `--test DIR|FILE` verifies and runs a deftest suite: the mode
  anchors on the repository enclosing the target, and every test file
  — and, in cascade, everything it loads — is verified against `HEAD`
  as it loads. The attested exit code reflects the suite result, so a
  CI run leaves an append-only record of *which commit's tests passed*.
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
restarts (`git pull` does not move a detached `HEAD`).

## The invariant

Definitions:

- **code file** — any file evaluated as code: the script, every module
  loaded through `require`, and every file loaded through `load-file` /
  `load-file-once` (which read via the `slurp-source` builtin).

At startup:

1. The script must lie inside a Git repository with a resolvable
   `HEAD` commit `C`.
2. The script must byte-match its blob in `C`'s tree. (With `--test`
   there is no entry script: the anchor is the repository enclosing
   the target, and rule 4 covers each test file as it loads.)
3. If `/etc/lisp/allowed_signers` exists: `C` must carry an SSH
   signature by a listed key — itself or via an annotated tag
   pointing at it. The signer (key comment and SHA256 fingerprint) is
   reported in the green block and the start record. Without the file
   the block still verifies consistency but carries an explicit
   yellow `signed  no` line — a consistency-only run is visibly
   weaker, never silently green.

At runtime, while the mode is active:

4. Every code file, when loaded, must lie inside the verified
   repository and byte-match its blob in `C`'s tree. A code file
   resolving outside the repository (an `-i` include dir elsewhere,
   `~/.config/lisp/`, `/usr/local/share/lisp/`) is refused.

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
  `MESSAGE="run started"`, `ARGV` (script and arguments — or
  `["--test", target]` — as JSON), `PROTECTED`, `SIGNED`
  (always; filter unsigned runs with `journalctl SIGNED=false`), and
  `SIGNER` + `SIGNER_FINGERPRINT` (SHA256) when rule 3 applied.
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
> land in shell history). Read them from files instead.

## Builtins

| Builtin | Behaviour |
|---|---|
| `(assert-integrity)` | Throws unless running under `lisp-integrity`; returns the verified commit hash. Committed code uses it to demand the mode — effective as long as operators know the program is supposed to carry it. |
| `(assert-integrity :with-signature)` | Additionally throws unless startup rule 3 was applied (an `/etc/lisp/allowed_signers` file existed and `HEAD` verified). For code that must not run unsigned even on hosts lacking the keys file. |
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
the repository and outside the process user's write reach. It is
**verification-only**: the host needs public keys and nothing else —
no ssh-agent, no private key, ever. Code is signed by whoever
releases it (a programmer or CI, with their own tooling and keys),
orthogonally to execution; `lisp-integrity` never writes to the code
repository, so it has nothing to sign.

Future evolution: keys baked into the binary at build time
(`-ldflags -X`), shrinking the trust anchor to the binary alone.

## Data and state

The code repository holds **code and configuration only**; everything
under it is verified uniformly against `HEAD`, and nothing in it is
writable by the running program. Mutable data lives **outside**: a
separate data repository managed explicitly from lisp (`lib/git`, with
allowed-signers signing included), a database (`lib/sql`), or plain
files. Data integrity is the application's concern, by design — the
interpreter attests *code*. (A `state-*` helper family over a separate
data repository may return in a future revision.)

## Deployment: hardening the assurance into a boundary

The mode becomes a real boundary only when the OS guarantees the
attacker cannot write to what the interpreter reads:

- repository checkout, `/etc/lisp/allowed_signers` and the
  `lisp-integrity` binary owned by `root` (or a dedicated `deploy`
  user);
- the process running as an unprivileged user with **no write access**
  to any of the three — nothing in the mode requires the process to
  write inside the checkout;
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
  `.lisp` files unfiltered. On case-insensitive or
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
- **The `.state/` store is removed** (`state-save` / `state-load` and
  the state-commit rules): the code repository is code and
  configuration only, and mutable data lives outside it (see Data and
  state). HEAD is therefore fully static between deployments.
- The preamble flags (`-P`) are not part of `lisp-integrity`.
