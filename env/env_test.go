package env

import (
	"testing"

	"github.com/jig/lisp/types"
)

func TestEnv(t *testing.T) {
	year := types.Symbol{Val: "year"}
	ns := NewEnv()
	ns.Set(year, 1984)
	res, err := ns.Get(year)
	if err != nil {
		t.Fatal(err)
	}
	if res.(int) != 1984 {
		t.Fatal()
	}
}

func TestSubordEnv(t *testing.T) {
	year := types.Symbol{Val: "year"}
	ns := NewEnv()

	s1env := NewSubordinateEnv(ns)
	s1env.Set(year, 1984)

	s2env := NewSubordinateEnv(ns)
	s2env.Set(year, 1985)

	res, err := s1env.Get(year)
	if err != nil {
		t.Fatal(err)
	}
	if res.(int) != 1984 {
		t.Fatal()
	}

	res2, err := s2env.Get(year)
	if err != nil {
		t.Fatal(err)
	}
	if res2.(int) != 1985 {
		t.Fatal()
	}

	if _, err := ns.Get(year); err == nil {
		t.Fatal("should not find symbol")
	}
}

// completionSet collects rune-suffix matches from Symbols into a string set,
// reattaching the prefix so the test assertions read naturally.
func completionSet(prefix string, lines [][]rune) map[string]int {
	out := map[string]int{}
	for _, ln := range lines {
		out[prefix+string(ln)]++
	}
	return out
}

func TestSymbolsCurrentScopePrefixMatch(t *testing.T) {
	ns := NewEnv().(*Env)
	ns.Set(types.Symbol{Val: "foo-bar"}, 1)
	ns.Set(types.Symbol{Val: "foo-baz"}, 2)
	ns.Set(types.Symbol{Val: "qux"}, 3)

	got := completionSet("foo-", ns.Symbols(nil, "foo-"))
	if got["foo-bar"] == 0 || got["foo-baz"] == 0 {
		t.Errorf("expected foo-bar and foo-baz in completions, got %v", got)
	}
	if got["qux"] != 0 {
		t.Errorf("non-matching symbol qux must not appear, got %v", got)
	}
}

func TestSymbolsOuterScopePrefixMatch(t *testing.T) {
	outer := NewEnv()
	outer.Set(types.Symbol{Val: "out-sym"}, 1)
	inner := NewSubordinateEnv(outer).(*Env)
	inner.Set(types.Symbol{Val: "in-sym"}, 2)

	got := completionSet("", inner.Symbols(nil, ""))
	if got["out-sym"] == 0 {
		t.Errorf("expected outer scope symbol out-sym in completions, got %v", got)
	}
	if got["in-sym"] == 0 {
		t.Errorf("expected inner scope symbol in-sym in completions, got %v", got)
	}
}

func TestSymbolsEmptyPrefixSortedPerScope(t *testing.T) {
	ns := NewEnv().(*Env)
	ns.Set(types.Symbol{Val: "charlie"}, 1)
	ns.Set(types.Symbol{Val: "alpha"}, 2)
	ns.Set(types.Symbol{Val: "bravo"}, 3)

	lines := ns.Symbols(nil, "")
	got := make([]string, len(lines))
	for i, ln := range lines {
		got[i] = string(ln)
	}
	want := []string{"alpha", "bravo", "charlie"}
	if len(got) != len(want) {
		t.Fatalf("expected %d entries, got %d (%v)", len(want), len(got), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("entry %d: want %q, got %q (full: %v)", i, w, got[i], got)
		}
	}
}

// TestSymbolsShadowedAppearsInBothScopes documents current behavior:
// a symbol shadowed in an inner scope is reported once per scope it occurs in,
// not deduplicated. Future work (LSP) may want to change this; if it does,
// update this test rather than letting silent regressions slip in.
func TestSymbolsShadowedAppearsInBothScopes(t *testing.T) {
	outer := NewEnv()
	outer.Set(types.Symbol{Val: "shared"}, 1)
	inner := NewSubordinateEnv(outer).(*Env)
	inner.Set(types.Symbol{Val: "shared"}, 2)

	got := completionSet("", inner.Symbols(nil, ""))
	if got["shared"] < 1 {
		t.Fatalf("expected shadowed symbol to appear, got %v", got)
	}
}
