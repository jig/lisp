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
	if !strings.HasSuffix(err.Error(), `symbol 'abc' not found`) {
		t.Fatal(err)
	}
}

func TestTryCatchError2(t *testing.T) {
	ns := env.NewEnv()
	res, err := REPL(context.Background(), ns, `(try abc (catch exc exc))`, types.NewCursorFile(t.Name()))
	if err != nil {
		t.Fatal(err)
	}

	//if !strings.HasSuffix(res.(string), `'abc' not found`) {
	if res != `«go-error "symbol 'abc' not found"»` {
		t.Fatalf("%s", res)
	}
}

func TestTryCatchError3(t *testing.T) {
	ns := env.NewEnv()
	res, err := REPL(context.Background(), ns, `(try (abc 1 2) (catch exc exc))`, types.NewCursorFile(t.Name()))
	if err != nil {
		t.Fatal(err)
	}

	// if !strings.HasSuffix(res.(string), `'abc' not found`) {
	if res != `«go-error "symbol 'abc' not found"»` {
		t.Fatalf("%s", res)
	}
}

func TestTryCatchThrowsMalType(t *testing.T) {
	ns := env.NewEnv()
	LoadCore(ns)
	res, err := REPL(context.Background(), ns, `(try (throw {:a 1}) (catch exc exc))`, types.NewCursorFile(t.Name()))
	if err != nil {
		t.Fatal(err)
	}

	// if !strings.HasSuffix(res.(string), `'abc' not found`) {
	if res != `{:a 1}` {
		t.Fatalf("%s", res)
	}
}
