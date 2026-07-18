# bat syntax

Syntax highlighting for jig/lisp in [bat](https://github.com/sharkdp/bat). The grammar (`jig-lisp.sublime-syntax`) is hand-maintained in sync with the VS Code TextMate grammar (`tools/vscode-lisp/syntaxes/lisp.tmLanguage.json`) — bat's syntect engine only loads `.sublime-syntax` files. It also highlights `format`/`printf` `%verbs` inside strings, matching the LSP.

## Install

The syntax file is embedded in the `lisp` binary, so any machine with the binary can self-install without the repository:

```
lisp --install-bat-syntax
```

This writes `syntaxes/jig-lisp.sublime-syntax` into bat's config dir (`$BAT_CONFIG_DIR`, `bat --config-dir`, or `~/.config/bat`), adds `--map-syntax "*.lisp:jig/lisp"` to bat's `config` (bat bundles a generic "Lisp" syntax for `.lisp`; the mapping overrides it), and runs `bat cache --build`. The command is idempotent.

To undo: remove the syntax file and the `--map-syntax` line, then `bat cache --clear`.
