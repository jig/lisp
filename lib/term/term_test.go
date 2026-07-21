package term_test

import (
	"context"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/lib/term"
	"github.com/jig/lisp/lib/term/nsterm"
	"github.com/jig/lisp/types"
)

func newEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	if err := nscore.Load(ns); err != nil {
		t.Fatal(err)
	}
	if err := nsterm.Load(ns); err != nil {
		t.Fatal(err)
	}
	return ns
}

func eval(t *testing.T, ns types.EnvType, src string) (types.MalType, error) {
	t.Helper()
	ast, err := lisp.READ(src, types.NewCursorFile(t.Name()), ns)
	if err != nil {
		t.Fatalf("READ %s: %v", src, err)
	}
	return lisp.EVAL(context.Background(), ast, ns)
}

func evalString(t *testing.T, ns types.EnvType, src string) string {
	t.Helper()
	v, err := eval(t, ns, src)
	if err != nil {
		t.Fatalf("EVAL %s: %v", src, err)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("EVAL %s: got %T, want string", src, v)
	}
	return s
}

// TestStyleForced covers the ANSI sequences with color forced on.
func TestStyleForced(t *testing.T) {
	t.Setenv("CLICOLOR_FORCE", "1")
	term.ResetColorCache()
	ns := newEnv(t)
	cases := []struct{ src, want string }{
		{`(term-style "x" {:fg :red})`, "\x1b[31mx\x1b[0m"},
		{`(term-style "x" {:fg :bright-red})`, "\x1b[91mx\x1b[0m"},
		{`(term-style "x" {:bg :blue})`, "\x1b[44mx\x1b[0m"},
		{`(term-style "x" {:bold true})`, "\x1b[1mx\x1b[0m"},
		{`(term-style "x" {:bold true :fg :green})`, "\x1b[1;32mx\x1b[0m"},
		{`(term-style "x" {:underline true :strikethrough true})`, "\x1b[4;9mx\x1b[0m"},
		{`(term-style "x" {:fg 208})`, "\x1b[38;5;208mx\x1b[0m"},
		{`(term-style "x" {:bg 17})`, "\x1b[48;5;17mx\x1b[0m"},
		{`(term-style "x" {:fg "#ff8800"})`, "\x1b[38;2;255;136;0mx\x1b[0m"},
		{`(term-style "x" {})`, "x"},
		{`(term-style "x" {:bold false})`, "x"},
		{`(term-red "x")`, "\x1b[31mx\x1b[0m"},
		{`(term-bold "x")`, "\x1b[1mx\x1b[0m"},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			if got := evalString(t, ns, tc.src); got != tc.want {
				t.Errorf("%s = %q, want %q", tc.src, got, tc.want)
			}
		})
	}

	if v, err := eval(t, ns, `(term-color?)`); err != nil || v != true {
		t.Errorf("(term-color?) = %v, %v; want true under CLICOLOR_FORCE", v, err)
	}
}

func TestStyleErrors(t *testing.T) {
	t.Setenv("CLICOLOR_FORCE", "1")
	term.ResetColorCache()
	ns := newEnv(t)
	for _, src := range []string{
		`(term-style "x" {:fg :no-such-color})`,
		`(term-style "x" {:fg "#zzzzzz"})`,
		`(term-style "x" {:fg 300})`,
		`(term-style "x" {:bold "yes"})`,
		`(term-style "x" {:typo true})`,
		`(term-style "x" nil)`,
	} {
		if _, err := eval(t, ns, src); err == nil {
			t.Errorf("%s did not error", src)
		}
	}
}

// TestStylePlain covers the off switch: NO_COLOR set (and no tty in the
// test binary anyway) must yield the input unchanged.
func TestStylePlain(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	term.ResetColorCache()
	ns := newEnv(t)
	if got := evalString(t, ns, `(term-style "x" {:fg :red :bold true})`); got != "x" {
		t.Errorf("styled under NO_COLOR: %q", got)
	}
	if v, err := eval(t, ns, `(term-color?)`); err != nil || v != false {
		t.Errorf("(term-color?) = %v, %v; want false under NO_COLOR", v, err)
	}
	// invalid options still error even when color is off
	if _, err := eval(t, ns, `(term-style "x" {:fg :no-such-color})`); err == nil {
		t.Error("invalid color did not error with color off")
	}
}

// TestStderrStyle covers the Go-facing helper the CLI uses for its own
// stderr output.
func TestStderrStyle(t *testing.T) {
	t.Setenv("CLICOLOR_FORCE", "1")
	term.ResetColorCache()
	if got, want := term.StderrStyle("green", true, "x"), "\x1b[1;32mx\x1b[0m"; got != want {
		t.Errorf("green bold = %q, want %q", got, want)
	}
	if got, want := term.StderrStyle("red", false, "x"), "\x1b[31mx\x1b[0m"; got != want {
		t.Errorf("red = %q, want %q", got, want)
	}
	if got := term.StderrStyle("no-such-color", false, "x"); got != "x" {
		t.Errorf("unknown color should be plain: %q", got)
	}
}

func TestStderrStylePlain(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	term.ResetColorCache()
	if got := term.StderrStyle("green", true, "x"); got != "x" {
		t.Errorf("under NO_COLOR should be plain: %q", got)
	}
}

func TestWidthNonTTY(t *testing.T) {
	ns := newEnv(t)
	v, err := eval(t, ns, `(term-width)`)
	if err != nil {
		t.Fatal(err)
	}
	if v != 0 {
		t.Errorf("(term-width) = %v, want 0 outside a terminal", v)
	}
}
