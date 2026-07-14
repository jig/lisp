package reader_test

import (
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
