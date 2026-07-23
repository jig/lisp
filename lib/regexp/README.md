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
| `(re-seq re-or-pattern s)` | vector of every match, each shaped like `re-find`'s result (`[]` if none) |
| `(re-replace re-or-pattern s replacement)` | `s` with every match replaced |
| `(re-replace-first re-or-pattern s replacement)` | `s` with only the first match replaced |
| `(re-split re-or-pattern s & limit)` | vector of the pieces `s` splits into around matches |

Every function accepts **either a compiled regex** (from `re-pattern`)
**or a raw pattern string** (compiled on use), so `re-pattern` is only
needed to reuse a pattern.

`re-matches`/`re-find`/`re-seq` mirror Clojure: `nil` when there is no
match, the whole match string when the pattern has no capture groups,
otherwise a vector `[whole g1 g2 …]` — with an unmatched optional group
as `nil`. `re-seq` collects every non-overlapping match left to right
into a vector (jig/lisp is eager, so it is a vector, not a lazy seq).

### Replacement strings

In `re-replace` / `re-replace-first` the replacement is a template:
`$1` (or `${1}`) inserts a captured group, `${name}` a named group, and
`$$` a literal `$`. Use the braced form `${1}` when a digit is followed
by more word characters, so `${1}0` is "group 1 then a zero" rather than
the group named `10`. (Function replacements are not supported.)

### Splitting

`re-split` follows Go's `Split`: trailing empty pieces are **kept**
(`(re-split ¬,¬ "a,b,,")` → `["a" "b" "" ""]`). An optional integer
limit caps the number of pieces, the last one holding the remainder;
a negative or absent limit returns them all.

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

(re-seq ¬\d+¬ "a1 bb 22 c333")            ; => ["1" "22" "333"]
(re-seq ¬(\d)(\w)¬ "1a 2b")               ; => [["1a" "1" "a"] ["2b" "2" "b"]]

(re-replace ¬\d+¬ "a1b22c333" "#")        ; => "a#b#c#"
(re-replace ¬(\d+)-(\d+)¬ "555-1234" "${2}.${1}")  ; => "1234.555"
(re-replace-first ¬\d+¬ "a1b22" "#")      ; => "a#b22"

(re-split ¬,¬ "a,b,c")                     ; => ["a" "b" "c"]
(re-split ¬,¬ "a,b,c,d" 2)                 ; => ["a" "b,c,d"]
```

## Loading

The `lisp` binary loads the namespace by default. Go embedders load it
with:

```go
import "github.com/jig/lisp/lib/regexp/nsregexp"

nsregexp.Load(env)
```
