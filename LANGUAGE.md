# The jig/lisp language

A compact reference to the **syntax** and **builtin libraries** of
`jig/lisp`, written to be read start-to-finish by a person or pasted
whole into an LLM's context. It is Clojure-*inspired* and derived from
[kanaka/mal](https://github.com/kanaka/mal); if you know Clojure most of
this is familiar, and the [Differences & gotchas](#differences--gotchas)
section lists what is *not*.

For embedding the interpreter in Go, the REPL, the debugger and the
language server, see [README.md](./README.md). The builtin reference in
this file is generated from the interpreter's own documentation metadata
(see [Maintaining this document](#maintaining-this-document)).

---

## Running

```bash
lisp script.lisp            # run a file
lisp -e "(+ 1 2)"           # evaluate one expression and print it
lisp                        # REPL (Ctrl-D to exit)
lisp --test DIR             # run every *_test.mal under DIR
lisp --fmt script.lisp      # canonical formatting
```

Command-line arguments after the script are visible to it as the list
`*ARGV*`; `*FILE*` holds the running script's absolute path. Use `--` to
stop flag parsing, and `-P '$NAME value'` to fill `$NAME` placeholders
(see [README](./README.md#preamble-placeholders--p--preamble)).

## A 30-second tour

```clojure
;; def binds a name; defn defines a function (optional docstring).
(def pi 3)
(defn area
  "Area of a square of side s."
  [s]
  (* s s))

(area 4)                     ;=> 16
(doc area)                   ;=> "Area of a square of side s."

;; let introduces local scope; the last form is the value.
(let [a 2
      b (+ a 3)]
  (* a b))                   ;=> 10

;; Collections: list, vector, hash-map, set.
(def xs [1 2 3 4])
(map (fn [x] (* x x)) xs)    ;=> (1 4 9 16)
(reduce + 0 xs)              ;=> 10
(get {:name "Ada"} :name)    ;=> "Ada"

;; Threading macros read top-to-bottom.
(->> xs
     (filter even?)
     (map inc)
     (reduce + 0))           ;=> 8

;; Errors are values you throw and catch.
(try
  (throw {:code 42})
  (catch e (get e :code))    ;=> 42
  (finally (println "done")))
```

---

## Lexical syntax

### Atoms

| Kind | Examples | Notes |
| ---- | -------- | ----- |
| Integer | `42`, `-7`, `0x2a`, `0o52`, `0b101010` | The working numeric type. Parsed with Go's base-`0` rules. |
| Float | `1.5`, `-0.25` | Read as 32-bit floats, but the core arithmetic builtins (`+ - * /`) operate on **integers** — floats are second-class. |
| String | `"hello"`, `"tab\there"` | Double-quoted, C-style escapes. **May not contain a literal newline** — use `¬…¬` (below) for multi-line text. |
| Keyword | `:name`, `:http/get` | Interned constant, commonly used as a map key. |
| Symbol | `foo`, `+`, `my-fn`, `map?` | A name; evaluates to whatever it is bound to. |
| Boolean | `true`, `false` | |
| Nil | `nil` | The absent value. |

**`¬`-delimited strings.** A string may be delimited with `¬` instead of
`"`, which removes the need to escape `"` and `\`. This is ideal for
embedded JSON. Escape a literal `¬` by doubling it (`¬¬`). Strings that
look like JSON objects (`{"…"}`) print back with `¬` delimiters.

```clojure
(json-decode {} ¬{"key": "value", "n": [1, 2, 3]}¬)
```

### Collections

| Literal | Type | Notes |
| ------- | ---- | ----- |
| `(a b c)` | list | Also a call form when evaluated (see below). |
| `[a b c]` | vector | Indexed; the usual choice for data. |
| `{:k v :k2 v2}` | hash-map | Alternating key/value pairs. |
| `#{a b}` | set | **Strings and keywords only** (hashed, unordered). |

### Comments and reader macros

| Syntax | Meaning |
| ------ | ------- |
| `; …` | Comment to end of line. |
| `'form` | `(quote form)` — do not evaluate. |
| `` `form `` | `(quasiquote form)` — quote, but honour unquotes inside. |
| `~form` | `(unquote form)` — evaluate inside a quasiquote. |
| `~@form` | `(splice-unquote form)` — splice a sequence in. |
| `@ref` | `(deref ref)` — read an atom or future. |
| `^{…} form` | attach metadata to the following form. |

## Evaluation model

- A **symbol** evaluates to its binding; an unbound symbol is an error.
- A **list** `(f a b)` is a call: `f` is evaluated to a function or
  macro, the arguments are evaluated left to right (unless `f` is a
  macro or special form), and `f` is applied. The empty list `()`
  evaluates to itself.
- Vectors, maps and sets evaluate their elements and rebuild themselves.
- Everything else is self-evaluating.
- **Truthiness:** only `nil` and `false` are falsey. Everything else —
  including `0`, `""` and empty collections — is truthy.
- `def` binds in the current environment; `let`/`loop`/`fn` parameters
  introduce a new nested scope. Definitions are immutable values;
  shared mutable state goes through **atoms**.

---

## Reference

The rest of this section — every special form, and every builtin grouped
by the library that provides it — is generated from the interpreter.
Names marked ⁽ᵐ⁾ are macros. All the libraries below are loaded by the
`lisp` binary; a Go embedder chooses which to load.

<!-- BEGIN GENERATED BUILTINS -->
_This section is generated from the interpreter's own documentation metadata; do not edit it by hand — run `go generate ./...`._

### special forms

Handled directly by the evaluator (they control when their arguments are evaluated), so they are not ordinary functions.

| Name | Arguments | Description |
| ---- | --------- | ----------- |
| `def` | `[symbol value]` | Binds symbol to the evaluated value in the current environment. |
| `fn` | `[params & body]` | Creates an anonymous function with the given parameter vector and body. |
| `defmacro` | `[name fn]` | Binds name to a macro (a function expanded at evaluation time). |
| `let` | `[bindings & body]` | Evaluates body in a new scope with the vector's symbol/value bindings; returns the last body form. |
| `do` | `[& body]` | Evaluates each form in order and returns the value of the last. |
| `if` | `[test then else]` | Evaluates test; returns then when it is truthy, else otherwise (else is optional). |
| `quote` | `[form]` | Returns form unevaluated. |
| `quasiquote` | `[form]` | Like quote, but ~ (unquote) and ~@ (splice-unquote) inside form are evaluated. |
| `macroexpand` | `[form]` | Fully expands the macro call form without evaluating the result. |
| `try` | `[expr & clauses]` | Evaluates expr, dispatching to a (catch …) and/or (finally …) clause on error. |
| `catch` | `[binding & body]` | Inside try: binds the caught error and evaluates body. |
| `finally` | `[& body]` | Inside try: body is always evaluated for side effects, error or not. |
| `loop` | `[bindings & body]` | Like let, but a recursion point: recur in tail position rebinds the bindings and jumps back, in constant stack. |
| `recur` | `[& args]` | In tail position, rebinds the nearest recursion point — the enclosing loop's bindings, or the enclosing function's parameters — to args and iterates. |
| `context` | `[& body]` | Provides a Go context to the enclosed forms. |

### core

Arithmetic, collections, predicates, strings, JSON, errors — always loaded.

| Name | Arguments | Description |
| ---- | --------- | ----------- |
| `*` | `[& numbers]` | Product of its arguments (1 with none). |
| `+` | `[& numbers]` | Sum of its arguments (0 with none). |
| `-` | `[x & more]` | Subtracts the remaining arguments from x; negates x when alone. |
| `/` | `[x & more]` | Divides x by the remaining arguments; (/ x) is the inverse 1/x. |
| `<` | `[x & more]` | True when the arguments are monotonically increasing. |
| `<=` | `[x & more]` | True when the arguments are monotonically non-decreasing. |
| `=` | `[a b]` | Value equality. |
| `>` | `[x & more]` | True when the arguments are monotonically decreasing. |
| `>=` | `[x & more]` | True when the arguments are monotonically non-increasing. |
| `and ⁽ᵐ⁾` | `[& xs]` | Evaluates its arguments in order, returning the first falsey one, or the last (true with none). |
| `apply` | `[f & args]` | Calls f with args, the last of which is a sequence spread as arguments. |
| `assert` | `[expr & error]` | Returns nil when expr is truthy, otherwise raises error (or a default). |
| `assoc` | `[map key val & kvs]` | Copy of map with the given key/value pairs added or replaced. |
| `assoc-in` | `[coll keys val]` | Copy of coll with val set at the nested path keys. |
| `base64` | `[bytes]` | Encodes a byte string to a base64 string. |
| `binary2str` | `[bytes]` | Converts a byte string to a string. |
| `concat` | `[& seqs]` | Concatenates the sequences into one list. |
| `cond ⁽ᵐ⁾` | `[& xs]` | Takes test/expr pairs; yields the expr of the first truthy test, or nil. |
| `conj` | `[coll & items]` | Adds items to a collection (position depends on the collection type). |
| `cons` | `[x seq]` | Prepends x to seq. |
| `contains?` | `[coll key]` | Whether coll has the given key/index. |
| `count` | `[coll]` | Number of elements in coll. |
| `dec` | `[x]` | Returns x - 1. |
| `defn ⁽ᵐ⁾` | `[name & fdecl]` | Defines a named function (defn name [params] body…); an optional docstring may follow name. |
| `deref` | `[ref]` | Current value of an atom or other dereferenceable (also @ref). |
| `dissoc` | `[map & keys]` | Copy of map without the given keys. |
| `doc` | `[f]` | Documentation string of a function/macro, or nil. |
| `drop` | `[n coll]` | coll without its first n elements. |
| `drop-last` | `[n coll]` | coll without its last n elements. |
| `empty?` | `[coll]` | Whether coll has no elements. |
| `ends-with?` | `[s suffix]` | Whether string s ends with suffix. |
| `error-string` | `[err]` | The message of an error as a string. |
| `eval` | `[form]` | Evaluates a lisp form (AST) and returns its result. |
| `false?` | `[x]` | Whether x is boolean false. |
| `first` | `[coll]` | First element of coll, or nil. |
| `fn?` | `[x]` | Whether x is callable. |
| `gensym` | `[]` | Returns a fresh, hopefully-unique symbol like G__N (for writing hygienic macros). |
| `get` | `[coll key]` | Value at key in a map/vector, or nil. |
| `get-in` | `[coll keys]` | Nested value reached by following the vector of keys. |
| `go-error` | `[format & args]` | Creates a Go error from a format string and arguments. |
| `hash-map` | `[& kvs]` | Creates a hash-map from alternating key/value arguments. |
| `hash-map-decode` | `[factory json]` | Decodes JSON into a Go-backed hash-map. |
| `hash-set` | `[& items]` | Creates a set of the given items. |
| `inc` | `[x]` | Returns x + 1. |
| `json-decode` | `[factory json]` | Decodes a JSON string into a lisp value. |
| `json-encode` | `[obj]` | Encodes a lisp value (or Go object) to a JSON string. |
| `keys` | `[map]` | Vector of the map's keys. |
| `keyword` | `[name]` | Creates a keyword from a string. |
| `keyword?` | `[x]` | Whether x is a keyword. |
| `list` | `[& items]` | Creates a list of the given items. |
| `list?` | `[x]` | Whether x is a list. |
| `macro?` | `[x]` | Whether x is a macro. |
| `map` | `[f coll]` | Applies f to each element of coll, returning a list. |
| `map?` | `[x]` | Whether x is a hash-map. |
| `merge` | `[& maps]` | Merges maps left to right; later keys win. |
| `meta` | `[obj]` | Metadata attached to obj, or nil. |
| `new-error` | `[value & [cursor]]` | Creates a lisp error wrapping value, optionally at a source position. |
| `new-go-error` | `[message]` | Creates a Go error with the given message. |
| `nil?` | `[x]` | Whether x is nil. |
| `not` | `[a]` | Logical negation: false when a is truthy, true otherwise. |
| `not=` | `[a b]` | Logical negation of =. |
| `nth` | `[coll n]` | The element of coll at zero-based index n. |
| `number?` | `[x]` | Whether x is a number. |
| `or ⁽ᵐ⁾` | `[& xs]` | Evaluates its arguments in order, returning the first truthy one, or nil. |
| `panic` | `[value]` | Raises value as a Go panic. |
| `pr-str` | `[& args]` | Like str but with readable (quoted) representations. |
| `println` | `[& args]` | Prints its arguments (unquoted) separated by spaces, then a newline. |
| `prn` | `[& args]` | Prints its arguments (readable) separated by spaces, then a newline. |
| `range` | `[start end]` | Vector of integers from start to end-1. |
| `read-string` | `[string]` | Reads the first lisp form from string. |
| `rename-keys` | `[map keymap]` | Copy of map with keys renamed according to keymap. |
| `rest` | `[coll]` | All but the first element of coll, as a list. |
| `seq` | `[coll]` | coll as a sequence, or nil when empty. |
| `sequential?` | `[x]` | Whether x is a list or vector. |
| `set` | `[coll]` | Creates a set from the elements of coll. |
| `set?` | `[x]` | Whether x is a set. |
| `sleep` | `[ms]` | Sleeps for ms milliseconds. |
| `spew` | `[x]` | Dumps x to stderr in Go syntax for debugging; returns nil. |
| `split` | `[string cutset]` | Splits string on any character of cutset, returning a vector. |
| `starts-with?` | `[s prefix]` | Whether string s starts with prefix. |
| `str` | `[& args]` | Concatenates the printed representations of its arguments. |
| `str2binary` | `[string]` | Converts a string to a byte string. |
| `string?` | `[x]` | Whether x is a string. |
| `subs` | `[s start end]` | Substring of s from start to end (end optional), counted in Unicode code points. |
| `subvec` | `[vec start end]` | Sub-vector of vec from start to end (end optional). |
| `symbol` | `[name]` | Creates a symbol from a string. |
| `symbol?` | `[x]` | Whether x is a symbol. |
| `take` | `[n coll]` | First n elements of coll. |
| `take-last` | `[n coll]` | Last n elements of coll. |
| `throw` | `[value]` | Raises value as an error. |
| `time-format` | `[ms]` | Formats epoch milliseconds (as of time-ms) as an RFC 3339 UTC timestamp with millisecond precision. |
| `time-ms` | `[]` | Current time in milliseconds since the epoch. |
| `time-ns` | `[]` | Current time in nanoseconds since the epoch. |
| `time-parse` | `[string]` | Parses an RFC 3339 timestamp and returns epoch milliseconds (as of time-ms). |
| `true?` | `[x]` | Whether x is boolean true. |
| `type?` | `[x]` | Type name of x as a string. |
| `unbase64` | `[string]` | Decodes a base64 string to a byte string. |
| `unwrap-error` | `[err]` | The error wrapped inside err (Go's errors.Unwrap). |
| `update` | `[map key f & args]` | Copy of map with key updated to (f old & args). |
| `update-in` | `[coll keys f]` | Copy of coll with the nested value at keys replaced by (f old). |
| `uuid` | `[]` | A random RFC-4122 UUID string. |
| `vals` | `[map]` | Vector of the map's values. |
| `vec` | `[coll]` | coll as a vector. |
| `vector` | `[& items]` | Creates a vector of the given items. |
| `vector?` | `[x]` | Whether x is a vector. |
| `version` | `[]` | Interpreter build information as a hash-map. |
| `when ⁽ᵐ⁾` | `[condition & body]` | Evaluates body in an implicit do when condition is truthy; otherwise nil. |
| `with-meta` | `[obj m]` | Copy of obj with metadata m. |

Runtime variables: `*host-language*`

### core — input/output

Reading and writing files and stdin.

| Name | Arguments | Description |
| ---- | --------- | ----------- |
| `load-file` | `[file-path]` | Reads and evaluates the lisp file at file-path in the current environment; returns the value of its last form. |
| `load-file-once` | `[file-path]` | Like load-file, but never loads the same path twice. |
| `readline` | `[prompt]` | Prints prompt and reads a line from input. |
| `slurp` | `[filename]` | Reads a file and returns its contents as a string. |
| `spit` | `[filename s & opts]` | Writes string s to a file, creating or truncating it; with :append true, appends instead. |

### core — runtime variables

Values the runtime binds for a running script.


Runtime variables: `*ARGV*`

### concurrent

Atoms and futures for shared, thread-safe state.

| Name | Arguments | Description |
| ---- | --------- | ----------- |
| `atom` | `[value]` | Creates a mutable, thread-safe atom holding value. |
| `atom?` | `[x]` | Whether x is an atom. |
| `future ⁽ᵐ⁾` | `[& body]` | Runs body on its own goroutine, returning a future; deref (or @) blocks for its result. |
| `future-call` | `[fn]` | Runs fn on a new goroutine, returning a future for its result. |
| `future-cancel` | `[future]` | Requests cancellation of a running future. |
| `future-cancelled?` | `[future]` | Whether the future was cancelled. |
| `future-done?` | `[future]` | Whether the future has finished. |
| `future?` | `[x]` | Whether x is a future. |
| `new-atom` | `[atom]` | Constructor placeholder — atoms cannot be deserialized. |
| `new-future-call` | `[fn]` | Constructor placeholder — futures cannot be deserialized. |
| `reset!` | `[atom value]` | Sets the atom to value and returns it. |
| `swap!` | `[atom f & args]` | Atomically sets the atom to (f current & args). |

### coreextended

Higher-order helpers written in lisp (the prelude): reduce, map, partial, protocols…

| Name | Arguments | Description |
| ---- | --------- | ----------- |
| `-> ⁽ᵐ⁾` | `[x & xs]` | Thread-first: inserts each stage's result as the first argument of the next form. |
| `->> ⁽ᵐ⁾` | `[x & xs]` | Thread-last: inserts each stage's result as the last argument of the next form. |
| `abs` | `[n]` | Absolute value of n. |
| `benchmark ⁽ᵐ⁾` | `[expr n]` | Evaluates expr n times, returning a vector with the elapsed milliseconds of each run. |
| `benchmark*` | `[f n results]` | Runs f n times, collecting the elapsed milliseconds of each run (helper for benchmark). |
| `defprotocol ⁽ᵐ⁾` | `[proto-name & methods]` | Defines a protocol proto-name and its methods, dispatching on the argument's type. |
| `even?` | `[n]` | Whether n is even. |
| `every?` | `[pred xs]` | Whether (pred x) is truthy for every x in xs. |
| `extend` | `[type proto methods & more]` | Registers a type's method implementations for a protocol (return value is nil). |
| `filter` | `[pred xs]` | List of the items in xs for which (pred x) is truthy. |
| `find-type` | `[obj]` | Returns a keyword naming obj's type (overridable via :type metadata). |
| `foldr` | `[f init xs]` | Right fold: (f x1 (f x2 (.. (f xn init)))) over the elements of xs. |
| `identity` | `[x]` | Returns its argument unchanged. |
| `into` | `[to from]` | Pours every item of from into to using conj; the result keeps to's type. |
| `max` | `[a & more]` | Largest of one or more numbers. |
| `memoize` | `[f]` | Returns a caching version of f: results are stored by argument and reused. |
| `min` | `[a & more]` | Smallest of one or more numbers. |
| `mod` | `[a b]` | Modulo of a by b; the sign follows the divisor b. |
| `neg?` | `[n]` | Whether n is less than 0. |
| `odd?` | `[n]` | Whether n is odd. |
| `partial` | `[f & args]` | Returns a function that calls f with the given args plus any it is later called with. |
| `pos?` | `[n]` | Whether n is greater than 0. |
| `pprint` | `[obj]` | Pretty-prints a lisp value with indentation. |
| `quot` | `[a b]` | Integer quotient of a divided by b, truncated toward zero. |
| `reduce` | `[f init xs]` | Left fold: (f (.. (f (f init x1) x2) ..) xn) over the elements of xs. |
| `reduce-kv` | `[f init xs]` | Left fold over a sequence of key/value pairs: applies (f acc k v) across xs. |
| `rem` | `[a b]` | Remainder of (quot a b); the sign follows the dividend a. |
| `remove` | `[pred xs]` | List of the items in xs for which (pred x) is falsy. |
| `run-fn-for` | `[fn max-secs]` | Returns how many times the no-arg function fn runs in max-secs seconds (after a warm-up). |
| `satisfies?` | `[protocol obj]` | Whether obj's type has been extended to protocol. |
| `some` | `[pred xs]` | Returns the first truthy (pred x) over xs, or nil. |
| `take-while` | `[pred xs]` | Leading items of xs while (pred x) is truthy. |
| `time ⁽ᵐ⁾` | `[exp]` | Evaluates exp, prints the elapsed time, and returns its value. |
| `zero?` | `[n]` | Whether n equals 0. |

### assert

Minimal test library (run with `lisp --test DIR`).

| Name | Arguments | Description |
| ---- | --------- | ----------- |
| `assert-false ⁽ᵐ⁾` | `[name expr]` | Test case (for test-suite): passes when expr is falsey. |
| `assert-throws ⁽ᵐ⁾` | `[name expr]` | Test case (for test-suite): passes when evaluating expr raises an error. |
| `assert-true ⁽ᵐ⁾` | `[name expr]` | Test case (for test-suite): passes when expr is truthy and does not throw. |
| `test-suite` | `[name & assert-cases]` | Runs assert-* cases and prints PASS/FAIL for the named suite. |

### system

Access to the host environment (env vars, …).

| Name | Arguments | Description |
| ---- | --------- | ----------- |
| `getenv` | `[name]` | Value of the environment variable name, or nil. |
| `setenv` | `[name value]` | Sets the environment variable name to value. |
| `unsetenv` | `[name]` | Removes the environment variable name. |

### lazy

Lazy sequences.

| Name | Arguments | Description |
| ---- | --------- | ----------- |
| `lazy-cycle` | `[coll]` | Infinite lazy repetition of coll's elements. |
| `lazy-drop` | `[n coll]` | Lazy sequence of coll without its first n items. |
| `lazy-drop-while` | `[pred coll]` | Lazy sequence of coll after the (pred x)-truthy prefix. |
| `lazy-filter` | `[pred coll]` | Lazy sequence of the items in coll for which (pred x) is truthy. |
| `lazy-first` | `[coll]` | First element of coll, or nil if empty. |
| `lazy-iterate` | `[f x]` | Infinite lazy sequence x, (f x), (f (f x)), … |
| `lazy-map` | `[f coll]` | Lazy sequence of (f x) for each x in coll. |
| `lazy-nth` | `[coll n]` | The nth element of coll (forces up to n). |
| `lazy-range` | `[] [end] [start end] [start end step]` | Lazy sequence of numbers; with no args it is infinite from 0. |
| `lazy-reduce` | `[f init coll]` | Reduces coll with f starting from init (forces coll). |
| `lazy-remove` | `[pred coll]` | Lazy sequence of the items in coll for which (pred x) is falsy. |
| `lazy-repeat` | `[x] [n x]` | Lazy sequence of x, infinite or of length n. |
| `lazy-rest` | `[coll]` | Lazy sequence of coll without its first element. |
| `lazy-seq` | `[coll]` | Views a list or vector as a lazy-seq. |
| `lazy-seq?` | `[x]` | Whether x is a lazy-seq. |
| `lazy-take` | `[n coll]` | Lazy sequence of the first n items of coll. |
| `lazy-take-while` | `[pred coll]` | Lazy prefix of coll while (pred x) is truthy. |
| `realize` | `[coll]` | Forces a finite lazy-seq into a vector. |

### require

Module loading by name through a search path.

| Name | Arguments | Description |
| ---- | --------- | ----------- |
| `require` | `[module & [:as alias] [:refer [names]\|:all]]` | Loads a module once and publishes its definitions as module/name (or alias/name with :as; :refer imports selected names, or :refer :all imports them all, unqualified). |
| `resolve-require` | `[module]` | Resolves a module name to the absolute path of its file through the search path. |

Runtime variables: `*interpreter-binary*`

### sql

SQL database access.

| Name | Arguments | Description |
| ---- | --------- | ----------- |
| `sql-close` | `[db]` | Closes a database handle. |
| `sql-exec` | `[db-or-tx sql & params]` | Runs a statement (INSERT/UPDATE/DDL); returns {:rows-affected n :last-insert-id m}. |
| `sql-open` | `[driver dsn]` | Opens a database and returns a handle; driver is e.g. "sqlite" or "pgx". |
| `sql-query` | `[db-or-tx sql & params]` | Runs a query; returns a vector of maps, one per row, keyed by keyword column names. |
| `sql-query-one` | `[db-or-tx sql & params]` | Like sql-query but returns the first row map, or nil. |
| `sql-transact` | `[db fn]` | Runs (fn tx) inside a transaction: commits on success, rolls back if it throws. |
| `with-open ⁽ᵐ⁾` | `[binding & body]` | (with-open [name (sql-open …)] body…) binds name and guarantees sql-close when body finishes or throws. |
| `with-tx ⁽ᵐ⁾` | `[binding & body]` | (with-tx [tx db] body…) runs body in a transaction bound to tx: commits on success, rolls back on throw. |

### cli

Command-line option parsing (clojure/tools.cli style).

| Name | Arguments | Description |
| ---- | --------- | ----------- |
| `cli/parse-opts` | `[args specs]` | Parses args (typically *ARGV*) against a vector of option specs. Returns a map:   :options   parsed values keyed by :id   :arguments leftover positional arguments   :summary   generated help text   :errors    a vector of messages, or nil when all is well |
| `cli/summarize` | `[specs]` | Builds a help string from a vector of option specs, one line per option. |

### integrity

Attest and verify lisp source (formatting, hashing, Ed25519 signatures).

| Name | Arguments | Description |
| ---- | --------- | ----------- |
| `ed25519-generate` | `[]` | Generates an Ed25519 key pair, as a map {:public :private} of base64 strings. |
| `ed25519-sign` | `[private s]` | Signs string s with a base64 Ed25519 private key; returns the base64 signature (deterministic). |
| `ed25519-verify` | `[public s signature]` | Reports whether the base64 signature of string s verifies against the base64 Ed25519 public key. |
| `fmt` | `[s]` | Formats lisp source s into its canonical form (as lisp --fmt does); errors if s does not parse. |
| `sha2-256` | `[s]` | SHA2-256 digest of string s, as lowercase hex. |

### test

| Name | Arguments | Description |
| ---- | --------- | ----------- |
| `are ⁽ᵐ⁾` | `[argv expr & rows]` | Template assertion: substitutes each row of values for argv in expr and asserts every instance, e.g. (are [x y] (= x y) 2 (+ 1 1) 4 (* 2 2)). |
| `deftest ⁽ᵐ⁾` | `[name & body]` | Registers body as the test named name; run with the --test runner or (test/run-tests!). |
| `is ⁽ᵐ⁾` | `[form & msg]` | Asserts form is truthy; inside deftest it records the outcome, outside it throws on failure. (is (= expected actual)) reports both values. |
| `test/check!` | `[form thunk & msg]` | Records form's outcome in the running test; (is …) expands to this. |
| `test/check-eq!` | `[form expected-thunk actual-thunk & msg]` | Records an equality check with expected/actual reporting; (is (= a b)) expands to this. |
| `test/expand-are` | `[argv expr rows]` | Macro helper: expands an (are …) template into a do of is forms. |
| `test/register!` | `[name fn]` | Registers fn as the test named name; deftest expands to this. |
| `test/run-tests!` | `[]` | Runs every registered test and returns the results as data. |

⁽ᵐ⁾ = macro (arguments are not evaluated before the call).

<!-- END GENERATED BUILTINS -->

---

## Modules (`require`)

`require` loads a module by name through a search path and publishes its
top-level definitions under a namespace prefix:

```clojure
(require "geometry")                 ; loads geometry.lisp
(geometry/area 2)                    ; qualified

(require "geometry" :as "g")         ; alias
(g/area 2)

(require "geometry" :refer ["area"]) ; import unqualified
(area 2)
```

Each module is evaluated at most once, in its own environment. Only
relative module names are allowed. See the
[README](./README.md#module-loading-with-require) for the full
resolution order (`-i` dirs, the project's `.lisp/`, `LISPPATH`, …).

`load-file`, by contrast, evaluates a file in the *current* environment
and resolves relative paths against the working directory.

## Concurrency & errors

- **Atoms** (`atom`, `deref`/`@`, `swap!`, `reset!`) hold shared,
  thread-safe mutable state. Prefer them to re-`def`-ing globals.
- **Futures** (`future`, `future-call`, `future-cancel`, …) run a body
  on their own goroutine; `@f` blocks for the result.
- **Errors are values.** `(throw x)` raises any value; `(try expr (catch
  e …) (finally …))` handles it. `finally` runs for side effects
  whether or not an error occurred. `(panic x)` raises a Go panic that
  the builtin boundary still turns into a catchable error.
- Interpreter errors carry a `file:line:` prefix and a stack trace, so
  match them by substring, not equality.

When an embedder passes a `context.Context` with a deadline, `try`
reserves 80% of the remaining time for its body and 20% for
`catch`/`finally`. See [README → Embedding contract](./README.md#embedding-contract).

## Differences & gotchas

Mostly for readers coming from Clojure or from other mal implementations:

- **Names:** `def`, `try`, `catch` (not `def!`, `try*`). `defn`, `defmacro`, `fn`.
- **Numbers are integers.** `+ - * /` are integer operations; float
  literals are read but not first-class in arithmetic. There are no
  ratios or bignums.
- **Sets are limited:** only strings and keywords as members.
- **`let` returns its last body form** (Clojure-like) and evaluates
  every body form; `(do)` returns `nil` (it does not error).
- **`(range a b)`** returns a *vector* of integers `a … b-1`.
- **Only `nil` and `false` are falsey** — `0` and `""` are truthy.
- **Non-tail recursion** can exhaust the Go stack; use `loop`/`recur`
  for unbounded iteration (it runs in constant stack).
- **Strings can't span lines** with `"…"`; use `¬…¬`.
- `hash-map` also converts a Go object to a map when a marshaler is
  registered for it; JSON always encodes both lists and vectors as
  arrays, and decodes arrays into vectors.

---

## Maintaining this document

The [Reference](#reference) block between the
`<!-- BEGIN GENERATED BUILTINS -->` / `<!-- END GENERATED BUILTINS -->`
markers is generated from the live interpreter — the same doc metadata
the LSP and `(doc name)` use — so it never drifts from the code. Do not
edit it by hand. Regenerate it with:

```bash
go generate ./...
```

A test (`docgen.TestLanguageDocUpToDate`) fails in CI if the block is
stale. Builtin docs live next to the code that registers them
(`call.Doc` for Go builtins, `:doc` docstrings for lisp functions and
macros, and `docmeta.SpecialForms` for the special forms); update them
there and regenerate.
