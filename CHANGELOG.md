# Changelog

Notable changes to `jig/lisp`. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/). The project is
pre-1.0, so minor tags can carry behaviour changes; the ones that may
need action when upgrading are called out under **Changed** with a
migration note.

## Unreleased (since v0.2.24)

### ⚠️ Changed — one binary, `debugger` build tag (was `lispdebug`)

The two-binary split (`lisp` + `lisp-debug`) is gone. There is one
`lisp` binary, and one build tag, renamed **`lispdebug` → `debugger`**:

- **Installing the CLI/REPL**: build with the tag —
  `go install -tags debugger github.com/jig/lisp/cmd/lisp@latest`. This
  compiles in the LSP server, the DAP debugger and coverage. A cheap
  hook check in the evaluator keeps the cost of an *idle* debugger
  build under ~1% geomean (~3% on eval-heavy loops) — frame tracking
  only starts when a hook is installed or a debug session begins.
- **Embedding in Go**: build without tags (the default for any
  importer) and the evaluator's debug paths are compiled out entirely,
  as before. `--dap`, `--lsp`, `--coverage` and `--debug` error out in
  such builds.
- Anyone scripting the old names must switch `-tags lispdebug` →
  `-tags debugger` and `lisp-debug` → `lisp`; the VS Code extension
  defaults now spawn `lisp`.

### ⚠️ Changed — error message formatting

Two related changes affect the **text** of error messages (what
`LispError.Error()` returns, and what the CLI/logs print). They do not
change any Go API signature, and — importantly — they do **not** change
the value a `catch` binds (see "What is *not* affected" below). If your
code matches on, parses, or displays the *text* of errors, read this.

Both changes are, in isolation, fixes: v0.2.24 leaked Go's internal
struct/reflection formatting into user-facing error text (including
malformed `%!s(...)` verbs). The new output is the intended
Lisp-readable form. But the text differs, so treat it as a breaking
change to the error *format*.

#### 1. Thrown non-error values render as Lisp data

When a non-error value is raised — `(throw {:a 1})`, `(throw 42)`,
`(throw "boom")`, or a Go builtin failing with a non-`error` payload —
`Error()` now renders it with the Lisp printer (`printer.Pr_str`)
instead of Go's `fmt.Sprint`.

| Expression        | v0.2.24 error text                    | develop error text |
|-------------------|---------------------------------------|--------------------|
| `(throw "boom")`  | `boom`                                | `"boom"` (quoted)  |
| `(throw 42)`      | `%!s(int=42)`                         | `42`               |
| `(throw {:a 1})`  | `{map[ʞa:%!s(int=1)] <nil> -e:1}`     | `{:a 1}`           |
| `(+ 1 "x")`       | `reflect: Call using string as type int` | `"reflect: Call using string as type int"` (quoted) |

Note the two practical gotchas: thrown **strings are now quoted**
(`"boom"`, not `boom`), and internal Go error messages that reach this
path also gain quotes.

(Introduced in the Stage-1 cleanup, commit `6a31817`.)

#### 2. Library / macro-expanded errors carry a `file:line:` prefix

Errors originating inside loaded Lisp library code (the `nscore`
headers) or inside macro-expanded code now carry a source-position
prefix they previously lacked, because expanded forms now get source
positions (commit `63c6647`).

| Expression       | v0.2.24 error text                                | develop error text |
|------------------|---------------------------------------------------|--------------------|
| `(reduce + [1])` | `too few arguments passed (…) (around do)`        | `$nscore:29: too few arguments passed (…) (around do)` |

Errors that already had a position (e.g. `-e:1: nth: index out of
range`) are unchanged.

### What is *not* affected

- **`try`/`catch`.** The value bound in a `catch` clause is the
  original thrown value, not its formatted text — identical in both
  versions. `(try (throw {:a 1}) (catch e (get e :a)))` returns `1` in
  both. If you handle errors by catching and inspecting the value, you
  need to change nothing.
- **Go API signatures.** `READ`, `EVAL`, `PRINT`, the `REPL*` helpers,
  `env.*`, `nscore.Load`, `lisperror.*` etc. keep their signatures.
  Upgrading does not break compilation.
- **Printing of normal values.** `prn`, `str`, `json-encode` and the
  rest render values exactly as before.

### Migration

- **Go embedders matching on error text:** match with
  `strings.Contains` on a stable substring, never string equality, and
  expect the new `file:line:` prefix on some messages. Better: type-
  assert to `lisperror.LispError` and use `ErrorValue()` (the original
  `MalType`) for thrown values, or `errors.Is` / `errors.As` for Go
  errors — none of which depend on the rendered text.
- **Thrown strings:** if you raise a string and later match its text,
  account for the surrounding quotes (`"boom"`). Prefer raising a value
  you can inspect in `catch` (a map/keyword) over matching a message.
- **Lisp code using `try`/`catch`:** inspect the caught **value**, not
  a formatted string, and you are unaffected.
- **Tests asserting exact error strings (Go or Lisp `assert-throws`):**
  update expected strings to the new format, or relax them to substring
  matches.

See the "Embedding contract" section of the [README](./README.md#embedding-contract)
for the general error-matching guidance.

### ⚠️ Changed — `coreextented` package renamed to `coreextended`

The misspelled package `lib/coreextented` (missing a `d`) is renamed to
`lib/coreextended`. Both the import path and the Go package identifier
change; the files inside (`coreextended.go`, `header-coreextended.lisp`)
and the child package `nscoreextended` were already spelled correctly.

There is **no back-compat shim** — this is a clean break for 0.3.
Embedders must update their imports:

```go
// before
import "github.com/jig/lisp/lib/coreextented"
import "github.com/jig/lisp/lib/coreextented/nscoreextended"
// after
import "github.com/jig/lisp/lib/coreextended"
import "github.com/jig/lisp/lib/coreextended/nscoreextended"
```

The `nscoreextended` subpackage path (`.../lib/coreextended/nscoreextended`)
and every identifier under it are unchanged apart from the parent
segment; a find-and-replace of `coreextented` → `coreextended` across
your code covers it.

### ⚠️ Changed — radix literals are arbitrary-precision big ints

`0x…`, `0o…` and `0b…` literals (optionally signed) now read as
**arbitrary-precision integers** (`*big.Int`) — the natural type for
serial numbers, hashes and masks — and print back as `(-)0x…`
uppercase hex padded to whole octets (`0xf` → `0x0F`, zero → `0x00`,
`-0x01` → `-0x01`), so they round-trip through the printer. They
compare numerically with `=` against other big ints, but are
**distinct from machine ints** (`(= 0x0A 10)` is `false`) and do not
participate in arithmetic (`(+ 0x01 1)` errors). Decimal literals keep
reading as signed machine ints, unchanged; legacy leading-zero octal
(`042`, `00`) is error-prone and now a read error — use `0o…`.

Migration: hex/octal/binary literals used *arithmetically* must become
decimal (or be wrapped in your own conversion); literals used as
identifiers or bit patterns keep working and now survive any width.

### Other notable additions since v0.2.24

Non-breaking, for context (see `git log v0.2.24..` for the full list):

- `--integrity REF` — run a script if and only if it matches what is
  committed in its Git repository at REF (a commit hash, tag or
  branch): HEAD must be exactly REF, the script must byte-match its
  committed blob, and the check cascades to every `require` resolved
  inside the same repository (modules resolving outside it are
  refused). `--integrity-signers FILE` additionally requires REF to be
  SSH-signed by a key listed in FILE (authorized_keys format, as
  `git-verify-commit`). The new `(assert-integrity)` builtin throws
  unless the run is verified — so committed code can demand the flag —
  and returns the verified commit hash. This is an operational
  assurance against drift and uncommitted edits, not a security
  boundary against whoever can rewrite the repository or the binary.
  See `lib/integrity/README.md`
- `exit` builtin: `(exit)` / `(exit status)` ends the process with the
  given status (`0` by default), like Clojure's `System/exit`. It stops
  before the interpreter echoes a script's final value, so a program run
  as a file can set a real exit code without printing a trailing result
- `loop`/`recur` special forms; `into`, and `conj` accepting `[k v]`
  pairs / maps
- `recur` in a function's tail position (Clojure semantics): the
  function is a recursion point — `recur` rebinds its parameters and
  restarts the body in constant stack, on both direct calls and the
  `apply`/`map`/`swap!` paths. Previously it leaked the internal
  `«recur»` sentinel as a value
- multi-line `"…"` strings (Clojure-style): a literal newline is kept
  verbatim; `¬…¬` remains for JSON and other escape-heavy content
- `web` library — a Ring-style HTTP/HTTPS server: request and response
  are hash-maps, handlers are `(fn [req] resp)`, middleware is
  `handler → handler`. `web-serve` (TLS, mTLS, graceful shutdown),
  `web-router` (data-driven routes with path params), response helpers
  (`web-json` with clean keys, `web-text`, `web-not-found`, …), and
  middleware (`web-wrap-recover`, `web-wrap-log` as structured slog
  JSON, `web-wrap-json-body`, `web-wrap-identity` for mTLS,
  `web-wrap-jwt`). JWT verification (`web-verify-jwt`) validates against
  a JWKS with issuer/audience checks — Keycloak-compatible (RS/ES).
  Client-side and streaming are planned separately. See
  `lib/web/README.md`
- `read-password` — reads a line from stdin with terminal echo
  disabled (via `golang.org/x/term`), for passwords and secrets; the
  prompt goes to stderr, and non-terminal input (pipes, tests) falls
  back to a plain line read. Returns nil on end of input, like
  `readline`
- `test` library — Clojure-style unit testing for lisp code: `deftest`,
  `is` (with expected/actual reporting for `(is (= …))`), `are`
  templates and `test/run-tests!`. The CLI runner grew with it:
  `--test` now also accepts a single file, runs registered tests after
  loading (legacy `*_test.mal` suites keep working), reports Go-style
  failures, exits non-zero, and `--test-json FILE` writes a machine
  report. In `debugger` builds, `--coverage FILE` records per-line
  execution of the lisp sources (lcov; test files excluded) for any run
  or test suite. The VS Code extension (0.9.0) integrates both: a
  Testing panel with per-test run buttons and failure diffs, and a
  **Coverage** profile that paints covered/uncovered lisp lines through
  VS Code's native coverage view
- `deftests/` — a deftest replica of the historical `tests/step*.mal`
  suites (200+ tests, 1300 value-based checks), run by `TestDeftests`
  and by `lisp --test deftests/`; the originals stay untouched under
  the line-based harness
- Debug Test: the VS Code extension (0.9.3) gains a **Debug** run
  profile and Go-style CodeLens — **Run Test | Debug Test** above each
  `deftest`, plus **Run File Tests | Run All Tests** at the top of any
  file that has tests. Debugging launches a DAP session that loads the file and
  runs just that test (new `test/run-test!` builtin + `--run-test`
  flag), so breakpoints in the test body are hit. The debugger no
  longer stops when a `(fn …)` closure is merely created (only when its
  body runs), which also removes a spurious load-time stop for
  breakpoints inside any function
- shebang scripts: a leading `#!/usr/bin/env lisp` line reads as a
  comment (the two bytes become `;;` in place, so positions stay
  exact), `lisp --fmt` preserves it verbatim, and the VS Code
  extension (0.9.1) recognises extensionless files by their first line
- `read-program` builtin and `reader.Read_program`: read a whole file
  — any number of top-level forms — into one `(do …)` AST with
  positions matching the source exactly. `load-file`, script
  execution, the LSP analyser and the coverage universe now build on
  it, retiring the textual `"(do …)"` wrapping, the `;; $MODULE`
  prefix line and every row-shift correction that came with them
- commas are whitespace, as in Clojure and kanaka/mal: `(1 2, 3)`
  reads as `(1 2 3)` (previously a comma silently read as a `,`
  symbol; the formatter already treated commas as whitespace)
- `with-out-str` (test library) — captures standard output as a
  string, Clojure-style, making printing behaviour testable; note it
  swaps the process-wide stdout, so concurrent goroutine output is
  captured too
- ⚠️ removed: the pre-deftest `assert` library (`assert-true`,
  `assert-false`, `assert-throws`, `test-suite`) — superseded by
  `deftest`/`is`/`are`, with no remaining users. The core `assert`
  builtin (runtime precondition, as in Clojure) stays
- core promotions: `and`, `or`, `when`, `inc`, `dec`, `gensym` moved
  from `coreextended` into the core header (their Clojure counterparts
  live in clojure.core), and `load-file-once` sits next to `load-file`
  (as a Go builtin: its seen-set needs mutable state and atoms belong
  to the concurrent library). Library headers can now rely on all of
  them with only core loaded; loading `coreextended` behaves as before
- Clojure-style arithmetic: `+ - * /` are variadic — `(+)`→0, `(*)`→1,
  `(+ x)`→x, `(- x)`→negation, `(/ x)`→1/x, `(- a b c)` folds left —
  and the ordering builtins `< <= > >=` chain (`(< 1 2 3)`). The
  numeric tower now spans machine ints, floats and big ints with
  Clojure-style contagion (int→float, int→big); big ints and floats do
  not mix. Floats previously had **no** arithmetic at all. Division by
  zero on ints/bigs is a catchable `"division by zero"` error (was a
  wrapped Go runtime panic); non-numbers report
  `"not a number (was of type T)"`
- `time-format` / `time-parse` — RFC 3339 UTC timestamps to and from
  the epoch milliseconds of `time-ms`
- printer: dedicated renderings for Go values — `time.Time` and
  `time.Duration` as milliseconds (matching `time-ms`), `*big.Int` as
  `0x…` hex
- fixed: the result of a `catch` body was evaluated twice, re-raising
  `throw` forms carried as data (issue #87)
- `require` with `:as` / `:refer` / `:refer :all`, a module search path
  and `LISPPATH`
- `cli` library — command-line option parsing modelled on
  clojure/tools.cli (`cli-parse-opts`), plus the `subs`, `starts-with?`
  and `ends-with?` string builtins it builds on
- `integrity` library — attest and verify lisp source: `fmt` (canonical
  formatting, as `lisp --fmt`), `sha2-256`, and deterministic Ed25519
  signatures (`ed25519-generate`, `ed25519-sign`, `ed25519-verify`)
- `spit` — the write counterpart of `slurp`, Clojure-style, with
  `:append true` support
- `*FILE*` — absolute path of the script being executed (cf. Clojure's
  `*file*`); unset in the REPL and `-e`
- LSP/DAP improvements (signature help, hover docs, macro-aware
  stepping) under the `debugger` build tag
- Clojure-style docstrings on `defn`, and `call.Doc` for documenting Go
  builtins
- Robustness pass: malformed input now returns errors instead of
  panicking, a fixed `Future` data race, and READ/EVAL fuzzers
