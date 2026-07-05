package lazy

import (
	"context"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/concurrent"
	"github.com/jig/lisp/lib/core"
	"github.com/jig/lisp/types"
)

// testEnv builds an interpreter with core, the basic header, the concurrent
// namespace (for atom/swap! used in the laziness tests) and the lazy
// namespace loaded.
func testEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	core.Load(ns)
	if _, err := lisp.REPL(context.Background(), ns, core.HeaderBasic(), types.NewCursorFile("preamble")); err != nil {
		t.Fatalf("HeaderBasic: %v", err)
	}
	concurrent.Load(ns)
	Load(ns)
	return ns
}

func evalErr(ns types.EnvType, src string) (types.MalType, error) {
	ast, err := lisp.READ(src, types.NewCursorFile("test"), ns)
	if err != nil {
		return nil, err
	}
	return lisp.EVAL(context.Background(), ast, ns)
}

func eval(t *testing.T, ns types.EnvType, src string) types.MalType {
	t.Helper()
	res, err := evalErr(ns, src)
	if err != nil {
		t.Fatalf("eval %q: %v", src, err)
	}
	return res
}

// asInts realises a Vector result into a Go []int for easy comparison.
func asInts(t *testing.T, v types.MalType) []int {
	t.Helper()
	vec, ok := v.(types.Vector)
	if !ok {
		t.Fatalf("expected Vector, got %T", v)
	}
	out := make([]int, len(vec.Val))
	for i, e := range vec.Val {
		n, ok := e.(int)
		if !ok {
			t.Fatalf("element %d is %T, want int", i, e)
		}
		out[i] = n
	}
	return out
}

func eq(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestProducersAndTransformers(t *testing.T) {
	ns := testEnv(t)
	cases := []struct {
		name string
		src  string
		want []int
	}{
		{"range bounded", `(realize (lazy-range 0 4))`, []int{0, 1, 2, 3}},
		{"range start-end-step", `(realize (lazy-range 1 10 3))`, []int{1, 4, 7}},
		{"range infinite + take", `(realize (lazy-take 4 (lazy-range)))`, []int{0, 1, 2, 3}},
		{"map over infinite", `(realize (lazy-take 3 (lazy-map (fn [x] (* x x)) (lazy-range))))`, []int{0, 1, 4}},
		{"filter over infinite", `(realize (lazy-take 3 (lazy-filter (fn [x] (> x 5)) (lazy-range))))`, []int{6, 7, 8}},
		{"remove", `(realize (lazy-remove (fn [x] (> x 2)) [1 2 3 4 1]))`, []int{1, 2, 1}},
		{"drop", `(realize (lazy-drop 2 (lazy-range 0 5)))`, []int{2, 3, 4}},
		{"take-while", `(realize (lazy-take-while (fn [x] (< x 3)) (lazy-range)))`, []int{0, 1, 2}},
		{"drop-while", `(realize (lazy-drop-while (fn [x] (< x 3)) [0 1 2 3 4 1]))`, []int{3, 4, 1}},
		{"iterate", `(realize (lazy-take 5 (lazy-iterate (fn [x] (* x 2)) 1)))`, []int{1, 2, 4, 8, 16}},
		{"repeat n", `(realize (lazy-repeat 3 7))`, []int{7, 7, 7}},
		{"repeat infinite + take", `(realize (lazy-take 2 (lazy-repeat 9)))`, []int{9, 9}},
		{"cycle", `(realize (lazy-take 7 (lazy-cycle [1 2 3])))`, []int{1, 2, 3, 1, 2, 3, 1}},
		{"eager vector as source", `(realize (lazy-map (fn [x] (+ x 1)) [10 20 30]))`, []int{11, 21, 31}},
		{"eager list as source", `(realize (lazy-filter (fn [x] (> x 1)) (list 1 2 3)))`, []int{2, 3}},
		{"pipeline", `(realize (lazy-take 3 (lazy-filter (fn [x] (> x 3)) (lazy-map (fn [x] (+ x 1)) (lazy-range)))))`, []int{4, 5, 6}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := asInts(t, eval(t, ns, tc.src))
			if !eq(got, tc.want) {
				t.Errorf("%s = %v, want %v", tc.src, got, tc.want)
			}
		})
	}
}

func TestConsumers(t *testing.T) {
	ns := testEnv(t)

	if got := eval(t, ns, `(lazy-first (lazy-range 5 9))`); got != 5 {
		t.Errorf("lazy-first = %v, want 5", got)
	}
	if got := eval(t, ns, `(lazy-first [])`); got != nil {
		t.Errorf("lazy-first of empty = %v, want nil", got)
	}
	if got := eval(t, ns, `(lazy-nth (lazy-range) 1000)`); got != 1000 {
		t.Errorf("lazy-nth = %v, want 1000", got)
	}
	if got := eval(t, ns, `(lazy-reduce + 0 (lazy-take 100 (lazy-range 1 1000000)))`); got != 5050 {
		t.Errorf("lazy-reduce = %v, want 5050", got)
	}
	if got := eval(t, ns, `(lazy-seq? (lazy-rest (lazy-range)))`); got != true {
		t.Errorf("lazy-rest should be a lazy-seq, got %v", got)
	}
	if _, err := evalErr(ns, `(lazy-nth (lazy-range 0 3) 10)`); err == nil {
		t.Error("lazy-nth out of range should error")
	}
}

// TestMemoisation checks that a lazy sequence forces each element at most
// once even across repeated traversals.
func TestMemoisation(t *testing.T) {
	ns := testEnv(t)
	eval(t, ns, `(def calls (atom 0))`)
	eval(t, ns, `(def s (lazy-map (fn [x] (do (swap! calls (fn [n] (+ n 1))) x)) [1 2 3]))`)
	eval(t, ns, `(realize s)`)
	eval(t, ns, `(realize s)`) // second traversal must not re-run the map fn
	if got := eval(t, ns, `(deref calls)`); got != 3 {
		t.Errorf("map fn call count = %v, want 3 (memoised)", got)
	}
}

// TestErrorPropagation checks that a throw inside a lazy computation surfaces
// when the element is forced.
func TestErrorPropagation(t *testing.T) {
	ns := testEnv(t)
	if _, err := evalErr(ns, `(realize (lazy-map (fn [x] (throw "boom")) [1 2 3]))`); err == nil {
		t.Fatal("expected the thrown error to propagate through realize")
	}
	// the error must only fire once the offending element is forced
	if _, err := evalErr(ns, `(lazy-first (lazy-map (fn [x] (throw "boom")) [1 2 3]))`); err == nil {
		t.Fatal("expected lazy-first to force and propagate the error")
	}
}
