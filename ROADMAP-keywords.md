# ROADMAP: first-class keyword type

> **Version numbering note (2026-07-25).** This document predates the
> renumbering: what it calls "0.4" ships as **v0.6.0**, and the
> pre-keyword line ("0.3.x" here) ships as **v0.5.0**. The 0.3.x tags
> are retracted and the 0.4.x numbers are skipped forever. See
> RELEASING.md.

Study (2026-07-24) on replacing the current keyword representation — a Go
`string` with the `ʞ` (U+029E) prefix, inherited from kanaka/mal — with a
dedicated Go type. Includes the related gap that `seq`, `map`, `filter` and
`reduce` do not accept hash-maps, since both changes touch the same map key
design.

## 1. Current representation and confirmed problems

Keywords are strings: `NewKeyword(s) = "ʞ" + s` (`types/types.go:67`).
`HashMap.Val` is `map[string]MalType` and `Set.Val` is `map[string]struct{}`,
so keywords and strings share one key space, disambiguated by the prefix.

All of the following were reproduced on `develop` (2026-07-24):

- **Type confusion from external data.** Any string entering the runtime with
  a leading `ʞ` *becomes* a keyword:

  ```clojure
  (let [s (get (json-decode {} ¬{"a":"ʞx"}¬) "a")]
    [(keyword? s) (string? s) (= s :x)])
  ;; => [true false true]
  ```

  This "keyword smuggling" affects every external input path (`json-decode`,
  `lib/web` request data, `lib/sql` results, file contents) and is relevant to
  `--integrity` mode, where inputs are untrusted.

- **`json-encode` leaks the prefix.** `core.go` marshals `hm.Val` directly:

  ```clojure
  (json-encode {:a 1 "b" 2})   ;; => ¬{"b":2,"ʞa":1}¬
  ```

- **Reader corrupts literal `ʞ` in strings.** The reader uses `ʞ` as a
  temporary sentinel while unescaping `\\` (`reader/reader.go:138-141`), so a
  literal `ʞ` in a string literal becomes a backslash:

  ```clojure
  (println "ʞ")        ;; prints \
  (println "a\\bʞc")   ;; prints a\b\c
  ```

  This bug is independent of the migration and can be fixed earlier by
  replacing the sentinel round-trip with a proper single-pass unescaper.

- **`seq` on a keyword splits it as a string**, `ʞ` included:

  ```clojure
  (seq :ab)   ;; => (: "a" "b")
  ```

- **Code noise.** 46 non-test Go sites do
  `strings.HasPrefix/TrimPrefix(k, "ʞ")` (lib/web, lib/sql, lib/term,
  lib/require, lsp, debugadapter, lisperror, printer, docgen…). With a real
  type these collapse into type switches. `String_Q` also pays a `HasPrefix`
  scan on *every* string check.

## 2. Related gap: sequence functions over hash-maps

Current state (reproduced):

- `(seq {:a 1})`, `(map f {:a 1})`, `(filter p {:a 1})`,
  `(reduce f init {:a 1})` — all fail (`seq requires string or list or
  vector`, `GetSlice called on non-sequence`).
- `keys`, `vals`, `count`, `into {}`, `conj` (entry form) already work.

The Clojure idiom `(into {} (map f m))` therefore fails at the `(map f m)`
half.

### Spec (agreed 2026-07-24)

Hash-maps become seqable as in Clojure: a sequence of entries, each entry a
2-element `Vector{[key value]}` (destructurable as `[k v]`, indexable with
`nth`). Single choke point: add a `case HashMap` to `types.GetSlice` that
builds the entry slice. `map`/`apply`/`first`/`rest`/`cons`/`concat`/`nth`
call `GetSlice` directly, and lisp-level `reduce`/`filter`/`into` sit on
`first`/`rest`/`empty?`/`conj`, so everything follows from that one case.
Additionally, `seq` (`lib/core`) gets its own `case HashMap` for Clojure
parity: `(seq {})` → `nil`, non-empty → `List` of entries.

Decisions baked into the spec:

- **Keys are returned exactly as stored** (today the canonical ʞ-string; no
  decode/re-encode), matching what `keys` already does. `(map (fn [[k v]] k)
  {:a 1})` yields `:a`; string keys yield strings.
- **Iteration order is sorted by key** (elements, for sets). The spec
  originally left the order unspecified (Clojure parity), but
  implementation showed that is *incorrect*, not merely unaesthetic: Go
  randomises map iteration per call, and `first`/`rest` each call
  `GetSlice` independently, so with an unstable order a `first`/`rest`
  traversal (lisp-level `reduce`/`filter`) drops or repeats entries.
  Clojure gets away with "unspecified" because its maps have a stable
  per-value iteration order; Go maps do not, so sorting is the cheapest
  way to buy the required stability. Tests should still compare
  order-insensitively. After the keyword migration the sort needs a
  `string|Keyword` ordering instead of plain `sort.Strings`.
- Optional follow-ups: `key`/`val` accessors; align `reduce-kv` with Clojure
  (`(f acc k v)` per entry — the current one expects a flat `k1 v1 …` seq,
  different semantics); `mapv` over maps if present.

**Spec gap found while implementing: sequential destructuring did not
exist.** The spec's objective assumed `(fn [[k v]] …)` worked; jig/lisp only
had `&` varargs, so the entry-destructuring idiom was impossible. Vector
patterns in binding forms were added as part of the same change (in
`env.bindPattern`, used by `NewSubordinateEnvWithBinds` for `fn`, and by
`let` via `env.Bind`): positional and recursive, missing elements bind
`nil`, extra are ignored, `&` binds the remainder as a list; top-level `fn`
arity stays strict; `loop`/`recur` bindings stay symbol-only for now.

**Status: implemented on branch `feat/seqable-hashmaps`** (GetSlice +
ConvertFrom + seq cases, `nth` rejection, destructuring, tests; CHANGELOG
entry). `(seq #{})` → `nil` was a behaviour change the step tests had to
adapt to.

### GetSlice caller audit (done 2026-07-24, all non-test callers)

Gains map support automatically, Clojure-parity semantics: `mAp`, `apply`,
`cons`, `concat`, `first`, `rest` (`lib/core`), `lazy.toSeq` (lazy-seqs over
maps), lisp-level `reduce`/`filter`/`into`.

Unaffected: `is_macro_call`/`macroexpand` (guarded by `Q[List]`), `Equal_Q`
(maps take the `HashMap` case before any `GetSlice`), `env.GenEnv` (binds
come from `fn` params, exprs are constructed lists), `NewHashMap`/`NewSet`
(map input now yields Vector entries which still fail their element type
checks, with a clearer message than before), `let`/`loop` binding forms
(entries fail the "non-symbol bind value" check).

Flagged:

- **`nth` divergence**: Clojure *errors* on `(nth {:a 1} 0)` (maps are not
  indexed); with the `GetSlice` case it would return a nondeterministic
  entry. Recommendation: explicitly reject `HashMap` in `nth`.
- **`vec` does not use `GetSlice`** — it uses `ConvertFrom`, which has no
  `HashMap` case, so `(vec {:a 1})` would still fail. Add the case there (or
  route `vec` through `GetSlice`) for Clojure parity (`(vec {:a 1})` →
  `[[:a 1]]`).
- **Sets are not `GetSlice`-able either** — `(map f #{…})` fails today for
  the same reason. Add `case Set` (elements as stored) in the same change;
  also `(seq #{})` currently returns an empty list, Clojure returns `nil`.
- Validation-by-error sites: `lib/test` (deftest arg shapes) and `lib/web`
  (route tables) currently rely on the `GetSlice` error to reject maps;
  after the change, misuse fails later with a less direct message. Cosmetic;
  no correct program changes behaviour.

### Interaction with the keyword migration

- The **keys-as-stored** principle means this feature adds *zero* new
  ʞ-aware code: the `GetSlice` case copies keys opaquely. After the type
  flip (option B), the same loop iterates `map[MalType]MalType` and entries
  carry real `Keyword` values — no lisp-visible change, one-line Go change.
  So the feature can safely ship **before** the migration, in 0.3.x.
- Entry sequences move keys out of the map into general data flow
  (`(map (fn [[k v]] …) m)`), multiplying the places where the key
  representation is observable — which strengthens the case for the real
  Keyword type, but does not depend on it.

### Tests to add (order-insensitive comparisons)

- `(into {} (map (fn [[k v]] [k (- v)]) {:seconds 5 :days 2}))` →
  `{:seconds -5 :days -2}` (the `backdate` pattern).
- `(seq {})` → `nil`; `(seq {:a 1})` → `([:a 1])`.
- `(first {:a 1})` → `[:a 1]`; `(count (rest {:a 1 :b 2}))` → `1`.
- `(reduce (fn [acc [k v]] (+ acc v)) 0 {:a 1 :b 2})` → `3`.
- `(filter (fn [[k v]] (> v 1)) {:a 1 :b 2})` → `([:b 2])`.
- `(map (fn [[k v]] k) {"x" 1 "y" 2})` → `("x" "y")` compared as set
  (string keys stay strings).
- `(count {:a 1 :b 2})` → `2` (regression guard; already works).
- `(nth {:a 1} 0)` → error (if the `nth` rejection is adopted).

## 3. Design options

The value type is easy: `type Keyword string` — comparable, zero-cost,
distinct in type switches. The real decision is the map/set key type:

| Option | Key type | Pros | Cons |
|---|---|---|---|
| A | keep `map[string]` + internal encoding at the map boundary | minimal churn; keeps the `faststr` fast path | key collision persists inside maps; still an encoding hack; breaks embedders anyway at the accessor level |
| **B (recommended)** | `map[MalType]MalType` / `map[MalType]struct{}` | removes the collision entirely; unlocks arbitrary-key maps and arbitrary-value sets (deferred Tier 2) in the same break | needs construction-time comparability validation; raw map ops ~20–50 % slower (micro); indexing with a plain string still compiles → must rename the field for a loud break |
| C | `map[MapKey]MalType`, `MapKey{Str string; KW bool}` | compile-time loud break; faster than B | closes the door on arbitrary keys; more verbose API |

Recommendation: **B**, with:

- `type Keyword string` as the value/key type for keywords.
- Construction-time validation of keys (comparable, initially restricted to
  `string|Keyword` to preserve current semantics; lifting the restriction
  later is then a non-breaking extension).
- **Rename `HashMap.Val` and `Set.Val`** (e.g. `Items`): with `map[MalType]`
  keys, legacy `hm.Val["ʞa"]` would still compile and silently return `nil`;
  renaming turns every embedder site into a compile error.

## 4. Implementation notes

- `reader`: `scanner.Keyword` token → `Keyword(token[1:])`. Also fix the
  unescape sentinel (§1), which is orthogonal.
- `printer`: `case Keyword:` prints `":" + name`; the string case loses its
  prefix branch. Sorting of map keys needs the `string|Keyword` ordering.
- `types.Equal_Q`: falls out of the default `a == b` case for `Keyword`.
- `json_encode`: keyword keys/values serialise as their bare name (Clojure
  behaviour) — a documented behaviour change; requires a `MarshalJSON` on
  `HashMap`/`Set` iterating typed keys, and on `Keyword` itself.
- `lib/call`: reflection dispatch already recovers mismatches into lisp
  errors. Builtins that receive keywords as `string` today (option parsers in
  web/term/sql/require/system, `keyword`, `time-add`…) get their signatures
  changed to `Keyword` or `MalType`. Mechanical.
- `lnotation`: `HM`/`SET` signatures follow the new key type; add a `KW`
  helper.
- `marshaler.HashMap` implementors (embedders) rewrite `"ʞa"` literals as
  `types.KW("a")`.
- Inventory: 68 `ʞ`/`ʞ` sites in Go (literal or escaped U+029E; 46 non-test) plus ~80
  `NewKeyword`/`Keyword_Q` call sites; all mechanical.

## 5. Performance

Microbenchmarks (linux/amd64, i7-8650U, Go 1.x from repo toolchain):

| Operation | current | typed |
|---|---|---|
| `keyword?` check | 8.0 ns (HasPrefix) | 7.3 ns (type assert) |
| keyword construction | 59 ns, 2 allocs (concat) | ~0 ns, 0 allocs |
| map get (32 keys) | 19.8 ns (`map[string]` fast path) | 26.1 ns (interface key, opt. B) / 23.5 ns (struct key, opt. C) |
| map build ×32 entries | 2.8 µs | 4.3 µs (B) / 5.0 µs (C) |

The cost is losing the `mapaccess_faststr` fast path: +20–30 % on raw map
gets, +50–80 % on inserts. Since hash-map operations are a small fraction of
EVAL time (dominated by env lookups and interface boxing), expected
end-to-end impact is low single digits. `string?` checks get cheaper across
the board and keyword construction becomes free. Validate with the MAL1 /
LoadSymbols benchmarks on a prototype branch before committing.

## 6. Compatibility and migration plan

**Lisp level: near transparent.** `:foo` reads, prints, compares and hashes
the same. Observable changes are the fixed bugs (§1), `json-encode` output
for keyword keys, and newly supported forms (§2).

**Go embedder level: unavoidable compile-time break.** No gradual path exists
within one binary — the representation is global. Breaks: `NewKeyword`
(return type), `"ʞa"` literals, `HashMap.Val`/`Set.Val` (renamed field, new
key type), `marshaler.HashMap` implementations, `lnotation.HM`/`SET`.

Plan:

- **0.3.x**
  - Fix the reader unescape sentinel bug (independent, no API change).
  - Ship seq/map/filter/reduce over hash-maps (and sets) per the spec in §2
    — confined to `GetSlice`/`seq`/`ConvertFrom`, keys returned as stored,
    so it does not add migration debt.
  - Announce in CHANGELOG/README that 0.4 replaces the keyword
    representation; mark `NewKeyword` as `// Deprecated:`.
- **0.4.0** — the flip, in one release:
  - `types.Keyword`, new map/set key design (option B), renamed fields,
    builtin signature updates, `types.KW` helper, migration guide
    (`"ʞa"` → `types.KW("a")`).
  - seq/map/filter/reduce over hash-maps (if not shipped in 0.3.x), on the
    new representation.
  - Same breaking window: arbitrary-value sets / arbitrary-key maps (Tier 2)
    if desired — they require exactly this key-type change.

## 7. Prototype status (branch `proto/keyword-type`, 2026-07-24)

Option B is implemented end to end: `types.Keyword` (+ `types.KW`
constructor, `NewKeyword` kept as a deprecated alias returning `Keyword`),
`HashMap.Val`/`Set.Val` renamed to **`Items`** with `map[MalType]…` keys,
`ValidKey` construction-time validation (string|Keyword), `KeyLess` total
order (strings first, then keywords — the grouping the sorted legacy
encoding produced). Full test suite green (31 packages, both tag sets);
MAL1 and a map-heavy lisp loop benchmark are within noise of `develop`,
as predicted in §5.

Findings and decisions made while implementing:

- **JSON round-trip is no longer lossless, by choice.** `json-encode`
  emits keyword keys/values as bare names and `json-decode` keeps
  producing plain string keys, so keyword→JSON→decode returns strings
  (Clojure parity). The old lossless round-trip existed only because the
  ʞ prefix leaked into the JSON — behaviour the marshaling step test
  itself marked `TODO`. Tests updated; a `:key-fn`-style keywordizing
  decode could be added later if wanted.
- The reader's `ʞ` unescape sentinel was replaced by a single-pass
  unescaper (the 0.3.x fix folded in): literal `ʞ` in strings survives.
- Builtin signature changes beyond the plan: `keyword` takes
  `MalType` (string or keyword), `contains?` takes a `MalType` key,
  `get` gained per-container key-type errors, web's `web-log` level is
  `MalType`. `AddPreamble` keeps `map[string]MalType` (placeholder names
  are not lisp keys).
- `(seq :ab)` now errors (keywords no longer fall into the string case) —
  the §1 bug fixed for free.
- The data printer (state-save canonical format) sorts by *printed* key,
  so state files keep their exact key order across the migration.
- gopls field-rename covered 33 files but missed tag-gated files
  (`debugadapter`, `command/coverage_debug.go`) — remember `-tags` builds
  when refactoring.

CHANGELOG now carries the 0.4 section with the embedder migration
guide. Tier-2 landed on `feat/tier2-collections`: `ValidKey` accepts
any immutable scalar (nil/bool/int/float/string/keyword), `KeyLess`
orders by type group then value, and `Pr_str` prints maps and sets in
that order (deterministic printing). Composites, symbols and big ints
stay invalid (unhashable / identity-compared). Found on the way:
`READWithPreamble` swallows placeholder parse errors (`item, _ :=`),
which had been silently defining `nil` for an unreadable placeholder in
`TestPassingLispDataFromGo` since before the migration — candidate for
the robustness backlog. Still pending for 0.4: LANGUAGE.md wording,
docgen/docs sweep, deciding whether `lnotation` gains keyword-key
helpers.

## 8. Conclusion

Feasible and desirable: the prefix encoding causes real correctness bugs
(data-driven type confusion, JSON leakage, reader corruption), not just
aesthetic debt. Performance cost is small and bounded; the API break is clean
if made loud at compile time. Next step: prototype branch implementing option
B, run the full test suite and MAL1 benchmarks, then schedule the 0.4 window.
