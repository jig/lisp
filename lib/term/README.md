# term

Plain-ANSI terminal styling: colors and text attributes for formatted stdout, with no dependencies and no TUI runtime. Pairs with `print` and `format` for composing styled lines.

## Loading

The namespace is loaded by the `lisp` binary. Embedders load it with:

```go
import "github.com/jig/lisp/lib/term/nsterm"

nsterm.Load(env)
```

## Quick example

```clojure
(println (term-style "error:" {:fg :red :bold true})
         (format "%s (%d retries)" "connection refused" 3))

(println (term-style "deploy" {:fg "#ff8800" :underline true})
         (term-green "ok"))
```

## When color is emitted

`term-style` wraps its input in ANSI codes only when styled output makes sense; otherwise it returns the string unchanged, so styled code degrades to plain text in pipes and logs:

- stdout is a terminal, **and**
- `NO_COLOR` is unset, **and**
- `TERM` is not `dumb`.

`CLICOLOR_FORCE=1` forces color on regardless (e.g. when piping into a pager that renders ANSI). `(term-color?)` reports the decision.

## Colors

`:fg` and `:bg` accept three forms:

| Form | Example | ANSI |
|---|---|---|
| keyword | `:red`, `:bright-cyan`, `:gray` | 16-color SGR |
| int 0-255 | `208` | 256-color palette |
| hex string | `"#ff8800"` | 24-bit truecolor |

Named colors: `:black :red :green :yellow :blue :magenta :cyan :white`, their `:bright-*` variants, and `:gray` (alias of `:bright-black`).

Attributes (booleans): `:bold :dim :italic :underline :blink :reverse :strikethrough`.

Unknown options and invalid colors throw — also when color is off — so typos never pass silently.

## API

| Builtin | Arguments | Returns |
|---|---|---|
| `term-style` | `[s opts]` | s wrapped in ANSI codes per opts, or s unchanged when color is off |
| `term-color?` | `[]` | whether styled output is enabled |
| `term-width` | `[]` | terminal width in columns, 0 when stdout is not a terminal |
| `term-red` … `term-cyan`, `term-gray` | `[s]` | color sugar over `term-style` |
| `term-bold`, `term-underline` | `[s]` | attribute sugar over `term-style` |

## Limitations (v1)

- Styling only: no cursor movement, screen clearing or raw mode — a TUI layer would build on top of this.
- The color decision is made once per process (cached); changing `NO_COLOR` at runtime has no effect.
- `term-width` reads the size at call time but does not watch for terminal resizes.
