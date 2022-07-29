package lisp

import (
	"context"
	"strings"
	"testing"

	"github.com/jig/lisp/env"
	"github.com/jig/lisp/types"
)

func TestBasicError(t *testing.T) {
	ns, _ := env.NewEnv(nil, nil, nil)
	_, err := REPL(context.Background(), ns, `(abc 1 2 3)`, types.NewCursorFile(t.Name()))
	if err == nil {
		t.Fatal("fatal error")
	}
	if !strings.HasSuffix(err.Error(), `'abc' not found`) {
		t.Fatalf("fatal error: %s", err)
	}
}

func TestTryCatchError2(t *testing.T) {
	ns, _ := env.NewEnv(nil, nil, nil)
	res, err := REPL(context.Background(), ns, `(try abc (catch exc exc))`, types.NewCursorFile(t.Name()))
	if err != nil {
		t.Fatal(err)
	}
	if res != `'abc' not found` {
		t.Fatalf("fatal error: %s", res)
	}
}

func TestTryCatchError3(t *testing.T) {
	ns, _ := env.NewEnv(nil, nil, nil)
	res, err := REPL(context.Background(), ns, `(try (abc 1 2) (catch exc exc))`, types.NewCursorFile(t.Name()))
	if err != nil {
		t.Fatal(err)
	}
	if res != `'abc' not found` {
		t.Fatalf("fatal error: %s", res)
	}
}
