package lisp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jig/lisp/env"
	"github.com/jig/lisp/runtime"
	"github.com/jig/lisp/types"
)

type recordingHook struct {
	events []runtime.EvalEvent
	err    error
}

func (h *recordingHook) OnEval(_ context.Context, ev runtime.EvalEvent) error {
	h.events = append(h.events, ev)
	return h.err
}

func withHook(t *testing.T, h runtime.EvalHook) {
	t.Helper()
	prev := runtime.Hook
	runtime.Hook = h
	t.Cleanup(func() { runtime.Hook = prev })
}

func mustEval(t *testing.T, source string) (types.MalType, error) {
	t.Helper()
	ns := env.NewEnv()
	ast, err := READ(source, types.NewCursorFile("hook-test"), ns)
	if err != nil {
		t.Fatalf("READ failed: %v", err)
	}
	return EVAL(context.Background(), ast, ns)
}

func TestEvalHookInvoked(t *testing.T) {
	h := &recordingHook{}
	withHook(t, h)

	if _, err := mustEval(t, "(let [a 1 b 2] (do a b))"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(h.events) == 0 {
		t.Fatal("hook was not invoked")
	}
	for i, ev := range h.events {
		if ev.AST == nil {
			t.Fatalf("event %d has nil AST", i)
		}
		if ev.Env == nil {
			t.Fatalf("event %d has nil Env", i)
		}
	}
}

func TestEvalHookErrorAborts(t *testing.T) {
	want := errors.New("aborted by hook")
	withHook(t, &recordingHook{err: want})

	_, err := mustEval(t, "(do 1 2 3)")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, want) && !strings.Contains(err.Error(), want.Error()) {
		t.Fatalf("expected error to wrap %q, got %v", want, err)
	}
}

func TestEvalHookNilNoOverhead(t *testing.T) {
	withHook(t, nil)

	res, err := mustEval(t, "(do 1 2 42)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, _ := res.(int); got != 42 {
		t.Fatalf("expected 42, got %v (%T)", res, res)
	}
}

func TestDebugEvalEnabledShim(t *testing.T) {
	prevHook := runtime.Hook
	prevFlag := DebugEvalEnabled
	runtime.Hook = nil
	DebugEvalEnabled = true
	t.Cleanup(func() {
		runtime.Hook = prevHook
		DebugEvalEnabled = prevFlag
	})

	if _, err := mustEval(t, "(do 1 2 3)"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := runtime.Hook.(runtime.PrintEvalHook); !ok {
		t.Fatalf("expected DebugEvalEnabled to install PrintEvalHook, got %T", runtime.Hook)
	}
}
