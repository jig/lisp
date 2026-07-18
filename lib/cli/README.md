# lib/cli — command-line option parsing for jig/lisp

A small option parser modelled on
[clojure/tools.cli](https://github.com/clojure/tools.cli), written in
**pure Lisp**. It turns a raw argument sequence (typically `*ARGV*`) into
a map of parsed options, leftover positional arguments, a generated help
string, and any errors.

## Loading

`cmd/lisp` loads it by default, so scripts can use `cli-parse-opts`
directly. To embed it:

```go
import "github.com/jig/lisp/lib/cli/nscli"

nscli.Load(ns) // evaluates the pure-Lisp header into the environment
```

Its primitives come almost entirely from **core** — `subs`,
`starts-with?`, `ends-with?`, `split`, `keyword`, `map`, `assoc`,
`get`, `conj`, `nth`, `hash-map`, `str`, … are Go builtins there, and
`defn`/`cond` come from core's basic header. The only functions it
needs from the **extended core** are `reduce` and `filter`.

So the minimal load sequence is `nscore.Load`, `nsconcurrent.Load`
(the extended core needs `atom`), `nscoreextended.Load`, then
`nscli.Load`. `cmd/lisp` already loads all of these.

## Usage

```lisp
(def specs
  [["-p" "--port PORT" "Port to listen on"
    :default 8080
    :parse-fn read-string
    :validate [(fn [n] (and (< 0 n) (< n 65536))) "must be 0..65535"]]
   ["-v" "--verbose" "Verbose output"]
   ["-h" "--help"    "Show help"]])

(def parsed (cli-parse-opts *ARGV* specs))

(get parsed :options)    ; => {:port 8080 :verbose true}
(get parsed :arguments)  ; => ["file.txt"]   (positionals)
(get parsed :errors)     ; => nil, or ["--port: must be 0..65535" …]
(get parsed :summary)    ; => a help string, one line per option
```

Read result fields with `get` (the `(:key m)` shorthand is not
supported).

## Option specs

Each spec is a vector `[short long desc & kv-opts]`:

| element | meaning |
|---------|---------|
| `short` | short flag, e.g. `"-p"` (or `nil`) |
| `long`  | `"--port PORT"` (takes an argument) or `"--verbose"` (boolean flag) |
| `desc`  | one-line description, used by the summary |

Optional key/value options:

| key | meaning |
|-----|---------|
| `:default val`        | value placed in `:options` when the flag is absent |
| `:parse-fn fn`        | applied to the raw string, e.g. `read-string` for a number |
| `:validate [pred msg]`| `pred` is called on the parsed value; `msg` is reported on failure (and the value is rejected) |
| `:id keyword`         | override the derived id (default: the long flag without `--`) |

## Subset (vs tools.cli)

Implemented: short/long flags, boolean flags, `:default`, `:parse-fn`,
`:validate`, `:id`, the `--` end-of-options separator, positional
arguments, and a generated summary.

Not yet implemented: the `--opt=val` joined form (use `--opt val`),
short grouping (`-abc`), `:update-fn` / `:multi` (repeated flags), and
in-order / sub-command parsing.
