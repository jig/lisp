# lib/lazy — lazy sequences for jig/lisp

Lazy sequences produce their elements **on demand** and memoise them, so they
can be infinite and traversed more than once cheaply. This makes idioms like
"the first 5 primes", "iterate until stable" or "take from an endless stream"
natural, without computing more than you consume.

The namespace is **self-contained**: producers and transformers return an
opaque `lazy-seq`; consumers walk it; `realize` forces a finite one into a
vector so the rest of core can work on the result. Any list or vector is
accepted wherever a `lazy-seq` is, so eager data flows in freely.

## Loading

`cmd/lisp` loads it by default. To embed it:

```go
import "github.com/jig/lisp/lib/lazy/nslazy"

nslazy.Load(ns) // registers the builtins; implemented entirely in Go
```

## Example

```clojure
;; the first 5 numbers > 3 from squaring the naturals, computed lazily
(realize
  (lazy-take 5
    (lazy-filter (fn [x] (> x 3))
      (lazy-map (fn [x] (* x x)) (lazy-range)))))
;; => [4 9 16 25 36]

;; powers of two, on demand
(realize (lazy-take 6 (lazy-iterate (fn [x] (* x 2)) 1)))
;; => [1 2 4 8 16 32]
```

`(lazy-range)` with no arguments is **infinite** — nothing runs until a
consumer pulls elements, and only as many as asked for are ever computed.

## Operations

### Producers (return a lazy-seq)

| Form | Meaning |
|------|---------|
| `(lazy-range)` | 0, 1, 2, … (infinite) |
| `(lazy-range end)` | 0 … end-1 |
| `(lazy-range start end)` | start … end-1 |
| `(lazy-range start end step)` | start, start+step, … |
| `(lazy-iterate f x)` | x, (f x), (f (f x)), … (infinite) |
| `(lazy-repeat x)` | x, x, x, … (infinite) |
| `(lazy-repeat n x)` | n copies of x |
| `(lazy-cycle coll)` | coll's elements, forever |

### Transformers (lazy-seq → lazy-seq)

| Form | Meaning |
|------|---------|
| `(lazy-map f coll)` | (f x) for each x |
| `(lazy-filter pred coll)` | items where `(pred x)` is truthy |
| `(lazy-remove pred coll)` | items where `(pred x)` is falsy |
| `(lazy-take n coll)` | first n items |
| `(lazy-drop n coll)` | all but the first n |
| `(lazy-take-while pred coll)` | leading run while `(pred x)` is truthy |
| `(lazy-drop-while pred coll)` | the rest after that leading run |

### Consumers and bridge

| Form | Meaning |
|------|---------|
| `(lazy-first coll)` | first element, or `nil` if empty |
| `(lazy-rest coll)` | a lazy-seq of everything after the first |
| `(lazy-nth coll n)` | the nth element (forces up to n) |
| `(lazy-reduce f init coll)` | left fold, forcing coll |
| `(realize coll)` | forces a **finite** lazy-seq into a vector |
| `(lazy-seq coll)` | views a list/vector as a lazy-seq |
| `(lazy-seq? x)` | whether x is a lazy-seq |

## Notes and limits

- **This namespace is a separate vocabulary.** `lazy-*` operations know about
  lazy sequences; the core `first`/`rest`/`map`/`take` do not. Cross back with
  `realize` (lazy → vector) and `lazy-seq` (vector/list → lazy). A future
  change could make the core seq functions lazy-aware; it would only *add*
  interop, not change anything here.
- **`realize` must be given a finite sequence.** On an infinite one it never
  returns — `lazy-take` (or `lazy-take-while`) first. It does honour context
  cancellation, so a cancelled evaluation stops it.
- **Memoised.** Each element is computed at most once, even across repeated
  traversals of the same lazy-seq.
- **Errors are lazy too.** If a mapping/predicate function throws, the error
  surfaces when that element is forced (by a consumer), not when the pipeline
  is built.
- A lazy-seq prints as `«lazy-seq»`; it is never forced just to be printed, so
  printing an infinite sequence is safe.
