package printer_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jig/lisp/printer"
	"github.com/jig/lisp/types"
)

// TestPrStr pins the printed form of each value kind. These strings are
// a compatibility surface (embedders match on them), so this test is
// meant to catch accidental format changes, not just raise coverage.
func TestPrStr(t *testing.T) {
	cases := []struct {
		name string
		in   types.MalType
		want string
	}{
		{"nil", nil, "nil"},
		{"int", 42, "42"},
		{"symbol", types.Symbol{Val: "foo"}, "foo"},
		{"keyword", "ʞfoo", ":foo"},
		{"list", types.List{Val: []types.MalType{1, 2, 3}}, "(1 2 3)"},
		{"vector", types.Vector{Val: []types.MalType{1, 2, 3}}, "[1 2 3]"},
		{"empty-list", types.List{}, "()"},
		{"nested", types.List{Val: []types.MalType{types.Vector{Val: []types.MalType{1}}, 2}}, "([1] 2)"},
		{"hashmap-1", types.HashMap{Val: map[string]types.MalType{"ʞa": 1}}, "{:a 1}"},
		{"set-1", types.Set{Val: map[string]struct{}{"ʞa": {}}}, "#{:a}"},
		{"go-error", errors.New("boom"), `«go-error "boom"»`},
		{"malfunc", types.MalFunc{
			Params: types.Vector{Val: []types.MalType{types.Symbol{Val: "x"}}},
			Exp:    types.List{Val: []types.MalType{types.Symbol{Val: "x"}}},
		}, "(fn [x] (x))"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := printer.Pr_str(tc.in, true); got != tc.want {
				t.Fatalf("Pr_str = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestPrStrStringEscaping pins string quoting in readable mode and the
// raw passthrough in non-readable mode.
func TestPrStrStringEscaping(t *testing.T) {
	in := "a\"b\\c\nd"
	if got, want := printer.Pr_str(in, true), `"a\"b\\c\nd"`; got != want {
		t.Fatalf("readable = %q, want %q", got, want)
	}
	if got, want := printer.Pr_str(in, false), in; got != want {
		t.Fatalf("non-readable = %q, want %q", got, want)
	}
}

// TestPrStrJSONString pins the ¬-delimited form for JSON-looking strings
// and its ¬-doubling escape.
func TestPrStrJSONString(t *testing.T) {
	if got, want := printer.Pr_str(`{"k":"v"}`, true), `¬{"k":"v"}¬`; got != want {
		t.Fatalf("json string = %q, want %q", got, want)
	}
	if got, want := printer.Pr_str(`{"k":"a¬b"}`, true), `¬{"k":"a¬¬b"}¬`; got != want {
		t.Fatalf("json ¬-escape = %q, want %q", got, want)
	}
}

// TestPrStrPointer covers the pointer-dereferencing default branch.
func TestPrStrPointer(t *testing.T) {
	x := 5
	if got := printer.Pr_str(&x, true); got != "5" {
		t.Fatalf("*int = %q, want 5", got)
	}
	var p *int
	if got := printer.Pr_str(p, true); got != "nil" {
		t.Fatalf("nil *int = %q, want nil", got)
	}
}

// TestPrList covers Pr_list joining directly.
func TestPrList(t *testing.T) {
	got := printer.Pr_list([]types.MalType{1, 2, 3}, true, "<", ">", ",")
	if got != "<1,2,3>" {
		t.Fatalf("Pr_list = %q", got)
	}
}

// TestPrStrHashMapMulti keeps the multi-entry case non-flaky: map order
// is unspecified, so assert structure, not exact order.
func TestPrStrHashMapMulti(t *testing.T) {
	got := printer.Pr_str(types.HashMap{Val: map[string]types.MalType{"ʞa": 1, "ʞb": 2}}, true)
	if !strings.HasPrefix(got, "{") || !strings.HasSuffix(got, "}") {
		t.Fatalf("not brace-wrapped: %q", got)
	}
	for _, part := range []string{":a 1", ":b 2"} {
		if !strings.Contains(got, part) {
			t.Fatalf("%q missing %q", got, part)
		}
	}
}
