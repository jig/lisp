package lisp

import (
	"context"
	"strings"
	"testing"

	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core"
	"github.com/jig/lisp/lib/coreextended"
	"github.com/jig/lisp/types"
)

func seqEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	core.Load(ns)
	core.LoadInput(ns)
	coreextended.Load(ns)
	ctx := context.Background()
	for _, h := range []string{core.HeaderBasic(), coreextended.HeaderCoreExtended()} {
		if _, err := REPL(ctx, ns, h, nil); err != nil {
			t.Fatalf("loading headers: %v", err)
		}
	}
	return ns
}

// Hash-maps and sets seq as in Clojure: a hash-map as a sequence of [key
// value] entry vectors, a set as a sequence of its elements. Iteration
// order is unspecified, so multi-entry results are compared with lisp
// equality (or reduced to order-free values), never positionally.
func TestSeqableHashMap(t *testing.T) {
	ns := seqEnv(t)
	cases := []struct{ src, want string }{
		// seq
		{"(seq {})", "nil"},
		{"(seq {:a 1})", "([:a 1])"},
		{"(= (set (map str (seq {:a 1 :b 2}))) #{\"[:a 1]\" \"[:b 2]\"})", "true"},
		// first / rest
		{"(first {:a 1})", "[:a 1]"},
		{"(first {})", "nil"},
		{"(count (rest {:a 1 :b 2}))", "1"},
		{"(rest {})", "()"},
		// map / filter / reduce with entry destructuring
		{"(reduce (fn [acc [k v]] (+ acc v)) 0 {:a 1 :b 2})", "3"},
		{"(filter (fn [[k v]] (> v 1)) {:a 1 :b 2})", "([:b 2])"},
		{"(= (set (map (fn [[k v]] k) {\"x\" 1 \"y\" 2})) #{\"x\" \"y\"})", "true"},
		// the (into {} (map f m)) round-trip (backdate pattern)
		{"(= (into {} (map (fn [[k v]] [k (- v)]) {:seconds 5 :days 2})) {:seconds -5 :days -2})", "true"},
		// other GetSlice-based functions
		{"(concat {:a 1} [2])", "([:a 1] 2)"},
		{"(cons 0 {:a 1})", "(0 [:a 1])"},
		{"(vec {:a 1})", "[[:a 1]]"},
		{"(apply count [{:a 1 :b 2}])", "2"},
		// regression guards
		{"(count {:a 1 :b 2})", "2"},
		{"(= (set (keys {:a 1 :b 2})) (set (map (fn [[k v]] k) {:a 1 :b 2})))", "true"},
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

func TestSeqableSet(t *testing.T) {
	ns := seqEnv(t)
	cases := []struct{ src, want string }{
		{"(seq #{})", "nil"},
		{"(seq #{\"a\"})", "(\"a\")"},
		{"(map (fn [x] x) #{\"a\"})", "(\"a\")"},
		{"(= (set (filter (fn [x] (not= x \"a\")) #{\"a\" \"b\" \"c\"})) #{\"b\" \"c\"})", "true"},
		{"(first #{\"a\"})", "\"a\""},
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

// Sequential destructuring in binding forms: a vector pattern in fn
// params or let bindings destructures positionally, recursively; missing
// elements bind nil, `&` binds the remainder as a list.
func TestSequentialDestructuring(t *testing.T) {
	ns := seqEnv(t)
	cases := []struct{ src, want string }{
		{"((fn [[a b]] (+ a b)) [1 2])", "3"},
		{"((fn [[a [b c]]] (+ a b c)) [1 [2 3]])", "6"},
		{"((fn [[a b]] b) [1])", "nil"},
		{"((fn [[a b]] [a b]) nil)", "[nil nil]"},
		{"((fn [[a & r]] r) [1 2 3])", "(2 3)"},
		{"((fn [[a & r]] r) [1])", "()"},
		{"((fn [x [a b] y] (+ x a b y)) 1 [2 3] 4)", "10"},
		{"(let [[a b] [1 2]] (+ a b))", "3"},
		{"(let [[a b] (list 1 2) c 3] (+ a b c))", "6"},
		{"(let [[k v] (first {:a 41})] [k (+ v 1)])", "[:a 42]"},
		// & followed by a pattern
		{"((fn [& [a b]] [a b]) 1 2)", "[1 2]"},
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

	for src, wantErr := range map[string]string{
		"((fn [[a b]] a) 7)": "cannot destructure",
		"(let [[a b] 7] a)":  "cannot destructure",
		"(let [:a 1] :a)":    "binding list expected symbol or vector",
		"((fn [a b] a) 1)":   "too few arguments",  // arity stays strict at the top level
		"((fn [a] a) 1 2)":   "too many arguments", // idem
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

// Maps and sets seq but are not indexed: nth must reject them instead of
// returning a nondeterministic element (Clojure errors here too).
func TestNthRejectsMapsAndSets(t *testing.T) {
	ns := seqEnv(t)
	for src, wantErr := range map[string]string{
		"(nth {:a 1} 0)":   "nth not supported on a hash-map",
		"(nth #{\"a\"} 0)": "nth not supported on a set",
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
