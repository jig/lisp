# Changelog

Notable changes to `jig/lisp`. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/). The project is
pre-1.0, so minor tags can carry behaviour changes; the ones that may
need action when upgrading are called out under **Changed** with a
migration note.

## Unreleased (since v0.2.24)

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

### Other notable additions since v0.2.24

Non-breaking, for context (see `git log v0.2.24..` for the full list):

- `loop`/`recur` special forms; `into`, and `conj` accepting `[k v]`
  pairs / maps
- `require` with `:as` / `:refer` / `:refer :all`, a module search path
  and `LISPPATH`
- `cli` library — command-line option parsing modelled on
  clojure/tools.cli (`cli/parse-opts`), plus the `subs`, `starts-with?`
  and `ends-with?` string builtins it builds on
- `integrity` library — attest and verify lisp source: `fmt` (canonical
  formatting, as `lisp --fmt`), `sha2-256`, and deterministic Ed25519
  signatures (`ed25519-generate`, `ed25519-sign`, `ed25519-verify`)
- LSP/DAP improvements (signature help, hover docs, macro-aware
  stepping) under the `lispdebug` build tag
- Clojure-style docstrings on `defn`, and `call.Doc` for documenting Go
  builtins
- Robustness pass: malformed input now returns errors instead of
  panicking, a fixed `Future` data race, and READ/EVAL fuzzers
