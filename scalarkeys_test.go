package lisp

import (
	"context"
	"strings"
	"testing"

	"github.com/jig/lisp/types"
)

// Hash-map keys and set elements accept any immutable scalar — nil,
// booleans, ints, floats, strings and keywords — ordered by type group
// then value (types.KeyLess), so sequencing stays deterministic.
func TestScalarCollectionKeys(t *testing.T) {
	ns := seqEnv(t)
	cases := []struct{ src, want string }{
		// set literals and constructors
		{"#{1 2 3}", "#{1 2 3}"},
		{"(set [3 1 2])", "#{1 2 3}"},
		{"(= #{1 2} (set [2 1]))", "true"},
		{"(contains? #{1 2} 2)", "true"},
		{"(contains? #{1 2} 3)", "false"},
		{"(contains? #{1.5} 1.5)", "true"},
		{"(count (into #{} [1 2 2 3]))", "3"},
		{"(seq #{3 1 2})", "(1 2 3)"},
		// map literals and access
		{"(get {1 \"one\" :a 2} 1)", "\"one\""},
		{"(get {true \"t\"} true)", "\"t\""},
		{"(get {nil \"n\"} nil)", "\"n\""},
		{"(get (assoc {} 42 \"x\") 42)", "\"x\""},
		{"(count (dissoc {1 \"a\" 2 \"b\"} 1))", "1"},
		{"(contains? {1 \"a\"} 1)", "true"},
		// deterministic cross-type entry order: nil < booleans < ints <
		// floats < strings < keywords
		{"(seq {5 \"a\" 1 \"b\"})", "([1 \"b\"] [5 \"a\"])"},
		{"(seq {\"s\" 3 :k 4 3 2 true 1 nil 0})", "([nil 0] [true 1] [3 2] [\"s\" 3] [:k 4])"},
		{"(into {} (map (fn [[k v]] [k (* 2 v)]) {1 10 2 20}))", "{1 20 2 40}"},
	}
	for _, c := range cases {
		res, err := REPL(context.Background(), ns, c.src, types.NewCursorFile(t.Name()))
		if err != nil {
			t.Errorf("eval %q: %v", c.src, err)
			continue
		}
		if res.(string) != c.want {
			t.Errorf("eval %q = %s, want %s", c.src, res, c.want)
		}
	}
}

// Composite values, symbols and big ints are not valid keys: composites
// are not hashable as Go map keys, and symbols and big ints only compare
// by identity.
func TestNonScalarKeysRejected(t *testing.T) {
	ns := seqEnv(t)
	for src, wantErr := range map[string]string{
		"(set [[1 2]])":          "set items must be scalar values",
		"(assoc {} [1] 2)":       "scalar key",
		"(assoc #{} [1])":        "scalar key",
		"{[1] 2}":                "hash-map keys must be scalar values",
		"(assoc {} 0x0A 1)":      "scalar key",
		"(dissoc {\"a\" 1} [1])": "scalar key",
		"(json-encode {1 2})":    "cannot JSON-encode",
	} {
		_, err := REPL(context.Background(), ns, src, types.NewCursorFile(t.Name()))
		if err == nil {
			t.Errorf("eval %q: expected error, got none", src)
			continue
		}
		if !strings.Contains(err.Error(), wantErr) {
			t.Errorf("eval %q: error %q, want it to contain %q", src, err, wantErr)
		}
	}
}
