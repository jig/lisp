package lisp

import (
	"context"
	"testing"

	. "github.com/jig/lisp/env"
	. "github.com/jig/lisp/types"
)

// TestMalformedInputNoPanic guards the interpreter against panicking on
// malformed but well-read Lisp input — for an embeddable interpreter a
// bad config file must surface a LispError, never crash the host Go
// process. Each case previously panicked with an index-out-of-range
// (see ROADMAP-robustness.md phase 2). A few valid forms are included to
// pin that the guards did not break the happy path.
func TestMalformedInputNoPanic(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		wantErr bool
	}{
		// 2.1 — catch clause with no binding / body.
		{"try-catch-no-binding", "(try 1 (catch))", true},
		{"try-catch-finally-no-binding", "(try 1 (catch) (finally 2))", true},
		// 2.2 — first() on an empty list; must not panic, evaluates to ().
		{"try-empty-list", "(try ())", false},
		// 2.3 — binding vector edge cases.
		{"fn-trailing-amp", "(do (def f (fn [&] 1)) (f))", true},
		{"fn-nonsymbol-after-amp", "(do (def g (fn [& 3] 1)) (g 1))", true},
		{"fn-nonsymbol-binding", "(do (def h (fn [3] 1)) (h 1))", true},
		// 2.4 — bare unquote / splice-unquote inside quasiquote.
		{"quasiquote-bare-unquote", "(quasiquote (unquote))", true},
		{"quasiquote-bare-splice", "(quasiquote ((splice-unquote)))", true},
		// Happy path — the guards must leave these working.
		{"try-catch-ok", "(try 1 (catch e e))", false},
		{"quasiquote-unquote-ok", "(quasiquote (unquote 5))", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := NewEnv()
			ast, err := READ(tc.src, nil, e)
			if err != nil {
				t.Fatalf("READ(%q): %v", tc.src, err)
			}
			_, err = evalNoPanic(t, ast, e)
			switch {
			case tc.wantErr && err == nil:
				t.Fatalf("EVAL(%q): expected an error, got nil", tc.src)
			case !tc.wantErr && err != nil:
				t.Fatalf("EVAL(%q): unexpected error: %v", tc.src, err)
			}
		})
	}
}

// evalNoPanic runs EVAL and turns a panic into a test failure rather
// than a process crash, so a regression reports cleanly.
func evalNoPanic(t *testing.T, ast MalType, e EnvType) (res MalType, err error) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("EVAL panicked instead of returning an error: %v", r)
		}
	}()
	return EVAL(context.Background(), ast, e)
}
