# regexp

Regular expressions for jig/lisp, backed by Go's [RE2](https://github.com/google/re2/wiki/Syntax)
engine. Following Clojure: `re-matches` is anchored (the whole string
must match), `re-find` is unanchored (matches anywhere).

## Writing patterns

Patterns are Go RE2 syntax. Write them as **raw `¬…¬` strings** to avoid
escaping — this is jig/lisp's equivalent of Clojure's `#"…"` literal:

```clojure
(re-find? ¬\d+¬ "abc123")     ; => true   — ¬…¬ keeps the backslash
;; a normal "…" string rejects \d, so "\\d+" would be needed instead
```

**RE2 is not PCRE**: there are no backreferences (`\1`) and no
lookahead/lookbehind. In exchange, matching is linear-time (no
catastrophic backtracking). Most everyday patterns are identical.

## Functions

| Function | Returns |
|---|---|
| `(re-pattern pattern)` | a compiled, reusable regex value (prints as `«regex …»`) |
| `(re-matches? re-or-pattern s)` | `true`/`false` — whole string matches (anchored) |
| `(re-find? re-or-pattern s)` | `true`/`false` — matches anywhere (substring) |
| `(re-matches re-or-pattern s)` | `nil`, the match string, or `[whole g1 g2 …]` (anchored) |
| `(re-find re-or-pattern s)` | `nil`, the match string, or `[whole g1 g2 …]` (unanchored) |

Every function accepts **either a compiled regex** (from `re-pattern`)
**or a raw pattern string** (compiled on use), so `re-pattern` is only
needed to reuse a pattern.

`re-matches`/`re-find` mirror Clojure: `nil` when there is no match, the
whole match string when the pattern has no capture groups, otherwise a
vector `[whole g1 g2 …]` — with an unmatched optional group as `nil`.

## Examples

```clojure
(re-matches? ¬\d+¬ "123")                 ; => true
(re-matches? ¬\d+¬ "12a")                 ; => false  (anchored)
(re-find?    ¬\d+¬ "abc123")              ; => true   (substring)

(re-find ¬(\d+)-(\d+)¬ "call 555-1234")   ; => ["555-1234" "555" "1234"]
(get (re-find ¬v(\d+)¬ "v42") 1)          ; => "42"   (a captured group)
(re-find ¬\d+¬ "abc123def")               ; => "123"  (no groups → the match)
(re-find ¬xyz¬ "abc")                     ; => nil    (no match)

(def word (re-pattern ¬[a-z]+¬))          ; reuse a compiled pattern
(re-find? word "HELLO world")             ; => true
```

## Loading

The `lisp` binary loads the namespace by default. Go embedders load it
with:

```go
import "github.com/jig/lisp/lib/regexp/nsregexp"

nsregexp.Load(env)
```

## Not yet implemented

`re-replace` / `re-replace-first`, `re-split`, and `re-seq` (all
matches) are planned for a later iteration.
