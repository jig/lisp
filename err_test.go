package lisp

import (
	"context"
	"strings"
	"testing"

	"github.com/jig/lisp/env"
	"github.com/jig/lisp/types"
)

func TestBasicError(t *testing.T) {
	ns := env.NewEnv()
	_, err := REPL(context.Background(), ns, `(abc 1 2 3)`, types.NewCursorFile(t.Name()))
	if err == nil {
		t.Fatal("fatal error")
	}
	if !strings.HasSuffix(err.Error(), `'abc' not found`) {
		t.Fatalf("fatal error: %s", err)
	}
}

func TestTryCatchError2(t *testing.T) {
	ns := env.NewEnv()
	res, err := REPL(context.Background(), ns, `(try abc (catch exc exc))`, types.NewCursorFile(t.Name()))
	if err != nil {
		t.Fatal(err)
	}
	hm, err := READ(res.(string), nil)
	if err != nil {
		t.Fatal(err)
	}

	if hm.(types.HashMap).Val["ʞerr"].(string) != `'abc' not found` {
		t.Fatalf("fatal error: %s", hm.(types.HashMap).Val["ʞerr"])
	}
}

func TestTryCatchError3(t *testing.T) {
	ns := env.NewEnv()
	res, err := REPL(context.Background(), ns, `(try (abc 1 2) (catch exc exc))`, types.NewCursorFile(t.Name()))
	if err != nil {
		t.Fatal(err)
	}
	hm, err := READ(res.(string), nil)
	if err != nil {
		t.Fatal(err)
	}

	if hm.(types.HashMap).Val["ʞerr"].(string) != `'abc' not found` {
		t.Fatalf("fatal error: %s", hm.(types.HashMap).Val["ʞerr"])
	}
}
