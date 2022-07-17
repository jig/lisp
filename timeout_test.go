package lisp

import (
	"context"
	"testing"
	"time"

	"github.com/jig/lisp/types"
)

func TestContextTimeoutFiresOnTime(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if _, err := REPL(ctx, newEnv(t.Name()), `(sleep 1000)`, types.NewCursorFile(t.Name())); err == nil {
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

	if _, err := REPL(ctx, newEnv(t.Name()), `(sleep 1)`, types.NewCursorFile(t.Name())); err != nil {
		t.Fatal(err)
	}
}
