# jig/lisp — VSCode extension

Language support and a debugger for the [`jig/lisp`](https://github.com/jig/lisp)
interpreter.

## Features

- Syntax highlighting for `.lisp` and `.mal` files.
- Debug Adapter Protocol client: launch, breakpoints (line), step
  over / in / out, continue, pause, call stack, scoped variables
  following the environment nesting (Locals / Closure / Globals, with
  the library-filled Globals collapsed by default), program output
  in the Debug Console, and expression evaluation (Debug Console,
  watch, hover) in the context of the selected stack frame.
- Language Server Protocol client: live parse diagnostics, symbol
  completion (core library plus document `def`/`defn`), hover with
  definition signatures, and document outline. `(require "module")`
  forms are followed statically, importing the module's definitions.
  Go-to-definition (F12) jumps to local definitions and into require'd
  module files; signature help lists a call's parameters as you type.
  Spawned as `lisp-debug --lsp` when a lisp document opens.
- Document formatting (a gofmt-style canonical layout, comments
  preserved) via the language server. Enabled on save by default for
  lisp files, or run **Format Document** (`⇧⌥F`) manually.

Find-references and rename are not part of this release.

## Known limitations

- The debugger models a single thread. Code running inside
  `(future …)` executes on a detached goroutine: it does not hit
  breakpoints and cannot be stepped (the rest of the session stays
  consistent). Full multi-thread debugging is future work.

## Requirements

The interpreter must be built with the `lispdebug` build tag — this is
the artefact that contains the DAP server. From the repo root:

```sh
go build -tags lispdebug -o lisp-debug ./cmd/lisp
sudo install lisp-debug /usr/local/bin/    # or anywhere on $PATH
```

By default the extension spawns `lisp-debug --dap <program>`. Override
the command via the `lisp.debugAdapter.command` setting.

## Building this extension

```sh
cd tools/vscode-lisp
npm install
npm run compile
```

For a `.vsix` bundle:

```sh
npx @vscode/vsce package
code --install-extension vscode-lisp-*.vsix
```

(Marketplace publishing is intentionally out of scope.)

## launch.json example

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
      "preamble": ["$NUMBER 1984"]
    }
  ]
}
```

`preamble` (optional) lists `$NAME <expr>` placeholder assignments,
forwarded to the interpreter as `--preamble` flags.

## Settings

| Setting                          | Default       | Purpose                                          |
| -------------------------------- | ------------- | ------------------------------------------------ |
| `lisp.debugAdapter.command`      | `lisp-debug`  | Path or name of the debug-build interpreter.     |
| `lisp.debugAdapter.extraArgs`    | `[]`          | Args inserted before `--dap` on every spawn.     |
| `lisp.languageServer.enabled`    | `true`        | Start the LSP client for lisp documents.         |
| `lisp.languageServer.command`    | `lisp-debug`  | Binary spawned as the LSP server (with `--lsp`). |
| `lisp.languageServer.includeDirs`| `[]`          | require search dirs for the editor (like `-i`).  |

The extension also sets, as defaults you can override, `editor.formatOnSave`
and `editor.defaultFormatter` for `[lisp]` documents so files are formatted
on save. To turn it off, add to your settings:

```json
"[lisp]": { "editor.formatOnSave": false }
```

## Development

The DAP wire is JSON-RPC over stdio. To inspect traffic, run the
interpreter manually:

```sh
lisp-debug --dap /path/to/script.lisp
```

and feed it framed messages (`Content-Length: N\r\n\r\n<json>`).
