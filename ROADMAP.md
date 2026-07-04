# Roadmap — pending DAP / LSP / require work

Backlog of agreed-but-unscheduled improvements, written down so they are
not forgotten. Effort and risk are estimates as of 2026-07: **surgical**
means additive and contained (safe to pick up any time), **invasive**
means it touches the interpreter hot path or the session model and
needs care.

## Quick wins (surgical, pick these first)

### ~~`LISPPATH` environment variable for require~~ (done)
Implemented as `<BINARY>PATH` (`LISPPATH` for the default binary),
OS path-list separated, placed *after* the git-root `.lisp/` so a
project's own modules always win over the shell env — the conservative
placement from the compatibility notes below.

### LSP: signature help
Show the expected parameters while typing a call, current parameter
highlighted. The definition analysis already extracts parameter vectors
(`definition.params`); needs the `signatureHelpProvider` capability and
a `textDocument/signatureHelp` handler. Entry point:
[lsp/server.go](lsp/server.go). *Effort: small. Impact: medium.*

### LSP: re-analyse open documents when required modules change
Today, editing `testlib.lisp` does not refresh the diagnostics of an
open `test.lisp` that requires it until the latter is touched. Register
for `workspace/didChangeWatchedFiles` and re-run `updateDocument` on
open docs. *Effort: small. Impact: medium — avoids stale diagnostics.*

## High value, moderate effort (still surgical)

### LSP: find references (Shift-F12)
List every use of a symbol. The scan in
[lsp/analyse.go](lsp/analyse.go) already collects call-head references;
it needs to also record non-head symbol occurrences with positions, and
a `textDocument/references` handler. *Effort: medium. Impact: high —
the most requested navigation feature after go-to-definition.*

### LSP: rename (F2)
Consistent rename across all uses. Builds directly on find references
(produce a `WorkspaceEdit` instead of a location list). **Less
innocuous than the rest of the LSP items: it edits user code, so wrong
matches corrupt sources** — needs conservative scope rules and tests
around shadowing (`let`/`fn` params). *Effort: medium (after
references). Impact: high.*

### DAP: conditional breakpoints and logpoints
Breakpoints firing only when a condition holds, or logging without
stopping. VSCode already sends `condition`/`logMessage` in
`setBreakpoints`; the server must evaluate the condition in the paused
frame's env — the `evaluate` machinery in
[debugadapter/server.go](debugadapter/server.go) is reusable, but the
evaluation happens inside the hook (debuggee goroutine), so watch for
re-entrancy. *Effort: medium. Impact: high for real debugging
sessions.*

## Invasive (plan before starting)

### DAP: exception breakpoints
Stop automatically where a lisp error is raised, before it unwinds.
Needs a hook point in EVAL's error path ([mal.go](mal.go)) and the
`exceptionBreakpointFilters` capability. **Touches the interpreter
error path.** *Effort: medium. Impact: high — today errors just print
after the fact.*

### DAP: `setVariable`
Edit a variable from the Variables pane while paused. Mutating the
paused env is easy (`env.Set`); the risk is semantic (shadowed scopes,
values shared through closures). *Effort: small-medium. Impact:
medium.*

### DAP: return value after step
Show the result of the just-executed form as a synthetic `(result)`
variable after F10/Shift-F11. Requires capturing EVAL results at frame
pop — **hot-path change in [mal.go](mal.go)** (currently results are
not recorded anywhere). *Effort: medium. Impact: medium.*

### DAP: multi-thread debugging (futures)
The big one, tracked since 2026-07-03: breakpoints and stepping inside
`(future …)`. Today futures are deliberately invisible
(`runtime.DetachThread` in
[lib/concurrent/concurrent.go](lib/concurrent/concurrent.go), guard in
[debugadapter/hook.go](debugadapter/hook.go)). Full support means a
`runtime.Thread` per goroutine, real DAP thread ids in `threads`,
`stopped`/`continued` events per threadId, and per-thread step state
replacing the single `state.mode` in
[debugadapter/state.go](debugadapter/state.go). *Effort: multi-day.
Impact: high for concurrent programs; zero for sequential scripts.*

## Cosmetic / large

### LSP: semantic tokens
Real semantic highlighting (macro vs function vs local binding vs
qualified symbol) instead of the TextMate grammar. Additive, but the
token-encoding protocol is fiddly. *Effort: medium-large. Impact:
cosmetic.*

### LSP: formatting
Canonical lisp indentation for Format Document. Needs a pretty-printer
with per-form indentation rules (`defn` vs `let` vs plain calls) —
opinionated and easy to get wrong against existing code styles.
*Effort: large. Impact: nice-to-have.*

## Backward compatibility (vital)

Ground rule for every item above: **anything touching the interpreter
(mal.go, lib/concurrent, runtime) goes behind the `lispdebug` build tag
so the release binary's semantics never change.** LSP/DAP protocol
additions are inherently safe: their only consumer is the VSCode
extension.

Item-specific notes:

- **`LISPPATH`** (done): the per-binary name (`<BINARY>PATH`) keeps it
  distinctive, and it sits after the git-root `.lisp/`, so setting it
  can never shadow a project's own modules.
- **Exception breakpoints**: the hook lands on EVAL's error path — the
  same area as the 2026-01-15 breaking changes (LispError format,
  try/catch/finally). It must observe, never wrap or alter propagated
  errors, or external `catch`/error-parsing code breaks.
- **Rename / formatting**: not a compat issue, but they rewrite user
  sources on explicit action; ship with conservative scope rules and
  tests before trusting them.
- Already-shipped delta to keep in mind: since the macro-expansion
  cursor fill (63c6647), errors raised inside macro-expanded code carry
  a `file:line:` prefix they previously lacked. Code matching error
  strings with `strings.Contains` (as the README recommends) is
  unaffected; strict equality matching is not.

## Suggested order

1. `LISPPATH` (minutes, immediate quality of life)
2. Module change watching + signature help (small, round out the LSP)
3. Find references → rename (the navigation pair)
4. Conditional breakpoints / logpoints (debugger power)
5. Exception breakpoints (first invasive one; plan the EVAL hook point)
6. Multi-thread futures (schedule real time for it)
