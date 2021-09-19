package mal

import (
	"context"
	"testing"
	"time"

	"github.com/jig/lisp/types"
)

func TestContextTimeoutFiresOnTime(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if _, err := REPL(newEnv(), `(sleep 1000)`, &ctx); err == nil {
		t.Fatalf("Must fail")
	} else {
		if err.(types.RuntimeError).ErrorVal.Error() != "timeout while evaluating expression" {
			t.Fatal(err)
		}
	}
}

func TestContextNoTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if _, err := REPL(newEnv(), `(sleep 1)`, &ctx); err != nil {
		t.Fatal(err)
	}
}
