package test_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/lib/test"
	"github.com/jig/lisp/lib/test/nstest"
	"github.com/jig/lisp/printer"
	"github.com/jig/lisp/types"
)

func newEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	if err := nscore.Load(ns); err != nil {
		t.Fatalf("load core: %v", err)
	}
	if err := nstest.Load(ns); err != nil {
		t.Fatalf("load test: %v", err)
	}
	return ns
}

func run(t *testing.T, ns types.EnvType, src string) types.MalType {
	t.Helper()
	ast, err := lisp.READ(src, types.NewCursorFile(t.Name()), ns)
	if err != nil {
		t.Fatalf("read %q: %v", src, err)
	}
	res, err := lisp.EVAL(context.Background(), ast, ns)
	if err != nil {
		t.Fatalf("eval %q: %v", src, err)
	}
	return res
}

func TestDeftestIsAre(t *testing.T) {
	ns := newEnv(t)
	run(t, ns, `(deftest passing (is (= 3 (+ 1 2))) (are [x y] (= x y) 2 2 4 4))`)
	run(t, ns, `(deftest failing (is (= 5 (+ 1 2)) "should be five") (is nil))`)
	run(t, ns, `(deftest erroring (is (= 1 (undefined-symbol))))`)

	reg := test.FromEnv(ns)
	if reg == nil {
		t.Fatal("no registry in env")
	}
	results := reg.RunAll(context.Background())
	if len(results) != 3 {
		t.Fatalf("expected 3 tests, got %d", len(results))
	}

	passing, failing, erroring := results[0], results[1], results[2]
	if !passing.OK() || len(passing.Checks) != 3 {
		t.Fatalf("passing: OK=%v checks=%d", passing.OK(), len(passing.Checks))
	}
	if failing.OK() || len(failing.Checks) != 2 {
		t.Fatalf("failing: OK=%v checks=%d", failing.OK(), len(failing.Checks))
	}
	if c := failing.Checks[0]; c.Expected != "5" || c.Actual != "3" || c.Message != "should be five" {
		t.Fatalf("failing check 0: %+v", c)
	}
	if erroring.OK() || erroring.Checks[0].Err == "" {
		t.Fatalf("erroring: OK=%v err=%q", erroring.OK(), erroring.Checks[0].Err)
	}
}

func TestReRegisterReplaces(t *testing.T) {
	ns := newEnv(t)
	run(t, ns, `(deftest same (is true))`)
	run(t, ns, `(deftest same (is true) (is true))`)
	reg := test.FromEnv(ns)
	results := reg.RunAll(context.Background())
	if len(results) != 1 {
		t.Fatalf("expected re-registration to replace, got %d tests", len(results))
	}
	if len(results[0].Checks) != 2 {
		t.Fatalf("expected the second registration to win, got %d checks", len(results[0].Checks))
	}
}

func TestIsOutsideDeftestThrows(t *testing.T) {
	ns := newEnv(t)
	if v := run(t, ns, `(is (= 2 (+ 1 1)))`); v != true {
		t.Fatalf("passing is outside deftest: got %v", printer.Pr_str(v, true))
	}
	ast, err := lisp.READ(`(is (= 1 2))`, types.NewCursorFile(t.Name()), ns)
	if err != nil {
		t.Fatal(err)
	}
	_, err = lisp.EVAL(context.Background(), ast, ns)
	if err == nil || !strings.Contains(err.Error(), "assertion failed") {
		t.Fatalf("failing is outside deftest should throw, got %v", err)
	}
}

func TestRunTestsFromLisp(t *testing.T) {
	ns := newEnv(t)
	run(t, ns, `(deftest one (is true))`)
	res := run(t, ns, `(test/run-tests!)`)
	printed := printer.Pr_str(res, true)
	if !strings.Contains(printed, `:name "one"`) || !strings.Contains(printed, ":ok true") {
		t.Fatalf("unexpected run-tests! output: %s", printed)
	}
}
