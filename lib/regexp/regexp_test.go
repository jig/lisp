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

func TestSeq(t *testing.T) {
	ns := newEnv(t)
	cases := []struct {
		src, want string
	}{
		// no groups -> each match is a string
		{`(re-seq ¬\d+¬ "a1 bb 22 c333")`, `["1" "22" "333"]`},
		// no match -> empty vector
		{`(re-seq ¬\d+¬ "abc")`, "[]"},
		// groups -> each match is [whole g1 g2 …]
		{`(re-seq ¬(\d)(\w)¬ "1a 2b")`, `[["1a" "1" "a"] ["2b" "2" "b"]]`},
		// a compiled pattern works too
		{`(re-seq (re-pattern ¬[a-z]+¬) "ab CD ef")`, `["ab" "ef"]`},
	}
	for _, c := range cases {
		if got := eval(t, ns, c.src); got != c.want {
			t.Errorf("%s = %s, want %s", c.src, got, c.want)
		}
	}
}

func TestReplace(t *testing.T) {
	ns := newEnv(t)
	cases := []struct {
		src, want string
	}{
		// replace all, literal replacement
		{`(re-replace ¬\d+¬ "a1b22c333" "#")`, `"a#b#c#"`},
		// group references with ${n}
		{`(re-replace ¬(\d+)-(\d+)¬ "555-1234 and 9-8" "${2}.${1}")`, `"1234.555 and 8.9"`},
		// $$ is a literal dollar
		{`(re-replace ¬\d¬ "a1" "$$")`, `"a$"`},
		// no match -> unchanged
		{`(re-replace ¬x¬ "abc" "y")`, `"abc"`},
		// replace-first only touches the first match
		{`(re-replace-first ¬\d+¬ "a1b22c333" "#")`, `"a#b22c333"`},
		{`(re-replace-first ¬(\d+)¬ "v42 v7" "[${1}]")`, `"v[42] v7"`},
		{`(re-replace-first ¬x¬ "abc" "y")`, `"abc"`},
	}
	for _, c := range cases {
		if got := eval(t, ns, c.src); got != c.want {
			t.Errorf("%s = %s, want %s", c.src, got, c.want)
		}
	}
}

func TestSplit(t *testing.T) {
	ns := newEnv(t)
	cases := []struct {
		src, want string
	}{
		{`(re-split ¬,¬ "a,b,c")`, `["a" "b" "c"]`},
		{`(re-split ¬\s+¬ "one  two   three")`, `["one" "two" "three"]`},
		// no match -> the whole string as a single piece
		{`(re-split ¬,¬ "abc")`, `["abc"]`},
		// trailing empty pieces are kept (Go semantics)
		{`(re-split ¬,¬ "a,b,,")`, `["a" "b" "" ""]`},
		// limit caps the number of pieces; the last keeps the remainder
		{`(re-split ¬,¬ "a,b,c,d" 2)`, `["a" "b,c,d"]`},
		// compiled pattern
		{`(re-split (re-pattern ¬\d¬) "a1b2c")`, `["a" "b" "c"]`},
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
	// re-split limit must be an integer
	evalErr(t, ns, `(re-split ¬,¬ "a,b" "2")`)
}
