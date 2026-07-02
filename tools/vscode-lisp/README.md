# jig/lisp — VSCode extension

Language support and a debugger for the [`jig/lisp`](https://github.com/jig/lisp)
interpreter.

## Features

- Syntax highlighting for `.lisp` and `.mal` files.
- Debug Adapter Protocol client: launch, breakpoints (line), step
  over / in / out, continue, pause, call stack, locals, program output
  in the Debug Console, and expression evaluation (Debug Console,
  watch, hover) in the context of the selected stack frame.

LSP features (hover, completion, go-to-definition) are not part of
this release; they ship in a follow-up PR.

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
      "stopOnEntry": true
    }
  ]
}
```

## Settings

| Setting                          | Default       | Purpose                                          |
| -------------------------------- | ------------- | ------------------------------------------------ |
| `lisp.debugAdapter.command`      | `lisp-debug`  | Path or name of the debug-build interpreter.     |
| `lisp.debugAdapter.extraArgs`    | `[]`          | Args inserted before `--dap` on every spawn.     |

## Development

The DAP wire is JSON-RPC over stdio. To inspect traffic, run the
interpreter manually:

```sh
lisp-debug --dap /path/to/script.lisp
```

and feed it framed messages (`Content-Length: N\r\n\r\n<json>`).
