# jig/lisp

Derived from [kanaka/mal](https://github.com/kanaka/mal) Go implementation of a Lisp interpreter. It is Clojure _inspired_.

This implementation is focused on _embeddability_ in Go projects. See [lisp main](./cmd/lisp) for an example on how to embed it in Go code. It includes a REPL, a debugger, a language server and a source code formatter for Visual Studio Code.

> **Learning the language?** [LANGUAGE.md](./LANGUAGE.md) is a single-file
> reference to the syntax and every builtin library, meant to be read
> whole (by a person or an LLM). The builtin reference there is generated
> from the interpreter itself, so it never drifts from the code.

It requires Go 1.25.

> **Upgrading?** See [CHANGELOG.md](./CHANGELOG.md) for behaviour changes
> that may need action — notably the error-message format changed since
> v0.2.24. Maintainers: see [RELEASING.md](./RELEASING.md) for how a tag
> is cut.

## Install

### Install the interpreter and REPL

You need to have Go installed and configured. Then run:

```bash
go install -tags debugger github.com/jig/lisp/cmd/lisp@latest
```

The `debugger` tag compiles in the LSP server, the DAP debugger and
coverage support; a hook check in the evaluator keeps its cost near zero
(<1% geomean) until a debugger actually attaches, so this is the build
you want for the CLI and the REPL. Omitting the tag produces a smaller,
hook-free evaluator that cannot debug — the right choice only when
embedding jig/lisp inside a Go program (a plain `import` of the module
gets this zero-overhead mode automatically).

### Development on Visual Studio Code

`jig/lisp` includes a VSCode extension that provides a debugger and a language server. See [./tools/vscode-lisp/README.md](./tools/vscode-lisp/README.md) for installation and usage instructions.

It provides an LSP (Language Server Protocol) server and a DAP (Debug Adapter Protocol) server, both included in the `lisp` binary when it is built with the `debugger` tag (the recommended install above). The extension spawns that same `lisp` binary — there is no separate debug binary.

Then build and install the VSCode extension (it is not published in the VSCode marketplace):

```bash
cd tools/vscode-lisp
npm install
npm run compile
npx @vscode/vsce package
code --install-extension vscode-lisp-*.vsix
```

### Test

To test the implementation use:

```bash
go test ./...
```

Tests actually validates the `step*.mal` files, from `kanaka/mal` and new tests for added functionality.

There are some benchmarks as well:

```bash
go test -benchmem -benchtime 5s -bench '^.+$' github.com/jig/lisp
```

### Testing lisp code (deftest/is/are)

The `test` library provides Clojure-style unit testing for lisp code
itself. Tests live in `*_test.lisp` files:

```clojure
(load-file "mylib.lisp")

(deftest double-works
  (is (= 4 (double 2)))
  (are [in out] (= out (double in))
    0 0
    5 10))
```

Run a directory (or a single file) with the CLI runner, which exits
non-zero on failure:

```bash
lisp --test ./tests               # human summary
lisp --test ./tests --test-json report.json
```

`(is (= expected actual))` reports both values on failure; a failing
`is` outside `deftest` throws, so it also works in plain scripts.
`(with-out-str …)` captures standard output as a string, Clojure-style,
so printing behaviour is testable too:

```clojure
(deftest greets
  (is (= "hello\n" (with-out-str (println "hello")))))
```

With
a `-tags debugger` build, add `--coverage cov.lcov` to write an lcov report of
the lisp lines executed (test files excluded), consumable by any lcov
tool and by the VS Code extension's **Coverage** test profile, which
paints covered/uncovered lines in the editor:

```bash
lisp --test ./tests --coverage cov.lcov
```

Coverage is recorded per form and mapped to lines; lines holding only
literal atoms (e.g. a lone keyword in an `if` branch) carry no source
position and are not counted as coverable.

### Changes respect to `kanaka/mal`

There almost 100 implementations on almost 100 languages available on repository [kanaka/mal](https://github.com/kanaka/mal).

Changes respect to [kanaka/mal](https://github.com/kanaka/mal):

- Using `def` instead of `def!`, `try` instead of `try*`, etc. symbols
- `atom` is multithread
- Tests executed using Go test library. Original implementation uses a `runtest.py` in Python to keep all implementations compatible. But it makes the Go development less enjoyable. Tests files are the original ones, there is simply a new `runtest_test.go` that substitutes the original Python script
- Some tests are actually in lisp (mal), using the macros commented in _Additions_ section (now only the test library itself). Well, actually not many at this moment, see "Test file specs" below
- Reader regexp's are removed and substituted by an ad-hoc scanner [jig/scanner](https://github.com/jig/scanner)
- `core` library moved to `lib/core`
- Using [chzyer/readline](https://github.com/chzyer/readline) instead of C `readline` for the mal REPL
- Multiline REPL
- REPL history stored in `~/.lisp_history` (instead of kanaka/mal's `~/.mal-history`)
- `(let () A B C)` returns `C` as Clojure `let` instead of `A`, and evaluates `A`, `B` and `C`
- `(do)` returns nil as Clojure instead of panicking
- `hash-map` creates maps or converts a Go object to a map if the marshaler is defined in Go for that object
- `reduce-kv` added
- `take`, `take-last`, `drop`, `drop-last`, `subvec` added


### Additions respect to `kanaka/mal`

- Errors return line position and stack trace
- `(range a b)` returns a vector of integers from `a` to `b-1`
- `(merge hm1 hm2)` returns the merge of two hash maps, second takes precedence
- `(unbase64 string)`, `(unbase64 byteString)`, `(str2binary string)`, `(binary2str byteString)` to deal with `[]byte` variables
- `(sleep ms)` sleeps `ms` milliseconds
- Support of `¬` as string terminator to simplify JSON strings. Strings that start with `{"` and end with `"}` are printed using `¬`, otherwise strings are printed as usual (with `"`). To escape a `¬` character in a `¬` delimited string you must escape it by doubling it: `¬Hello¬¬World!¬` would be printed as `Hello¬World`. This behaviour allows to not to have to escape `"` nor `\` characters
- `(json-decode {} ¬{"key": "value"}¬)` to decode JSON to lisp hash map
- `(json-encode obj)` JSON encodes either a lisp structure or a go. Example: `(json-encode (json-decode {} ¬{"key":"value","key1": [{"a":"b","c":"d"},2,3]}¬))`. Note that lisp vectors (e.g. `[1 2 3]`) and lisp lists (e.g. `(list 1 2 3)` are both converted to JSON vectors always. Decoding a JSON vector is done on a lisp vector always though
- `(hash-map-decode (new-go-object) ¬{"key": "value"}¬)` to decode hash map to a Go struct if that struct has the appropiate Go marshaler
- `(context (do ...))` provides a Go context. Context contents depend on Go, and might be passed to specific functions context compatible
- Unit-testing library (`deftest`, `is`, `are`, `with-out-str`) run with `lisp --test DIR`, which loads every `*_test.lisp` / `*_test.mal` file in `DIR` and runs the registered tests. See "Testing lisp code" below
- `web` library — a Ring-style HTTP/HTTPS server (`web-serve`, `web-router`, response helpers, middleware) with TLS, mTLS and Keycloak-compatible JWT verification. See [lib/web/README.md](./lib/web/README.md)
- `(read-password prompt)` reads a line from stdin with terminal echo disabled (for secrets); prompt goes to stderr
- Project compatible with GitHub CodeSpaces. Press `.` on your keyboard and you are ready to deploy a CodeSpace with mal in it
- `(assert expr & optional-error)` asserts expression is not `nil` nor `false`, otherwise it success returning `nil`
- Errors are decorated with line numbers
- `(rename-keys hm hmAlterKeys)` as in Clojure
- `(get-in m ks)` to access nested values from a `m` map; `ks` must be a vector of hash map keys
- `(uuid)` returns an 128 bit rfc4122 random UUID
- `(split string cutset)` returns a lisp Vector of the elements splitted by the cutset (see [./tests/stepH_strings](./tests/stepH_strings.mal) for examples)
- support of (hashed, unordered) sets. Only sets of strings or keywords supported. Use `#{}` for literal sets. Functions supported for sets: `set`, `set?`, `conj`, `get`, `assoc`, `dissoc`, `contains?`, `empty?`. `meta`, `with-meta` (see [./tests/stepA_mal](./tests/stepA_set.mal) and [./tests/stepF_mal](./tests/stepF_set.mal) for examples). `json-encode` will encode a set to a JSON array
- `update`, `update-in` and `assoc-in` supported for hash maps and vectors
- Go function `READ_WithPreamble` works like `READ` but supports placeholders to be filled on READ time (see [./placeholder_test.go](./placeholder_test.go) for som samples)
- Added support for `finally` inside `try`. `finally` expression is evaluated for side effects only. `finally` is optional
- Added `spew`
- Added `future`, and `future-*` companion functions from Clojure
- `type?` returns the type name string
- `go-error`, `unwrap` and `panic` mapping to Go's `errors.New/fmt.Errorf`, `Unwrap` and `panic` respectively
- `getenv`, `setenv` and `unsetenv` functions for environment variables
- `defn`, `wait` macros added (see [./tests/stepN_defn.mal.go](./tests/stepN_defn.mal) for an example of `defn` and `wait` macro usage, or go to Clojure documentation)
- Clojure-style docstrings: `(defn name "docstring" [params] body…)` attaches a docstring to the function, readable with `(doc name)` and surfaced by the LSP. Backwards compatible — a leading string is a docstring only when a parameter vector follows it
- Go builtins can be documented from the code that registers them with `call.Doc(env, name, arglist, doc)`; the docs travel with the value in the environment, so the LSP and `(doc name)` describe exactly the builtins an interpreter loads — including an embedder's own. Native-function `meta` stays `nil` (kanaka/mal compatible): documentation lives in dedicated fields, not metadata
- `partial` function added (see [./tests/stepN_defn.mal.go](./tests/stepN_defn.mal) for an example of `partial` usage, or go to Clojure documentation)
- `subs`, `starts-with?` and `ends-with?` string functions (Clojure-style; `subs` counts in Unicode code points)
- `spit`, the write counterpart of `slurp` (Clojure-style): `(spit filename s)` creates or truncates, `(spit filename s :append true)` appends
- `*FILE*` holds the absolute path of the script being executed (cf. Clojure's `*file*`), so a script can read its own source; unset in the REPL and `-e`
- `cli` library for command-line option parsing, modelled on [clojure/tools.cli](https://github.com/clojure/tools.cli) — `(cli-parse-opts *ARGV* specs)`. See [./lib/cli/README.md](./lib/cli/README.md)
- `integrity` library to attest and verify lisp source: `fmt` (canonical formatting, as `lisp --fmt`), `sha2-256`, and deterministic Ed25519 signatures (`ed25519-generate`, `ed25519-sign`, `ed25519-verify`). See [./lib/integrity/README.md](./lib/integrity/README.md)

## Embed jig/lisp in Go code

You execute `jig/lisp` from Go code and get results from it back to Go. Example from [./example_test/example_test.go](./example_test/example_test.go):

```go
func ExampleEVAL() {
    newEnv := env.NewEnv()

    // Load required lisp libraries
    for _, library := range []struct {
        name string
        load func(newEnv types.EnvType) error
    }{
        {"core mal", nscore.Load},
        {"core mal with input", nscore.LoadInput},
        {"command line args", nscore.LoadCmdLineArgs},
        {"concurrent", nsconcurrent.Load},
        {"core mal extended", nscoreextended.Load},
        {"system", nssystem.Load},
    } {
        if err := library.load(newEnv); err != nil {
            log.Fatalf("Library Load Error %q: %v", library.name, err)
        }
    }

    // parse (READ) lisp code
    ast, err := lisp.READ(`(+ 2 2)`, nil, newEnv)
    if err != nil {
        log.Fatalf("READ error: %v", err)
    }

    // eval AST
    result, err := lisp.EVAL(context.TODO(), ast, newEnv)
    if err != nil {
        log.Fatalf("EVAL error: %v", err)
    }

    // use result
    if result.(int) != 4 {
        log.Fatalf("Result check error: %v", err)
    }

    // optionally print resulting AST
    fmt.Println(lisp.PRINT(result))
    // Output: 4
}
```

### Embedding contract

Guarantees an embedder can rely on. The interpreter is meant to run
untrusted or hand-written Lisp (config files, a transmission format), so
the boundaries matter.

**Panics vs errors.** `READ`, `EVAL` and the `REPL*` helpers report
problems by returning an `error` (a `lisperror.LispError` carrying a
source position and a stack trace). Malformed input — bad syntax, wrong
arity, ill-formed special forms — returns an error and must never panic
the host process; if you can make a plain Lisp string panic `EVAL`,
that is an interpreter bug, please report it. Two things are *not*
covered by this: a builtin handed a Go value of the wrong dynamic type
by your own code, and unbounded **non-tail** recursion, which can
exhaust the Go stack (an unrecoverable fatal error, not a catchable
panic — `loop`/`recur` and tail calls run in constant stack and are
safe). The Lisp `(panic x)` builtin is a deliberate feature: it is
caught by the builtin call boundary and surfaces as an `error`, so it
does not crash the host either.

**Context.** `EVAL` takes a `context.Context`. Pass one with a deadline
(`context.WithTimeout`) to bound execution — `try` reserves 80% of the
remaining time for the body and 20% for `catch`/`finally`, and a
cancelled context stops evaluation with a timeout error. A `nil`
context is accepted and simply disables cancellation and timeouts; use
`context.TODO()` in tests, a real context in production.

**Concurrency.** An `Env` is internally synchronised (an `RWMutex`), so
concurrent reads and definitions are safe at the structure level. The
recommended pattern is one shared base env holding the loaded libraries
plus a per-goroutine child (`env.NewSubordinateEnv(base)`) for each
`EVAL`, so top-level `def`s don't leak between evaluations. Share
*mutable* state between goroutines through **atoms** (`atom`, `swap!`,
`reset!` — thread-safe), not by `def`-ing into a shared env. `future`
runs its body on its own goroutine; in `debugger` builds it detaches
from the debug session (see the roadmap for multi-thread debugging).

**Matching errors.** Since errors are decorated with a `file:line:`
prefix and a stack trace, match them with `strings.Contains` (or
`errors.Is` / `errors.As` against a `lisperror.LispError`), not string
equality.

### Create new Go builtins

You can create new Go builtins and register them in the environment.

Look at samples under [lib/](./lib/) libraries. In particular, look at [lib/core/](./lib/core) for a full example of a library of Go builtins.

Create a new library and then add it (follow example above):

```go
        ...
        {"core mal extended", nscoreextended.Load},
        {"system", nssystem.Load},
        {"my new library", nsmynewlibrary.Load},
```

## Debug in VSCode

The interpreter includes a DAP (Debug Adapter Protocol) server and an
LSP (Language Server Protocol) server, both enabled with the
`debugger` build tag, plus a VSCode extension in
[./tools/vscode-lisp](./tools/vscode-lisp). Together they provide:

- **Debugger (DAP)**: breakpoints (including conditional breakpoints and
  logpoints), exception breakpoints (stop where an error is raised),
  step over / in / out, call stack, locals, editing variables while
  paused, the return value of a stepped form, Debug Console evaluation
  (with watch and hover) and program output.
- **Language server (LSP)**: live parse diagnostics while you type,
  symbol completion (core library plus your `def`/`defn`), hover with
  definition signatures, the document outline (Ctrl+Shift+O,
  breadcrumbs), signature help while typing a call, find all references
  (Shift+F12), scope-aware rename (F2) of a symbol defined in the
  file or a local binding, and semantic highlighting. `(require
  "module")` forms are resolved statically, so definitions from
  required modules are known to completion, hover, signature help,
  go-to-definition (F12) and the unknown-symbol check. Extra module
  search directories for the editor can be set with the
  `lisp.languageServer.includeDirs` setting (the editor equivalent of
  `-i`).

### Debug a file

Add a launch configuration (`.vscode/launch.json`):

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "type": "lisp",
      "request": "launch",
      "name": "Debug current Lisp file",
      "program": "${file}",
      "stopOnEntry": true,
      // "cwd": "${workspaceFolder}"   // optional path for (require "module") resolution, defaults to the git root directory (following Go logic)
    }
  ]
}
```

Open a `.lisp` file, set breakpoints with <kbd>F9</kbd> and start with
<kbd>F5</kbd>. `println`/`prn` output appears in the Debug Console, and
while paused you can evaluate any expression there in the context of
the selected stack frame.

The language server needs no configuration: it starts automatically
when a `.lisp` file is opened (disable with the
`lisp.languageServer.enabled` setting).

See [./tools/vscode-lisp/README.md](./tools/vscode-lisp/README.md) for
extension settings and development notes.

## REPL

Once installed, you can run the REPL with:

```bash
lisp
```

Use <kbd>Ctrl</kbd> + <kbd>D</kbd> to exit Lisp REPL.

### Execute lisp program

```bash
lisp helloworld.lisp
```

Scripts can also be directly executable: a leading shebang line reads
as a comment (positions in error messages are unaffected, and
`lisp --fmt` preserves it verbatim). Use the portable `env` form:

```bash
cat > hello <<'EOF'
#!/usr/bin/env lisp
(println "hello" (first *ARGV*))
EOF
chmod +x hello
./hello world
```

In VS Code, extensionless files with a lisp shebang are recognised by
their first line and get highlighting, LSP and the debugger as usual.

### Execute inline expression

```bash
lisp -e "(+ 1 2)"
```

If both a script and `-e` are provided, the script executes first and the `-e` expression executes last. Only the `-e` result is printed.

`-e` cannot be combined with `--version` or `--test`.

A bit longer example:

```bash
lisp helloargs.lisp --eval '(do (println "evaled to" *ARGV*) 42)' first-arg second-arg
```

It will print:

```text
Hello Args:  (first-arg second-arg)
evaled to (first-arg second-arg)
42
```

### Preamble placeholders (-P/--preamble)

Scripts can reference `$NAME` placeholders, filled at read time — the
same mechanism Go embedders use through `READWithPreamble` (see
"L notation" and [./placeholder_test.go](./placeholder_test.go)). When
running a script from the CLI, values come from (later sources win):

1. Leading `;; $NAME <expr>` lines in the file itself (defaults)
2. `-P/--preamble` flags (repeatable)

```bash
lisp -P '$NUMBER 1984' -P '$NAME "world"' script.lisp
```

```lisp
;; $NUMBER 1                  ; default, overridden by -P
(println (* $NUMBER 2))
```

A placeholder with no value anywhere reads as `nil`. Source line
numbers are preserved, so breakpoints in the debugger keep matching;
under VSCode, assignments can be listed in the launch configuration:

```json
{ "type": "lisp", "request": "launch", "program": "${file}",
  "preamble": ["$NUMBER 1984"] }
```

### Pass script arguments that look like flags

Use `--` to stop flag parsing so script arguments are passed through:

```bash
lisp -- helloworld.lisp --foo --bar
```

It will print:

```text
Hello Args:  (--first-arg --second-arg)
("--first-arg" "--second-arg")
```

### Module loading with require

The optional `require` library loads modules by name through a search
path, independently of the process working directory (unlike
`load-file`, whose relative paths resolve against the cwd). Each module
is evaluated at most once, in its own environment, and its top-level
definitions are published under a namespace prefix (Clojure style):

```lisp
(require "geometry")                    ; loads geometry.lisp (do not add .lisp)
(geometry/area 2)                       ; definitions are qualified

(require "geometry" :as "g")            ; short alias
(g/area 2)

(require "geometry" :refer ["area"])    ; import selected names unqualified
(area 2)                                ; (geometry/area still available)
```

Module-internal references stay unqualified: the module's functions
close over the module environment, so `area` can call a sibling helper
without any prefix.

Resolution order:

1. Directories passed via `-i/--include` (repeatable)
2. `.<binary>/` under the enclosing Git repository root (for `lisp`: `.lisp/`)
3. Directories in the `<BINARY>PATH` environment variable, OS
   path-list separated (for `lisp`: `LISPPATH`, `:`-separated on Unix)
4. `$HOME/.config/<binary>/`
5. `/usr/local/share/<binary>/`

```bash
lisp -i ./lib -i ./vendor -e '(do (require "geometry") (geometry/area 2))'
LISPPATH=~/lisp-libs:/opt/lisp lisp script.lisp
```

`LISPPATH` sits below the project's `.lisp/` on purpose: a project's
own modules always win over whatever the shell environment points at.

Notes:

- Only relative module paths are allowed (no absolute paths, no `..`,
  no hidden path segments)
- Re-requiring an already loaded module does not re-evaluate it, but
  does publish new aliases or refers
- The library is optional: load it from Go with `nsrequire.Load("lisp")`
  (see [./cmd/lisp](./cmd/lisp))

# Licence

"jig/lisp" implementation is licensed under the MPL 2.0 (Mozilla Public License 2.0). See [LICENCE](./LICENCE) for more details.
