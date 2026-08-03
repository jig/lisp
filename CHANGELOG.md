# Changelog

Notable changes to `jig/lisp`. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/). The project is
pre-1.0, so minor tags can carry behaviour changes; the ones that may
need action when upgrading are called out under **Changed** with a
migration note.

> **Version numbering note.** The 0.3.x tags were published
> prematurely and are **retracted** (`@latest` resolves to v0.2.24);
> the 0.4.x numbers are **skipped forever** (guarded by a `retract`
> range) because pre-release documents used "0.4" for the keyword
> work. The pre-keyword line ships as **v0.5.0** and the keyword
> migration as **v0.6.0**.

## 0.6.0 (unreleased)

On top of 0.5.0. The headline change is the keyword
representation — a compile-time break for Go embedders (see the
Migration section below); lisp code is almost entirely unaffected.

### ⚠️ Changed — keywords are a first-class type

Keywords are now a dedicated Go type, `types.Keyword`, instead of a
string carrying a `ʞ` (U+029E) prefix. Hash-map keys and set elements
are `map[types.MalType]…`, restricted to strings and keywords
(validated at construction). **Lisp code is unaffected** — `:foo`
reads, prints, compares and hashes exactly as before — but the type
split fixes real bugs:

- External data can no longer forge keywords: a JSON value `"ʞx"`
  used to *become* the keyword `:x` (`keyword?` true, `string?`
  false); it now stays a string. This closed a data-driven type
  confusion affecting every input path (`json-decode`, `lib/web`,
  `lib/sql`, files) and `--integrity` runs.
- `(json-encode {:a 1})` returns `{"a":1}` — the internal prefix no
  longer leaks (`{"ʞa":1}` before).
- A literal `ʞ` inside a string survives reading; the reader used the
  same rune as an unescape sentinel and silently corrupted it to `\`.
- `(seq :ab)` errors instead of splitting the keyword as a string.

**JSON round-trips are no longer lossless, by design**: keyword keys
and values serialise as their bare name, and `json-decode` keeps
producing plain string keys (as in Clojure), so
`(get (json-decode {} (json-encode {:a 1})) :a)` must become
`(get … "a")`. The old losslessness existed only because the prefix
leaked into the wire format.

### Added — scalar hash-map keys and set elements

Hash-map keys and set elements accept any **immutable scalar**: nil,
booleans, ints, floats, strings and keywords (previously strings and
keywords only). `#{1 2 3}`, `{1 "one"}`, `(assoc m 42 v)`,
`(contains? #{1.5} 1.5)` all work; entries and elements order by type
group (nil < booleans < ints < floats < strings < keywords) then
value, and `pr-str`/`str` now print maps and sets in that
deterministic order. Composite values (vectors, maps, sets), symbols
and radix big ints remain invalid keys — the first are not hashable,
the latter two only compare by identity — and error at construction.
`json-encode` of a map with non-string/keyword keys errors (JSON
objects require string keys).

### Added — `lib/log`, structured JSON logging

New `log` namespace: `log-debug` / `log-info` / `log-warn` /
`log-error` emit one structured record with alternating key/value
attributes — `(log-info "user created" :id 42)`. Keys are keywords or
strings (keywords lose their sigil); primitive values pass through,
anything else renders via the Lisp printer. The minimum level comes
from the `LOG_LEVEL` environment variable
(`debug`/`info`/`warn`/`error`, default `info`), read at load time.

Records never go to the screen and the destination is not
configurable by environment, by design: **systemd-journald** when its
socket is available — native fields `MESSAGE` / `PRIORITY` /
`SYSLOG_IDENTIFIER` (script basename) plus each attribute uppercased
(`:trace-id` → `TRACE_ID`); read back with `journalctl -t <script> -o
json` — and otherwise (macOS, no systemd) JSON lines appended to
`$XDG_STATE_HOME/lisp/<script>.log` (default
`~/.local/state/lisp/<script>.log`).

### ⚠️ Removed — `web-log` and `web-wrap-log`

Logging moves out of `lib/web` into `lib/log`. Migration:
`(web-log :info "msg" "k" v)` becomes `(log-info "msg" :k v)` (per
level), and `web-wrap-log` users write the one-liner middleware over
`log-info` themselves (example in `lib/web/README.md`).

### ⚠️ Changed — `(version)` moves to `lib/version` and identifies the embedder

The `version` builtin leaves core for a new `lib/version` +
`nsversion` namespace, and its hash-map gains `:main {:name
:version}` — the running program itself: an embedder's module (from
Go build info) or whatever the new `version.SetMain(name, ver)` Go
API sets (e.g. values injected with `-ldflags -X`). `--version`
prints that main identity first when it is not jig/lisp, so an
embedder's REPL built on `command.Execute` reports itself correctly;
jig/lisp and jig/scanner remain listed as components.

Migration (embedders): load the namespace —
`nsversion.Load(env)` — to keep the `(version)` builtin; core's
`Versions()` helper moved to `lib/version` unchanged.

### ⚠️ Changed — integrity moves to the `lisp-integrity` binary

`--integrity REF` and `--integrity-keys FILE` are **removed** from the
`lisp` binary. The new **`lisp-integrity`** binary is always-on: it
runs only script files (no REPL, `-e`, stdin, test/fmt/debug or
DAP/LSP modes) and verifies the script — and, in cascade, every file
loaded as code — against **`HEAD`**. There is no ref argument:
pinning a release is a property of the checkout
(`git checkout --detach v1.4.2`), and `HEAD` is fully static between
deployments.

- **Keys**: the flag is replaced by the fixed path
  `/etc/lisp/allowed_signers` (authorized_keys format, root-owned).
  Its presence activates the signature rule for every run on the
  host: `HEAD` must be signed by a listed key, itself or via a signed
  annotated tag pointing at it. The set is **verification-only** —
  unlike `--integrity-keys`, it never drives signing: the host holds
  public keys and needs no ssh-agent, and code is signed by whoever
  releases it, orthogonally to execution. (Git data signing is
  per-call via `git-commit`/`git-tag` `:sign`.)
- **Consent**: after the green block, `lisp-integrity` asks
  `proceed? [y/N]`; `-y` skips, and a non-TTY stdin without `-y`
  fails closed (systemd units say `lisp-integrity -y …`).
- **Attestation**: every run emits a start record (`COMMIT`, `REPO`,
  `TRACE_ID`, `ARGV`, `SIGNER`, `PROTECTED`) and an end record
  (`EXIT_CODE`) to journald — required on Linux, XDG state-file
  fallback on macOS with `PROTECTED=false` — and every `log-*` record
  carries the same run fields. A start without an end means abnormal
  termination. Never pass secrets on the command line: `ARGV` is
  recorded in append-only storage.
- **`(assert-integrity)`** now suggests `lisp-integrity` in its error
  and gains **`(assert-integrity :with-signature)`**, which throws
  unless the signature rule was applied.
- **`lisp-integrity --version`** reports version information like
  `lisp --version` (main identity first for embedders) and refuses to
  be combined with any other argument.
- **Signature visibility**: the green block always carries a signature
  line — `signer <comment> SHA256:<fingerprint>` when the rule
  applied, a yellow `signed no (no /etc/lisp/allowed_signers)` notice
  otherwise — and the start record always carries `SIGNED=true/false`
  (plus `SIGNER`/`SIGNER_FINGERPRINT` when signed), so
  consistency-only runs are visibly weaker and auditable
  (`journalctl SIGNED=false`).
- **`lisp-integrity --test DIR|FILE`** (with optional `--test-json`)
  runs a deftest suite under the same guarantees: the mode anchors on
  the repository enclosing the target and every test file — and, in
  cascade, everything it loads — is verified against `HEAD` as it
  loads; the attested exit code records which commit's tests passed.
- INTEGRITY.md is rewritten as the v2 specification (v1 differences
  are summarised at its end).

Migration: `lisp --integrity v1.4.2 s.lisp` becomes
`git checkout --detach v1.4.2 && lisp-integrity -y s.lisp`, and
`--integrity-keys FILE` becomes installing FILE at
`/etc/lisp/allowed_signers`.

### ⚠️ Changed — `reduce-kv` folds an associative collection

`reduce-kv` now matches Clojure: it takes a hash-map (calling
`(f acc k v)` for every entry) or a vector (`(f acc idx v)` per
element); `nil` folds to `init`. The previous form — a *flat* sequence
`k1 v1 k2 v2 …` — is gone. New entry accessors `key` and `val` read a
`[k v]` entry. Migration: `(reduce-kv f init (list k1 v1 …))` becomes
`(reduce-kv f init (apply hash-map (list k1 v1 …)))`, or restructure
the data as a map up front.

### Added — destructuring in `loop`/`recur`

`loop` bindings accept the same vector patterns as `fn` parameters and
`let` bindings (introduced in 0.5.0), re-destructuring on every
`recur`. Sequential destructuring now covers every binding form. Also
new: `key`/`val` entry accessors in `coreextended`.

### Fixed — preamble placeholder parse errors surface

`READWithPreamble` used to silently bind `nil` for a placeholder whose
value did not parse, deferring the failure to an unrelated error deep
inside the program; it now returns a read error naming the
placeholder.

### ⚠️ Changed — git signing is per-call (`:sign`), not a process switch

`git-commit` and annotated `git-tag` take a `:sign` option carrying an
allowed-signers list (authorized_keys-format **public** keys): the
runtime signs with a matching key from the ssh-agent — or from a
signer a Go embedder registered with `SetSigner` (e.g. a
`crypto.Signer` via `gossh.NewSignerFromSigner`) — resolved **before
anything is written**, so a missing key refuses up front and leaves
the repository untouched; a late signing failure still rolls back
(commit) or deletes the tag. Without `:sign` nothing signs: the
process-global signing policy (`SetSigningKeys`) is **removed**, so
code declares its own data-signing identities per operation — usually
different keys from `/etc/lisp/allowed_signers`, which verifies code
and never signs.

### Added — HTTP client: `web-request` / `web-get`

`lib/web` gains the outbound half, Ring-symmetric: `(web-get url)` and
`(web-request {:method :url :headers :body :timeout-ms})` return
`{:status :headers :body}` — the same shape a handler produces (as
clj-http does in Clojure). Network errors and timeouts throw; a
non-2xx status is returned, not thrown. Connections are pooled per
host (keep-alive). See `examples/httpclient.lisp` and
`examples/httpbench.lisp` (a miniature ApacheBench on futures +
loop/recur).

### Added — `sort` and `sort-by`

`(sort coll)` returns the elements of a list or vector as a sorted
list (stable), using the language's scalar order (nil < booleans <
numbers < strings < keywords) — the one hash-map printing already
uses. `(sort-by f coll)` sorts by `(f element)`; `f` may be a keyword
used as a map accessor, Clojure-style: `(sort-by :ms results)`.

### Added — `git-commits-since`

`(git-commits-since repo rev)` reports where a commit sits relative to
`HEAD`: a vector of the hashes stacked on top of `rev` in `HEAD`'s
first-parent history, newest first — `[]` when `rev` is `HEAD` itself,
`(count …)` its distance behind. Unknown revisions, and revisions off
the first-parent chain (side branches, the non-mainline side of a
merge), throw distinct errors. Combine with `git-show` to print the
pending commits.

### ⚠️ Removed — the `.state/` store

`state-save` and `state-load` are **removed**, together with the
state-commit rules of integrity mode: the code repository holds code
and configuration only, verified uniformly against `HEAD`, and the
running program needs no write access to the checkout. Mutable data
lives outside the code repository — a separate data repository managed
from lisp with `lib/git` (allowed-signers signing included), a
database via `lib/sql`, or plain files. Migration: replace
`(state-save name v)` / `(state-load name)` with your own persistence
outside the checkout; a `state-*` family over a separate data
repository may return in a future release.

### Fixed — signing failures are atomic

When a signing policy is installed (a Go embedder API), a signing
failure in `git-commit` restores HEAD and the index (the staged
changes survive for a retry), and one in annotated `git-tag` deletes
the just-created tag — instead of leaving unsigned objects behind.

### Migration (embedders — Go code using jig/lisp)

Scripts and lisp data files need no changes. Go code embedding the
interpreter recompiles against three deliberate compile-time breaks:

1. **`HashMap.Val` and `Set.Val` are renamed to `Items`**, with key
   type `map[types.MalType]MalType` / `map[types.MalType]struct{}`.
   Every literal and access breaks loudly on purpose: with the old
   field name kept, `hm.Val["ʞa"]` would have compiled against the new
   key type and silently returned `nil`.
2. **`"ʞ…"` string literals** used as keys or values must become
   `types.KW("…")`. Fixing the `Items` errors leads the compiler to
   each one.
3. **`types.NewKeyword` returns `types.Keyword`** (kept as a
   deprecated alias of the new constructor `types.KW`); assignments to
   `string` variables and comparisons against strings stop compiling.

Unchanged: `List.Val`/`Vector.Val`, `Symbol`, `REPL`/`EVAL`/`READ`,
`AddPreamble` (placeholder names stay `map[string]MalType`), and all
of `lnotation` (`L`/`S`/`LS`/`V`/`HM`/`SET`).
`marshalingexample_test.go` shows a typical embedder marshaler before
and after.

After the compiler is happy, audit for the changes that *don't* break
compilation:

- `case string:` branches in type switches that used to receive
  keywords: keywords no longer match — add `case types.Keyword:`.
- Custom builtins registered through `lib/call` with `string`
  parameters that lisp code calls with keywords: they now return a
  lisp error at call time (`reflect: Call using types.Keyword as type
  string`) — widen the parameter to `types.Keyword` or
  `types.MalType`.
- `key.(string)` assertions with `, ok` over map keys: they now fail
  (silently, if only `ok` is checked) for keyword keys.
- `types.Keyword_Q(someGoString)` is now always false; `%T` prints
  `types.Keyword`.
- Keyword-keyed JSON round-trips: switch post-decode lookups to
  string keys (see above).

A quick sweep finds the risky spots:

```sh
grep -rn 'ʞ' --include='*.go' .
grep -rn 'case string\|\.(string)\|Keyword_Q' --include='*.go' .
```

## 0.5.0 (unreleased)

Everything since v0.2.24 up to (excluding) the keyword-type
migration. Briefly tagged `v0.3.0` on 2026-07-24; that tag is
retracted — this content ships as v0.5.0.

### ⚠️ Changed — file I/O moved from `core` to `system`

`slurp`, `spit`, `load-file` and `load-file-once` are no longer part of
`core`; they moved to the `system` library. This makes `core` a pure
base with **no ambient filesystem access** — it keeps `eval`,
`read-program` and `read-string` (evaluate code you already hold as
data) but can no longer read a file to run it. File access is now an
explicit capability:

- **`+system`** (`nssystem.Load`) — raw host I/O: `slurp`/`spit`,
  `load-file`/`load-file-once`, plus `chdir`/`cwd`/`mkdtemp`/`remove-all`
  and the env-var builtins.
- **`+require`** — structured module loading; `require` reads modules
  itself (in Go), so it works without `system`.

**Not user-facing:** the `lisp` binary loads `system` by default, so
scripts using `slurp`/`spit`/`load-file` are unaffected, as is running a
script (which the CLI does via `load-file`). **Embedders** who loaded
only `core`/`nscore` and used those builtins must now also load
`nssystem.Load` (after core and its input layer — `load-file` builds on
`slurp-source` from system plus `read-program`/`eval` from core). The
`core.VerifySource` integrity hook moved to `system.VerifySource`.

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

### Added — seqable hash-maps and sets, sequential destructuring

Hash-maps and sets are now **seqable**, as in Clojure. A hash-map seqs
as a sequence of its entries — each a `[key value]` vector,
destructurable and `nth`-indexable — and a set as a sequence of its
elements. This makes `seq`, `first`, `rest`, `map`, `filter`, `reduce`,
`apply`, `cons`, `concat`, `vec`, `into` and lazy-seqs work over both,
so the idiomatic round-trip finally holds:

```clojure
(into {} (map (fn [[k v]] [k (- v)]) {:seconds 5 :days 2}))
;;=> {:seconds -5 :days -2}
```

Entries come out **sorted by key** (elements sorted, for sets). A
deterministic order is required for correctness, not just aesthetics:
Go randomises map iteration per call and `first`/`rest` each seq the
collection independently, so with an unstable order a `first`/`rest`
traversal would drop or repeat entries. `nth` on a hash-map or set
itself is rejected (they seq, but are not indexed collections —
Clojure errors here too).

Binding forms gained **sequential destructuring**: a vector pattern in
`fn` parameters or `let` bindings destructures the value positionally
and recursively (`(fn [[k v]] …)`, `(let [[a [b c]] x] …)`), missing
elements bind `nil`, extra elements are ignored, and `&` binds the
remainder as a list. Top-level `fn` arity checking stays strict.
`loop`/`recur` bindings remain symbol-only for now.

⚠️ One behaviour change: `(seq #{})` now returns `nil` (Clojure
parity), where it previously returned `()`. `(seq {})` is also `nil`.

### Other notable additions since v0.2.24

Non-breaking, for context (see `git log v0.2.24..` for the full list):

- `--integrity REF` — run a script if and only if it matches what is
  committed in its Git repository at REF (a commit hash, tag or
  branch): HEAD must be exactly REF (or a `.state/`-only descendant),
  the script must byte-match its committed blob, and the check
  cascades to every file evaluated as code — `require` modules and
  `load-file` targets — resolved inside the same repository (files
  resolving outside it are refused). `--integrity-keys FILE`
  additionally requires REF to be SSH-signed by a key listed in FILE
  (authorized_keys / `.pub` format, as `git-verify-commit` — not git's
  `allowed_signers`; a principal-first line is rejected, keys are
  matched by key with no expiry); the audit outcome is reported on
  stderr — a colored human-readable block (green ✓ / red ✗, one field
  per line) on an interactive terminal, a structured JSON line when
  redirected. That same key set also **drives signing**:
  while `--integrity-keys` is active, every `git-commit`, annotated
  `git-tag` and `state-save` made during the run is SSH-signed with the
  **ssh-agent** key listed there — no private key ever enters the
  process, and it fails closed if no listed key is loaded in the agent.
  (`git-commit`/`git-tag` therefore no longer take a `:sign {:key …}`
  option — signing is agent-driven only.) Under `--integrity-keys`,
  startup also **requires** every state commit between the ref and HEAD
  to be signed by a listed key, so the whole chain is signed by a
  trusted key (state authenticity, not just consistency). New builtins:
  `(assert-integrity)` throws unless the run is verified — so
  committed code can demand the flag — and returns the verified commit
  hash; `(state-save name value & [message])` / `(state-load name & [default])`
  persist program state as canonical lisp data under `.state/`
  (deterministic key order, width-aware wrapping, reader-macro sugar —
  `'x`, `` `x ``, `~x`, `~@x`, `@x`, `^meta form`),
  committing on save and verifying against HEAD on load, so state
  commits keep the original REF valid across restarts; `slurp-source`
  (which `load-file` now builds on) is `slurp` plus the code
  verification. This is an operational assurance against drift and
  uncommitted edits, not a security boundary against whoever can
  rewrite the repository or the binary. Full specification in
  `INTEGRITY.md`
- `regexp` library — regular expressions backed by Go's RE2 engine
  (linear-time; no backreferences or lookaround). Patterns are written
  as raw `¬…¬` strings (jig/lisp's equivalent of Clojure's `#"…"`).
  `re-pattern` compiles a reusable regex; `re-matches?` / `re-find?`
  return booleans (anchored / substring), `re-matches` / `re-find`
  return Clojure-style match data (`nil`, the match string, or a vector
  of capture groups). `re-seq` returns every match, `re-replace` /
  `re-replace-first` rewrite matches (with `$1` / `${name}` group
  references), and `re-split` breaks a string around matches. See
  `lib/regexp/README.md`
- `system` library grows directory/process primitives: `(chdir path)`
  (changes the process-global working directory — affects the `.state`
  store and git detection), `(cwd)` (the working directory as an
  absolute path), `(mkdtemp & [prefix])` (a fresh unique temp directory)
  and `(remove-all path)` (recursive delete, no error if absent). These
  let in-process tests drive `state-save`/`state-load` against a
  temporary git repo (via `mkdtemp` + `git-init` + `chdir`) instead of
  the real one
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
