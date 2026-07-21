package printer_test

import (
	"errors"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jig/lisp/printer"
	"github.com/jig/lisp/reader"
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

// pointerStringer is a helper whose String() has a pointer receiver, so
// it would be lost if Pr_str dereferenced the value before checking for
// fmt.Stringer.
type pointerStringer struct{ n int }

func (p *pointerStringer) String() string { return "stringer:" + strconv.Itoa(p.n) }

// TestPrStrStringer covers the fmt.Stringer branch: Go values with a
// custom String() must render via that method instead of exposing their
// internal struct layout (String() methods often use pointer receivers,
// so the branch must come before the pointer dereference in the default
// branch).
func TestPrStrStringer(t *testing.T) {
	if got, want := printer.Pr_str(&pointerStringer{n: 7}, true), "stringer:7"; got != want {
		t.Fatalf("*pointerStringer = %q, want %q", got, want)
	}
}

// TestPrStrBigInt covers the dedicated *big.Int case: a 0x-prefixed
// uppercase hex literal padded to whole octets — the same form the
// reader accepts — plus a sign prefix for (Go-side) negatives and nil
// safety.
func TestPrStrBigInt(t *testing.T) {
	for _, tc := range []struct {
		in   *big.Int
		want string
	}{
		{big.NewInt(1234567890), "0x499602D2"},
		{big.NewInt(15), "0x0F"},
		{big.NewInt(-1234567890), "-0x499602D2"},
		{big.NewInt(0), "0x00"},
		{nil, "nil"},
	} {
		if got := printer.Pr_str(tc.in, true); got != tc.want {
			t.Fatalf("*big.Int %v = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestPrStrTime covers the time.Time and time.Duration cases: both
// render as their millisecond count, matching time-ms.
func TestPrStrTime(t *testing.T) {
	if got, want := printer.Pr_str(time.UnixMilli(1234567890123).UTC(), true), "1234567890123"; got != want {
		t.Fatalf("time.Time = %q, want %q", got, want)
	}
	if got, want := printer.Pr_str(1500*time.Millisecond, true), "1500"; got != want {
		t.Fatalf("time.Duration = %q, want %q", got, want)
	}
}

// TestPrStrNilStringer guards the nil-pointer case of the fmt.Stringer
// branch: a nil pointer whose String() dereferences its receiver
// (pointerStringer) must render as "nil", not panic. Before the nil
// guard the Stringer branch called String() on the nil pointer and
// crashed, where the default branch had safely printed "nil".
func TestPrStrNilStringer(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Pr_str panicked on a nil Stringer pointer: %v", r)
		}
	}()

	var ps *pointerStringer
	if got := printer.Pr_str(ps, true); got != "nil" {
		t.Fatalf("nil *pointerStringer = %q, want %q", got, "nil")
	}
	var bi *big.Int
	if got := printer.Pr_str(bi, true); got != "nil" {
		t.Fatalf("nil *big.Int = %q, want %q", got, "nil")
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

func TestPrDataDeterministicMixedCollections(t *testing.T) {
	value := types.HashMap{Val: map[string]types.MalType{
		"ʞz": types.HashMap{Val: map[string]types.MalType{"ʞb": 2, "ʞa": 1}},
		"a": types.List{Val: []types.MalType{
			types.Symbol{Val: "quote"},
			types.Vector{Val: []types.MalType{3, 2, 1}},
		}},
		"ʞa": types.Set{Val: map[string]struct{}{"beta": {}, "ʞalpha": {}, "alpha": {}}},
		"z":  nil,
	}}
	want := `{"a" (quote [3 2 1]) "z" nil :a #{"alpha" "beta" :alpha} :z {:a 1 :b 2}}`

	for range 20 {
		if got := printer.Pr_data(value, 200); got != want {
			t.Fatalf("Pr_data = %q, want %q", got, want)
		}
	}
}

func TestPrDataNestedWidthAwareLayout(t *testing.T) {
	value := types.Vector{Val: []types.MalType{
		types.HashMap{Val: map[string]types.MalType{
			"ʞvalues": types.Vector{Val: []types.MalType{1, 2, 3, 4}},
			"ʞname":   "alpha",
		}},
		types.HashMap{Val: map[string]types.MalType{
			"ʞquoted": types.List{Val: []types.MalType{
				types.Symbol{Val: "quote"},
				types.Vector{Val: []types.MalType{10, 20, 30, 40}},
			}},
			"ʞname": "beta",
		}},
	}}
	want := `[{:name "alpha"
  :values [1 2 3 4]}
 {:name "beta"
  :quoted (quote [10
                  20
                  30
                  40])}]`

	got := printer.Pr_data(value, 24)
	if got != want {
		t.Fatalf("Pr_data:\n%s\nwant:\n%s", got, want)
	}

	roundTrip, err := reader.Read_str(got, types.NewCursorFile(t.Name()), nil)
	if err != nil {
		t.Fatalf("read Pr_data output: %v", err)
	}
	if roundTripPrinted := printer.Pr_data(roundTrip, 24); roundTripPrinted != want {
		t.Fatalf("round-trip Pr_data:\n%s\nwant:\n%s", roundTripPrinted, want)
	}
}
