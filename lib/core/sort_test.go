package core_test

import (
	"context"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/types"
)

func sortEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	if err := nscore.Load(ns); err != nil {
		t.Fatal(err)
	}
	return ns
}

func sortEval(t *testing.T, ns types.EnvType, src string) string {
	t.Helper()
	out, err := lisp.REPL(context.Background(), ns, src, types.NewCursorFile(t.Name()))
	if err != nil {
		t.Fatalf("EVAL %s: %v", src, err)
	}
	s, _ := out.(string)
	return s
}

func sortErr(t *testing.T, ns types.EnvType, src string) {
	t.Helper()
	if _, err := lisp.REPL(context.Background(), ns, src, types.NewCursorFile(t.Name())); err == nil {
		t.Fatalf("%s did not error", src)
	}
}

// TestSort pins (sort coll): stable ordering over the scalar total
// order, list/vector/nil inputs, and the non-scalar rejection.
func TestSort(t *testing.T) {
	ns := sortEnv(t)
	for src, want := range map[string]string{
		`(sort [3 1 2])`:               `(1 2 3)`,
		`(sort (list "b" "a"))`:        `("a" "b")`,
		`(sort [:b 2 "a" nil true])`:   `(nil true 2 "a" :b)`, // type groups
		`(sort [])`:                    `()`,
		`(sort nil)`:                   `()`,
		`(sort [2 1.5])`:               `(2 1.5)`,          // ints before floats (type groups)
		`(str (sort [1 3 2]) [1 3 2])`: `"(1 2 3)[1 3 2]"`, // input untouched
	} {
		if got := sortEval(t, ns, src); got != want {
			t.Errorf("%s = %s, want %s", src, got, want)
		}
	}
	sortErr(t, ns, `(sort [[1] [2]])`) // composites have no order
	sortErr(t, ns, `(sort {:a 1})`)    // not a list/vector
}

// TestSortBy pins (sort-by f coll): function keys, the keyword-as-map-
// accessor form, stability, and key validation.
func TestSortBy(t *testing.T) {
	ns := sortEnv(t)
	for src, want := range map[string]string{
		`(sort-by (fn [m] (get m :ms)) [{:ms 9} {:ms 1} {:ms 4}])`: `({:ms 1} {:ms 4} {:ms 9})`,
		`(sort-by :ms [{:ms 9} {:ms 1}])`:                          `({:ms 1} {:ms 9})`,
		// descending order via key negation
		`(sort-by (fn [x] (- 0 x)) [1 3 2])`: `(3 2 1)`,
		// stable: equal keys keep input order
		`(sort-by :k [{:k 1 :i 1} {:k 1 :i 2}])`: `({:i 1 :k 1} {:i 2 :k 1})`,
	} {
		if got := sortEval(t, ns, src); got != want {
			t.Errorf("%s = %s, want %s", src, got, want)
		}
	}
	sortErr(t, ns, `(sort-by :k [1 2])`)           // keyword key needs maps
	sortErr(t, ns, `(sort-by (fn [x] [x]) [1 2])`) // derived key not scalar
}
