package lisp

import (
	"context"
	"reflect"
	"strings"
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
		if !strings.HasSuffix(err.Error(), "timeout while evaluating expression") {
			t.Fatalf("%s != %s", err.Error(), "timeout while evaluating expression")
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

func TestFutureContextTimeoutFiresOnTime(t *testing.T) {
	ctxB := context.Background()
	future := "(defmacro future (fn [& body] `(^{:once true} future-call (fn [] ~@body))))"
	if _, err := REPL(ctxB, newEnv(t.Name()), `(eval (read-string (str "(do "`+future+`" nil)")))`, types.NewCursorFile(reflect.TypeOf(&future).PkgPath())); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if _, err := REPL(ctx, newEnv(t.Name()), `@(future (sleep 1000))`, types.NewCursorFile(t.Name())); err == nil {
		t.Fatalf("Must fail")
	} else {
		if !strings.HasSuffix(err.Error(), "timeout while dereferencing future") {
			t.Fatalf("%s != %s", err.Error(), "timeout while dereferencing future")
		}
	}
}

func TestFutureContextNoTimeout(t *testing.T) {
	ctxB := context.Background()
	future := "(defmacro future (fn [& body] `(^{:once true} future-call (fn [] ~@body))))"
	if _, err := REPL(ctxB, newEnv(t.Name()), `(eval (read-string (str "(do "`+future+`" nil)")))`, types.NewCursorFile(reflect.TypeOf(&future).PkgPath())); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if _, err := REPL(ctx, newEnv(t.Name()), `@(future (sleep 1))`, types.NewCursorFile(t.Name())); err != nil {
		t.Fatal(err)
	}
}
