package lisp

import (
	"context"
	"testing"
	"time"
)

func TestContextTimeoutFiresOnTime(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if _, err := REPL(ctx, newEnv(), `(sleep 1000)`); err == nil {
		t.Fatalf("Must fail")
	} else {
		if err.Error() != "timeout while evaluating expression" {
			t.Fatal(err)
		}
	}
}

func TestContextNoTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if _, err := REPL(ctx, newEnv(), `(sleep 1)`); err != nil {
		t.Fatal(err)
	}
}
