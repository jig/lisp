// Package term provides plain-ANSI terminal styling for jig/lisp:
// term-style wraps a string in SGR escape codes chosen from a declarative
// options map, and helpers report whether color is appropriate and how
// wide the terminal is. No external dependencies and no TUI runtime —
// just colors and text attributes for formatted stdout.
//
// Color is emitted only when it makes sense: stdout must be a terminal,
// NO_COLOR must be unset and TERM must not be "dumb". CLICOLOR_FORCE=1
// forces color on regardless (useful when piping styled output). When
// color is off, term-style returns its input unchanged, so styled code
// degrades to plain text in pipes and logs.
package term

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	_ "embed"

	"golang.org/x/term"

	"github.com/jig/lisp/lib/call"
	. "github.com/jig/lisp/types"
)

//go:embed header-term.lisp
var headerTerm string

// HeaderTerm returns the lisp-defined part of the namespace (the color
// and attribute sugar), loaded by nsterm.
func HeaderTerm() string { return headerTerm }

// Load registers the term builtins in env.
func Load(env EnvType) {
	call.CallOverrideFN(env, "term-style", termStyle)
	call.CallOverrideFN(env, "term-color?", termColorQ)
	call.CallOverrideFN(env, "term-width", termWidth)

	call.Doc(env, "term-style", "[s opts]",
		"Wraps string s in ANSI codes per opts {:fg :bg :bold :dim :italic :underline :blink :reverse :strikethrough}; colors are keywords (:red, :bright-red, ...), 0-255 ints or \"#rrggbb\". Returns s unchanged when color is off.")
	call.Doc(env, "term-color?", "[]",
		"Whether styled output is enabled: stdout is a terminal, NO_COLOR is unset and TERM is not \"dumb\"; CLICOLOR_FORCE=1 forces it on.")
	call.Doc(env, "term-width", "[]",
		"The terminal width in columns, or 0 when stdout is not a terminal.")
}

// colorEnabled decides once whether to emit ANSI codes on stdout;
// colorEnabledStderr does the same for stderr (where the CLI writes its
// own audit output).
var (
	colorEnabled       = sync.OnceValue(func() bool { return colorDecisionFor(os.Stdout.Fd()) })
	colorEnabledStderr = sync.OnceValue(func() bool { return colorDecisionFor(os.Stderr.Fd()) })
)

func colorDecisionFor(fd uintptr) bool {
	if os.Getenv("CLICOLOR_FORCE") == "1" {
		return true
	}
	if _, present := os.LookupEnv("NO_COLOR"); present {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(fd))
}

// StderrStyle wraps s in an SGR sequence for the named color (as
// term-style's :fg accepts — "green", "red", …), bold when bold is true,
// honoring NO_COLOR / CLICOLOR_FORCE / TERM=dumb and whether stderr is a
// terminal. Returns s unchanged when color is disabled or the color name
// is unknown. It is the Go-facing counterpart of term-style, for the
// CLI's own colored output on stderr.
func StderrStyle(color string, bold bool, s string) string {
	if !colorEnabledStderr() {
		return s
	}
	code, ok := namedColors[color]
	if !ok {
		return s
	}
	seq := strconv.Itoa(code)
	if bold {
		seq = "1;" + seq
	}
	return "\x1b[" + seq + "m" + s + "\x1b[0m"
}

// StderrIsTerminal reports whether stderr is a terminal, so the CLI can
// print human-readable output there and machine-readable JSON when it is
// redirected to a file or pipe.
func StderrIsTerminal() bool {
	return term.IsTerminal(int(os.Stderr.Fd()))
}

// namedColors maps color keywords to their base SGR foreground code;
// backgrounds add 10.
var namedColors = map[string]int{
	"black": 30, "red": 31, "green": 32, "yellow": 33,
	"blue": 34, "magenta": 35, "cyan": 36, "white": 37,
	"bright-black": 90, "bright-red": 91, "bright-green": 92, "bright-yellow": 93,
	"bright-blue": 94, "bright-magenta": 95, "bright-cyan": 96, "bright-white": 97,
	"gray": 90, // alias of bright-black
}

// attrs maps boolean style options to their SGR code.
var attrs = []struct {
	name string
	code int
}{
	{"bold", 1}, {"dim", 2}, {"italic", 3}, {"underline", 4},
	{"blink", 5}, {"reverse", 7}, {"strikethrough", 9},
}

// colorCodes renders a color option (keyword, 0-255 int or "#rrggbb")
// as SGR codes; base is 38 for foreground, 48 for background.
func colorCodes(v MalType, base int) ([]string, error) {
	switch t := v.(type) {
	case string:
		if Keyword_Q(t) {
			name := t[len("ʞ"):]
			code, ok := namedColors[name]
			if !ok {
				return nil, fmt.Errorf("unknown color :%s", name)
			}
			if base == 48 {
				code += 10
			}
			return []string{strconv.Itoa(code)}, nil
		}
		if len(t) == 7 && t[0] == '#' {
			r, errR := strconv.ParseUint(t[1:3], 16, 8)
			g, errG := strconv.ParseUint(t[3:5], 16, 8)
			b, errB := strconv.ParseUint(t[5:7], 16, 8)
			if errR == nil && errG == nil && errB == nil {
				return []string{strconv.Itoa(base), "2",
					strconv.FormatUint(r, 10), strconv.FormatUint(g, 10), strconv.FormatUint(b, 10)}, nil
			}
		}
		return nil, fmt.Errorf("invalid color %q (use a keyword, 0-255 or \"#rrggbb\")", t)
	case int:
		if t < 0 || t > 255 {
			return nil, fmt.Errorf("color index %d out of range 0-255", t)
		}
		return []string{strconv.Itoa(base), "5", strconv.Itoa(t)}, nil
	default:
		return nil, fmt.Errorf("invalid color type %T", v)
	}
}

func termStyle(s string, opts MalType) (MalType, error) {
	hm, ok := opts.(HashMap)
	if !ok {
		return nil, fmt.Errorf("term-style: options must be a map, got %T", opts)
	}
	codes, err := sgrCodes(hm.Val)
	if err != nil {
		return nil, err
	}
	if len(codes) == 0 || !colorEnabled() {
		return s, nil
	}
	return "\x1b[" + strings.Join(codes, ";") + "m" + s + "\x1b[0m", nil
}

// sgrCodes translates the options map into an SGR code sequence. Unknown
// options error, so typos do not silently produce unstyled output.
func sgrCodes(opts map[string]MalType) ([]string, error) {
	known := map[string]bool{"fg": true, "bg": true}
	var codes []string
	for _, a := range attrs {
		known[a.name] = true
		if v, ok := opts[NewKeyword(a.name)]; ok {
			b, isBool := v.(bool)
			if !isBool {
				return nil, fmt.Errorf(":%s must be a boolean, got %T", a.name, v)
			}
			if b {
				codes = append(codes, strconv.Itoa(a.code))
			}
		}
	}
	if v, ok := opts[NewKeyword("fg")]; ok && v != nil {
		c, err := colorCodes(v, 38)
		if err != nil {
			return nil, err
		}
		codes = append(codes, c...)
	}
	if v, ok := opts[NewKeyword("bg")]; ok && v != nil {
		c, err := colorCodes(v, 48)
		if err != nil {
			return nil, err
		}
		codes = append(codes, c...)
	}
	for k := range opts {
		name := k
		if Keyword_Q(k) {
			name = k[len("ʞ"):]
		}
		if !known[name] {
			return nil, fmt.Errorf("term-style: unknown option :%s", name)
		}
	}
	return codes, nil
}

func termColorQ() (MalType, error) {
	return colorEnabled(), nil
}

func termWidth() (MalType, error) {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 0, nil
	}
	return w, nil
}
