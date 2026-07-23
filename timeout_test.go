package lisp

import (
	"context"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jig/lisp/types"
)

// These tests exercise context-deadline handling in EVAL. They run inside
// a synctest bubble: the time package uses a fake clock that only advances
// when every goroutine is durably blocked, so the deadlines below fire
// deterministically (no wall-clock jitter) and instantly (no real wait).

func TestContextTimeoutFiresOnTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := REPL(ctx, newEnv(t.Name()), `(sleep 1000)`, types.NewCursorFile(t.Name()))
		if err == nil || !strings.Contains(err.Error(), "timeout while evaluating expression") {
			t.Fatalf("want a timeout error, got %v", err)
		}
	})
}

func TestContextNoTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		if _, err := REPL(ctx, newEnv(t.Name()), `(sleep 1)`, types.NewCursorFile(t.Name())); err != nil {
			t.Fatal(err)
		}
	})
}

func TestFutureContextTimeoutFiresOnTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := REPL(ctx, newEnv(t.Name()), `@(future (sleep 1000))`, types.NewCursorFile(t.Name()))
		if err == nil || !strings.Contains(err.Error(), "timeout while dereferencing future") {
			t.Fatalf("want a future-deref timeout, got %v", err)
		}
	})
}

func TestFutureContextNoTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
		defer cancel()

		if _, err := REPL(ctx, newEnv(t.Name()), `@(future (sleep 1))`, types.NewCursorFile(t.Name())); err != nil {
			t.Fatal(err)
		}
	})
}

func TestTimeoutOnTryCatch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ns := newEnv(t.Name())
		ast, err := READ(`(try (sleep 10000) (catch e (str "ERR: " (error-string e))))`, types.NewCursorFile(t.Name()), ns)
		if err != nil {
			t.Fatal(err)
		}
		// `try` gives 80% of the deadline to the body and reserves 20% for
		// the catch/finally handler (mal.go). Under the real clock a small
		// 20% slice was flaky: scheduling jitter between the body's timeout
		// firing and the handler running could exceed it, and the
		// top-of-loop ctx check then re-fired the expired deadline outside
		// the try, uncatchably. Under synctest's fake clock there is no
		// jitter — the catch handler runs at zero fake time — so the
		// timeout is caught deterministically even with this tight deadline.
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		res, err := EVAL(ctx, ast, ns)
		if err != nil {
			if err.Error() == "timeout while evaluating expression" {
				t.Fatalf("timeout not caught: %s", err)
			}
			t.Fatalf("unexpected error caught: %s", err)
		}
		if res.(string) != "ERR: timeout while evaluating expression" {
			t.Fatalf("unexpected result: %s", res)
		}
	})
}
