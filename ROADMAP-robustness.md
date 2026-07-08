# Roadmap — robustness, testing and docs findings (2026-07 audit)

Findings from a code audit on 2026-07-08 (develop @ c1fdc0c), written
down so they can be picked up and fixed independently, one item at a
time. Same vocabulary as [ROADMAP.md](ROADMAP.md): **surgical** means
contained and safe to pick up any time, **invasive** touches the
interpreter hot path and needs care.

Every bug below was **reproduced against the actual binary** — repro
commands are included so a fix can start by turning the repro into a
regression test. Mark items done with ~~strikethrough~~ + date, as in
ROADMAP.md.

Status summary (tick as you go):

- [x] 1.1 CI: run tests with `-race` (done 2026-07-08)
- [x] 1.2 Data race in `Future.Done` / `Future.Cancelled` (done 2026-07-08)
- [x] 2.1 Panic: `(try 1 (catch))` — catch with no binding (done 2026-07-08)
- [x] 2.2 Panic: `(try ())` — `first()` on empty list (done 2026-07-08)
- [x] 2.3 Panic: `(fn [&])` called — trailing `&` in binds (done 2026-07-08)
- [x] 2.4 Panic: `(quasiquote (unquote))` — and sibling `splice-unquote` (done 2026-07-08)
- [x] 2.5 `recur` outside `loop` silently returns the sentinel (done 2026-07-08)
- [x] 2.6 nil-context contract: `try` panics where the EVAL loop tolerates nil (done 2026-07-08)
- [x] 2.7 `malRecover` re-panics on non-error panic values (done 2026-07-08)
- [x] 3.1 Fuzz tests for READ and EVAL (done 2026-07-08)
- [ ] 4.1 Honest coverage numbers (`-coverpkg`) + targeted gap tests
- [x] 5.1 Document the embedding contract (done 2026-07-08)
- [ ] 5.2 `lib/coreextented` typo in the public import path
- [ ] 5.3 Sweep of commented-out dead code
- [ ] 5.4 Release notes for tags

Suggested order: phase 1 first (one real bug + the CI line that would
have caught it), then phase 2 (all four panics share the same fix
pattern), then phase 3 to verify no panics remain. Phases 4–5 are
independent and can be picked at leisure.

## Phase 1 — CI and the confirmed race (surgical, do first)

**Done 2026-07-08** (branch `fix/future-data-race`): `Future.Done` and
`Future.Cancelled` are now `atomic.Bool`; `Cancel` uses a
`CompareAndSwap` to make its check-then-act atomic. CI gained a
ubuntu-only `race` job (plain + `lispdebug`). Regression test
`TestFutureConcurrentStateNoRace` in
[lib/concurrent/future_race_test.go](lib/concurrent/future_race_test.go)
reproduces the original race (4 reports under `-race` on the old code)
and passes on the new. `go test -race ./...` is clean.

### ~~1.1 CI: run tests with `-race`~~ (done 2026-07-08)

`.github/workflows/test.yml` runs `go test ./...` without `-race`, so
the data race in 1.2 was never seen by CI (a local
`go test -race ./...` fails today). Add a race pass — either a flag on
the existing matrix step or a separate ubuntu-only job if macOS minutes
matter. The `lispdebug` pass should get it too.

*Effort: trivial. Do together with 1.2 so the new job is born green.*

### ~~1.2 Data race in `Future.Done` / `Future.Cancelled`~~ (done 2026-07-08)

`Future.Done` and `Future.Cancelled` are plain bools
([lib/concurrent/concurrent.go](lib/concurrent/concurrent.go)):

- written by the future's goroutine: `defer func() { f.Done = true }()`
  (NewFuture, ~line 138)
- read without synchronisation by `future-done?` / `future-cancelled?`
  (Load, lines 28–29)
- `Cancel()` (line 152) does an unsynchronised check-then-act on `Done`

Repro: `go test -race ./...` (reported from the step-test suite).

Fix options, pick one:

- `atomic.Bool` for both fields — smallest diff, but note both fields
  are exported, so external readers keep compiling only if the type
  keeps a `Done`/`Cancelled` API (methods instead of fields is an API
  break; see compat note below).
- Derive state from the existing channels/context instead of storing
  it (`ctx.Err()` for cancelled; a `close`d done channel for done).

Compatibility: the fields are exported and `Future` is a public type.
If changing field types is unacceptable before v0.3, an internal
mutex + keeping the fields as-is (documented as "read via methods
only") is the conservative middle ground.

Acceptance: `go test -race ./...` clean; a test that spins
`future-done?` in a loop while the future completes stays clean.

*Effort: small. Impact: real bug today — the race detector flags reads
that can miss a completion or tear on some platforms.*

## Phase 2 — interpreter panics reachable from pure Lisp (surgical)

For an interpreter sold as *embeddable* (config files, transmission
format) these are the highest-severity class: malformed **input** must
never crash the host Go process. All four panics below are index
errors on unchecked list shapes; the fix pattern is identical (length
check → `lisperror.NewLispError` with the form's cursor), and each
repro becomes a table entry in a new `malformed_input_test.go`
asserting *error, not panic*.

The neighbouring forms already do this right — `(defmacro)`, `(def)`,
`(let [x] x)`, `(1 2 3)` all return clean positioned errors — so this
is finishing a job, not introducing a new convention.

**Done 2026-07-08** (branch `fix/interpreter-panics`): 2.1–2.4 fixed;
`quasiquote`/`qq_loop` now return `(MalType, error)` instead of
indexing blindly. Regression table
[malformed_input_test.go](malformed_input_test.go) asserts *error, not
panic* for every repro (plus a `(fn [3] …)` non-symbol-binding case
found while fixing 2.3), and pins two happy-path forms. The `catch`
arity message is unchanged (compat), just moved before the indexing.

### ~~2.1 `(try 1 (catch))` — catch with no binding~~ (done 2026-07-08)

```
$ echo '(try 1 (catch))' | lisp /dev/stdin
panic: runtime error: index out of range [1] with length 1
```

[mal.go](mal.go) `case "try"`: `catchBind = last.(List).Val[1]`
(~line 640) indexes without checking the catch clause's length. Same
unchecked access in the `finally`→`catch` path a few lines below
(~line 650). Note there *is* already a "catch must have 2 arguments at
least" check — it just runs after the indexing.

### ~~2.2 `(try ())` — `first()` on an empty list~~ (done 2026-07-08)

```
$ echo '(try ())' | lisp /dev/stdin
panic: runtime error: index out of range [0] with length 0
```

`first()` ([mal.go](mal.go) ~line 777) checks the argument is a List
but not that it is non-empty before `list.(List).Val[0]`. One-line
guard; fixes any caller.

### ~~2.3 `(fn [&])` called — trailing `&` in the binds vector~~ (done 2026-07-08)

```
$ echo '(def f (fn [&] 1)) (f)' | lisp /dev/stdin
panic: runtime error: index out of range [1] with length 1
```

[env/env.go](env/env.go) `_newSubordinateEnvWithBinds` (~line 63):
`binds[i+1]` assumes a symbol follows `&`. Also unchecked on the same
line: the `.(types.Symbol)` assertion (a non-symbol after `&`, e.g.
`(fn [& 3])`, panics too). Guard both; return the same kind of
positioned error the function already produces for arity mismatches.

### ~~2.4 `(quasiquote (unquote))` — and the `splice-unquote` sibling~~ (done 2026-07-08)

```
$ echo '(quasiquote (unquote))' | lisp /dev/stdin
panic: runtime error: index out of range [1] with length 1
```

[mal.go](mal.go) `quasiquote` (~line 189): `return a.Val[1]` after
`starts_with(a.Val, "unquote")` — a bare `(unquote)` has length 1.
`qq_loop` (~line 171) has the same shape for `splice-unquote`
(`e.Val[1]`); fix and test both together.

**Done 2026-07-08** (branch `fix/eval-contract`): 2.5–2.7 fixed. EVAL
is now a thin wrapper over `evalInternal` that rejects a `recurValue`
escaping every loop; recursive interpreter calls go through
`evalInternal` so `loop` still receives the sentinel transparently.
`try` guards its deadline lookup for nil contexts; `malRecover` wraps
non-error panic values. Tests in
[eval_contract_test.go](eval_contract_test.go).

### ~~2.5 `recur` outside `loop` silently returns the sentinel~~ (done 2026-07-08)

```
$ echo '(recur 1 2)' | lisp /dev/stdin
«recur»            # exit code 0
```

A `recurValue` that escapes to the caller of EVAL means no enclosing
`loop` consumed it. The in-code comment (mal.go, `recurValue`) argues
a non-tail `recur` should fail at a real consumer — fine — but at the
*top level* nothing consumes it and it leaks as a printable value with
exit 0. Cheapest complete fix: in the public entry points
(`EVAL`'s callers: REPL / REPLWithPreamble / ReadEvalWithPreamble) or
at the end of EVAL's outermost return, translate an escaping
`recurValue` into "recur outside loop" error. Decide whether
`MalFunc` application should also reject it (Clojure allows `recur`
to rebind fn params — out of scope here; jig/lisp only pairs it with
`loop`).

### ~~2.6 nil-context contract: `try` panics where the EVAL loop tolerates nil~~ (done 2026-07-08)

EVAL's main loop guards `if ctx != nil` (mal.go ~line 438), which
reads as "nil context is supported" — but the `try` branch calls
`ctx.Deadline()` unconditionally (~line 666), so any embedder passing
a nil ctx panics on the first `try`. Pick a side:

- **Require non-nil ctx** (recommended): drop the nil guard in the
  loop, state the requirement in EVAL's godoc, and make the public
  entry points fail fast with a clear error. Matches stdlib
  conventions (`context.TODO()` exists for a reason).
- Or support nil everywhere: guard the `try` branch the same way.

**Decision (2026-07-08): support nil everywhere.** The EVAL loop
already tolerated nil, so guarding the `try` deadline lookup is the
smaller, non-breaking change — no embedder that passes nil today
breaks. A nil context simply disables cancellation and try timeouts;
this is now stated in EVAL's godoc and belongs in the embedding
contract doc (5.1).

### ~~2.7 `malRecover` re-panics on non-error panic values~~ (done 2026-07-08)

[mal.go](mal.go) ~line 787: `*err = rerr.(error)` — an unchecked type
assertion inside a recover. Go builtins are shielded by
`call._recover` (which handles non-error values properly, see
[lib/call/call.go](lib/call/call.go) ~line 163), so today only
runtime errors (which satisfy `error`) reach this path — defensive
hardening, not a live bug. Reuse `call._recover`'s switch. *Effort:
trivial.*

## Phase 3 — fuzzing (surgical; validates phase 2)

**Done 2026-07-08** (branch `feature/fuzzing`): `FuzzReadStr`
([reader/fuzz_test.go](reader/fuzz_test.go)) and `FuzzEval`
([fuzz_test.go](fuzz_test.go)), seeded with valid forms + every phase-2
repro. `FuzzEval` is `package lisp_test` (external) because `nscore`
imports `lisp`; it reads then evaluates under a 100ms timeout in a
fresh child of a core-loaded env so top-level defs don't leak. Both ran
**70s locally with no crasher**. CI gained a `fuzz` job (30s each).
Known limit documented in the test: deep non-tail Lisp recursion can
still exhaust the Go stack (unrecoverable) — byte-level fuzzing is very
unlikely to synthesise it, and a recursion-depth limit is a separate
item, not covered here.

### ~~3.1 Fuzz tests for READ and EVAL~~ (done 2026-07-08)

Native Go fuzzing would have found every panic in phase 2 unattended.
Two targets:

- `FuzzREAD` in the reader package: `Read_str` on arbitrary input must
  return `(ast, error)`, never panic. Seed with the repo's `.lisp` /
  `.mal` files and the phase-2 repros.
- `FuzzEVAL` at the repo root: READ + EVAL with a short
  `context.WithTimeout` (100ms) and a minimal env (`nscore.Load`
  only), asserting "no panic". The timeout matters: fuzzing will find
  infinite loops, and the ctx check in EVAL's loop is what stops them
  — which incidentally also exercises 2.6's decision.

CI: a short job (`go test -fuzz=FuzzREAD -fuzztime=30s`, same for
EVAL) on pushes to develop/main; corpus files committed under
`testdata/fuzz/` as they accumulate. Long fuzzing sessions stay
manual/local.

Acceptance: both fuzzers survive ≥10 min locally with phase-2 fixes
in. Every crasher found gets committed to the corpus before fixing.

*Effort: small to write; finds bugs forever after. Do after phase 2
or the first minute of fuzzing just rediscovers known crashes.*

## Phase 4 — test coverage (surgical, ongoing)

### 4.1 Honest coverage numbers + targeted gap tests

`go test -cover ./...` under-reports badly: `lib/core` shows 16.5%
but is exercised constantly by the root step-tests — cross-package
coverage isn't counted without `-coverpkg`. First step, measure for
real:

```bash
go test -coverpkg=./... -coverprofile=cover.out ./... && go tool cover -func=cover.out
```

Then close the *genuine* gaps (as of 2026-07-08, own-package numbers):

- **env 35%** — and the uncovered varargs path is exactly where bug
  2.3 lives. Table-test `_newSubordinateEnvWithBinds` directly:
  varargs, too-few/too-many args, malformed binds.
- **repl 12%** — the readline loop is hard to test, but the
  eval-and-print plumbing can be driven with a fake io.Reader.
- **printer, lisperror, runtime, marshaler, types — no test files at
  all.** printer/lisperror matter most (error rendering has
  compatibility constraints per ROADMAP.md's compat notes — pin the
  current formats with tests *before* anything touches them).
- **docmeta 0%** own-package (its consistency test lives elsewhere) —
  fine, no action.

Optional: add the `-coverpkg` run to CI with a soft threshold
(report, don't fail) to watch the trend.

*Effort: incremental, parallelisable — each bullet is a standalone PR.*

## Phase 5 — maintainability and documentation (pick at leisure)

### ~~5.1 Document the embedding contract~~ (done 2026-07-08)

**Done 2026-07-08** (branch `docs/embedding-contract`): an "Embedding
contract" section in the README plus a `# Embedding contract` block in
the root-package godoc, covering panics-vs-errors (incl. the non-tail
recursion and `(panic …)` caveats), nil context, the shared-base +
per-goroutine-child + atoms concurrency pattern, and `strings.Contains`
error matching. The concurrency claim was checked with a throwaway
`-race` test (50 concurrent EVALs over a shared base env mutating one
atom → 50, clean).

The README explains *how* to embed but not the *guarantees*. One
"Embedding contract" section (README or root-package godoc) stating:

- what can panic vs what returns error (after phase 2: "malformed
  input never panics; panics escaping EVAL are interpreter bugs")
- whether ctx may be nil (the 2.6 decision)
- thread-safety: `Env` is internally locked (RWMutex) and one env may
  be shared by concurrent EVALs; atoms are the intended mutable-state
  primitive; futures detach from the debugger session
- error matching guidance: `strings.Contains`, not equality (per the
  ROADMAP.md compat note on cursor prefixes)

*Effort: small. Do after 2.6 so the ctx statement is true.*

### 5.2 `lib/coreextented` typo in the public import path

"coreextented" (sic) is imported by every embedder, so a rename is an
API break. Options, in increasing ambition:

1. Document as a known wart (this entry) — zero risk, default.
2. New `lib/coreextended` package re-exporting everything; old path
   kept as a deprecated wrapper; remove at v0.3.
3. Rename outright at the next minor bump with a loud release note.

Related memory note: stdlib functions live in
`header-coreextended.lisp` (correctly spelled) — only the Go package
directory carries the typo. Decide, then either do 2/3 or strike this
item as "documented, won't fix".

### 5.3 Sweep of commented-out dead code

Leftover commented code confuses later readers into thinking there is
a pending decision: [env/env.go](env/env.go) lines ~74, ~78, ~191,
~198; [mal.go](mal.go) ~line 113 (duplicated `strings.Cut` line).
Pure deletion, one small PR. While there: the `a1`/`a2` extraction
switch in mal.go (~lines 491–504) collapses to two `if len > n`
lookups. **Deliberately out of scope: restructuring EVAL's big switch
— it is the mal/TCO style and churn there is all risk, no payoff.**

### 5.4 Release notes for tags

Tags reach v0.2.24 with no changelog anywhere. Lightest viable
process: `gh release create` per tag with auto-generated notes from
merge commits (the history is clean conventional-commits, so
auto-notes read well). Backfilling old tags is optional; start with
the next one.

## Backward compatibility (same ground rule as ROADMAP.md)

- Phase 2 turns **panics into errors** — strictly widening: code that
  worked keeps working, code that crashed now gets a catchable
  `LispError`. The only observable change is exit behaviour on inputs
  that previously killed the process. Safe.
- 2.5 (`recur` sentinel) changes a *silent wrong result* into an
  error — technically a behaviour change; note it in the release.
- 1.2: `Future`'s exported fields are public API — prefer the fix
  variant that keeps the struct shape unless v0.3 is imminent.
- Error **message strings** are compatibility surface (README advises
  `strings.Contains` matching): new errors are fine, changing existing
  wordings is not — phase 4's pin-the-format tests for
  printer/lisperror protect exactly this.
