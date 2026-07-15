package reader_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/call"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/lisperror"
	"github.com/jig/lisp/printer"
	"github.com/jig/lisp/reader"
	"github.com/jig/lisp/types"
)

type Example struct {
	N int
	S string
}

type Embeddable struct {
	in *Embeddable
}

func new_embeddable(in ...Embeddable) (Embeddable, error) {
	if len(in) == 1 {
		return Embeddable{
			in: &in[0],
		}, nil
	}
	return Embeddable{}, nil
}

func new_example(n int, s string) (Example, error) {
	return Example{
		N: n,
		S: s,
	}, nil
}

func (ex Example) LispPrint(_Pr_str func(types.MalType, bool) string) string {
	return "«example " + _Pr_str(ex.N, true) + " " + _Pr_str(ex.S, true) + "»"
}

func (em Embeddable) LispPrint(_Pr_str func(types.MalType, bool) string) string {
	if em.in == nil {
		return "«embeddable»"
	}
	return "«embeddable " + _Pr_str(em.in, true) + "»"
}

func TestAdHocReaders(t *testing.T) {
	ns := env.NewEnv()
	if err := nscore.Load(ns); err != nil {
		t.Fatal()
	}
	call.Call(ns, new_example)
	call.Call(ns, new_embeddable, 0, 1)

	t.Run("example", func(t *testing.T) {
		ast, err := reader.Read_str(`«example 33 "hello"»`, types.NewCursorFile(t.Name()), nil, ns)
		if err != nil {
			t.Error(err)
		}
		switch ast := ast.(type) {
		case Example:
			if ast.N != 33 || ast.S != "hello" {
				t.Fatal()
			}
		default:
			t.Fatal()
		}
	})
	t.Run("error", func(t *testing.T) {
		ast, err := reader.Read_str(`«error "poum"»`, types.NewCursorFile(t.Name()), nil, ns)
		if err != nil {
			t.Error(err)
		}
		switch ast := ast.(type) {
		case lisperror.LispError:
			if ast.ErrorValue() != "poum" {
				t.Fatal()
			}
		default:
			t.Fatal()
		}
	})
	t.Run("error in error", func(t *testing.T) {
		ast, err := reader.Read_str(`«error «error "poum"»»`, types.NewCursorFile(t.Name()), nil, ns)
		if err != nil {
			t.Error(err)
		}
		switch ast.(type) {
		case lisperror.LispError:
			// currently internal LispError is not wrapped in another LispError so this is not tested
		default:
			t.Fatal()
		}
	})
	t.Run("embeddable", func(t *testing.T) {
		ast, err := reader.Read_str(`«embeddable»`, types.NewCursorFile(t.Name()), nil, ns)
		if err != nil {
			t.Error(err)
		}
		switch ast := ast.(type) {
		case nil:
			t.Fatal()
		case Embeddable:
			if ast.in != nil {
				t.Fatal()
			}
		default:
			t.Fatal()
		}
	})
	t.Run("embeddable in embeddable", func(t *testing.T) {
		ast, err := reader.Read_str(`«embeddable «embeddable»»`, types.NewCursorFile(t.Name()), nil, ns)
		if err != nil {
			t.Error(err)
		}
		switch ast.(type) {
		case Embeddable:
		case nil:
			t.Fatal()
		default:
			t.Fatal()
		}
	})
}

// TestRadixLiterals checks that 0x/0o/0b literals (optionally signed)
// read as *big.Int and round-trip through the printer, and that decimal
// stays a machine int while legacy leading-zero octal is rejected.
func TestRadixLiterals(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want string // via printer round-trip
	}{
		{`0x0`, "0x00"},
		{`0xCAFE_CAFE`, "0xCAFECAFE"},
		{`0o17`, "0x0F"},
		{`0b101`, "0x05"},
		{`0xFFFF_FFFF_FFFF_FFFF_FFFF`, "0xFFFFFFFFFFFFFFFFFFFF"}, // > 64 bits
		{`-0x01`, "-0x01"},
		{`-0xCAFE_CAFE`, "-0xCAFECAFE"},
	} {
		ast, err := reader.Read_str(tc.src, types.NewCursorFile(t.Name()), nil)
		if err != nil {
			t.Fatalf("%s: %v", tc.src, err)
		}
		if _, ok := ast.(*big.Int); !ok {
			t.Fatalf("%s: got %T, want *big.Int", tc.src, ast)
		}
		if got := printer.Pr_str(ast, true); got != tc.want {
			t.Fatalf("%s: printed %s, want %s", tc.src, got, tc.want)
		}
	}

	for _, src := range []string{`0`, `42`, `-42`} {
		ast, err := reader.Read_str(src, types.NewCursorFile(t.Name()), nil)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if _, ok := ast.(int); !ok {
			t.Fatalf("%s: got %T, want int", src, ast)
		}
	}

	// leading-zero octal is error-prone and rejected
	for _, src := range []string{`042`, `00`, `-042`} {
		if _, err := reader.Read_str(src, types.NewCursorFile(t.Name()), nil); err == nil || !strings.Contains(err.Error(), "leading-zero octal literal not supported") {
			t.Fatalf("%s: got %v, want leading-zero octal error", src, err)
		}
	}
}

// TestCommasAreWhitespace checks that commas separate tokens exactly
// like spaces, as in Clojure and kanaka/mal.
func TestCommasAreWhitespace(t *testing.T) {
	for src, want := range map[string]string{
		"(1 2, 3,,,,)": "(1 2 3)",
		"[1, 2, 3]":    "[1 2 3]",
		"{:a 1, :b 2}": "",  // multi-key print order unstable: parse-only
		",,,7,,,":      "7", // leading and trailing commas
	} {
		ast, err := reader.Read_str(src, types.NewCursorFile(t.Name()), nil)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if want == "" {
			continue
		}
		if got := printer.Pr_str(ast, true); got != want {
			t.Fatalf("%s: printed %s, want %s", src, got, want)
		}
	}
}

// TestMultilineString checks that a "…" literal may span physical lines
// (Clojure-style): the literal newline is kept verbatim in the string value,
// and an unterminated literal still errors at EOF.
func TestMultilineString(t *testing.T) {
	t.Run("literal newline is kept", func(t *testing.T) {
		ast, err := reader.Read_str("\"line1\nline2\"", types.NewCursorFile(t.Name()), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s, ok := ast.(string); !ok || s != "line1\nline2" {
			t.Fatalf("got %#v, want %q", ast, "line1\nline2")
		}
	})
	t.Run("printed back as a single escaped line", func(t *testing.T) {
		ast, err := reader.Read_str("\"a\nb\"", types.NewCursorFile(t.Name()), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := printer.Pr_str(ast, true); got != `"a\nb"` {
			t.Fatalf("Pr_str = %s, want %q", got, `"a\nb"`)
		}
	})
	t.Run("unterminated still errors", func(t *testing.T) {
		if _, err := reader.Read_str("\"abc\n", types.NewCursorFile(t.Name()), nil); err == nil {
			t.Fatal("expected an error for an unterminated string")
		}
	})
}
