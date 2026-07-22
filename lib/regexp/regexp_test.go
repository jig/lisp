package regexp_test

import (
	"context"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/lib/regexp/nsregexp"
	"github.com/jig/lisp/types"
)

func newEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	if err := nscore.Load(ns); err != nil {
		t.Fatal(err)
	}
	if err := nscore.LoadInput(ns); err != nil {
		t.Fatal(err)
	}
	if err := nsregexp.Load(ns); err != nil {
		t.Fatal(err)
	}
	return ns
}

// eval returns the printed form of src (REPL already prints the result).
func eval(t *testing.T, ns types.EnvType, src string) string {
	t.Helper()
	out, err := lisp.REPL(context.Background(), ns, src, types.NewCursorFile(t.Name()))
	if err != nil {
		t.Fatalf("EVAL %s: %v", src, err)
	}
	s, _ := out.(string)
	return s
}

func evalErr(t *testing.T, ns types.EnvType, src string) {
	t.Helper()
	if _, err := lisp.REPL(context.Background(), ns, src, types.NewCursorFile(t.Name())); err == nil {
		t.Fatalf("%s did not error", src)
	}
}

func TestPredicates(t *testing.T) {
	ns := newEnv(t)
	cases := []struct {
		src, want string
	}{
		// re-matches? is anchored (whole string)
		{`(re-matches? ¬\d+¬ "123")`, "true"},
		{`(re-matches? ¬\d+¬ "12a")`, "false"},
		{`(re-matches? ¬\d+¬ "a123")`, "false"},
		// re-find? is unanchored (substring)
		{`(re-find? ¬\d+¬ "abc123")`, "true"},
		{`(re-find? ¬\d+¬ "abc")`, "false"},
		{`(re-find? ¬xyz¬ "abc")`, "false"},
		// a compiled pattern works the same as a raw string
		{`(re-find? (re-pattern ¬[a-z]+¬) "HELLO world")`, "true"},
		{`(re-matches? (re-pattern ¬[a-z]+¬) "HELLO")`, "false"},
	}
	for _, c := range cases {
		if got := eval(t, ns, c.src); got != c.want {
			t.Errorf("%s = %s, want %s", c.src, got, c.want)
		}
	}
}

func TestMatchData(t *testing.T) {
	ns := newEnv(t)
	cases := []struct {
		src, want string
	}{
		// no groups -> the whole match string
		{`(re-find ¬\d+¬ "abc123def")`, `"123"`},
		{`(re-matches ¬\d+¬ "123")`, `"123"`},
		// no match -> nil
		{`(re-find ¬xyz¬ "abc")`, "nil"},
		{`(re-matches ¬\d+¬ "12a")`, "nil"},
		// groups -> [whole g1 g2 …]
		{`(re-find ¬(\d+)-(\d+)¬ "call 555-1234 now")`, `["555-1234" "555" "1234"]`},
		{`(re-matches ¬(\d+)-(\d+)¬ "555-1234")`, `["555-1234" "555" "1234"]`},
		// unmatched optional group -> nil
		{`(re-find ¬(a)?(b)¬ "b")`, `["b" nil "b"]`},
		// extracting a captured group
		{`(get (re-find ¬v(\d+)¬ "v42") 1)`, `"42"`},
	}
	for _, c := range cases {
		if got := eval(t, ns, c.src); got != c.want {
			t.Errorf("%s = %s, want %s", c.src, got, c.want)
		}
	}
}

func TestPatternValue(t *testing.T) {
	ns := newEnv(t)
	// re-pattern prints as «regex …» and is idempotent (accepts a regex).
	if got := eval(t, ns, `(str (re-pattern ¬[a-z]+¬))`); got != `"«regex [a-z]+»"` {
		t.Errorf("printed = %s", got)
	}
	if got := eval(t, ns, `(re-find? (re-pattern (re-pattern ¬\d¬)) "a1")`); got != "true" {
		t.Errorf("re-pattern on a regex = %s, want true", got)
	}
}

func TestErrors(t *testing.T) {
	ns := newEnv(t)
	// invalid pattern
	evalErr(t, ns, `(re-matches? ¬(unclosed¬ "x")`)
	evalErr(t, ns, `(re-pattern ¬[¬)`)
	// wrong argument types
	evalErr(t, ns, `(re-find? 42 "x")`)
	evalErr(t, ns, `(re-matches? ¬\d¬ 42)`)
}
