//go:build debugger

package lisp

import (
	"context"
	"sync"
	"testing"

	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core"
	"github.com/jig/lisp/runtime"
	"github.com/jig/lisp/types"
)

// recordingStackHook captures the maximum observed stack depth and a
// snapshot of frames each time OnEval fires.
type recordingStackHook struct {
	mu        sync.Mutex
	maxDepth  int
	snapshots [][]runtime.Frame
}

func (h *recordingStackHook) OnEval(ctx context.Context, _ runtime.EvalEvent) error {
	t := runtime.ThreadFromContext(ctx)
	if t == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if d := t.Depth(); d > h.maxDepth {
		h.maxDepth = d
	}
	h.snapshots = append(h.snapshots, t.Snapshot())
	return nil
}

func TestThreadGrowsAndShrinksWithEVAL(t *testing.T) {
	prev := runtime.Hook
	t.Cleanup(func() { runtime.Hook = prev })

	h := &recordingStackHook{}
	runtime.Hook = h

	thread := runtime.NewThread()
	ctx := runtime.WithThread(context.Background(), thread)

	ns := env.NewEnv()
	core.Load(ns)
	core.LoadInput(ns)
	if _, err := REPL(ctx, ns, core.HeaderBasic(), nil); err != nil {
		t.Fatalf("preamble: %v", err)
	}

	src := "(let [a 1 b 2] (+ a b))"
	ast, err := READ(src, types.NewCursorFile("stack-test"), ns)
	if err != nil {
		t.Fatalf("READ: %v", err)
	}
	if _, err := EVAL(ctx, ast, ns); err != nil {
		t.Fatalf("EVAL: %v", err)
	}

	if h.maxDepth < 2 {
		t.Fatalf("expected at least depth 2 (let + body), got %d", h.maxDepth)
	}
	if got := thread.Depth(); got != 0 {
		t.Fatalf("expected stack to be empty after EVAL, got depth %d", got)
	}
}

func TestFrameMutatesAcrossTCO(t *testing.T) {
	prev := runtime.Hook
	t.Cleanup(func() { runtime.Hook = prev })

	h := &recordingStackHook{}
	runtime.Hook = h

	thread := runtime.NewThread()
	ctx := runtime.WithThread(context.Background(), thread)

	ns := env.NewEnv()
	core.Load(ns)
	core.LoadInput(ns)
	if _, err := REPL(ctx, ns, core.HeaderBasic(), nil); err != nil {
		t.Fatalf("preamble: %v", err)
	}

	// `do` triggers TCO: the inner forms reuse the same EVAL frame.
	src := "(do 1 2 3 4 5)"
	ast, err := READ(src, types.NewCursorFile("tco-test"), ns)
	if err != nil {
		t.Fatalf("READ: %v", err)
	}
	if _, err := EVAL(ctx, ast, ns); err != nil {
		t.Fatalf("EVAL: %v", err)
	}

	// We expect at least one snapshot whose top frame's AST is not the
	// initial (do ...) form — TCO mutated it.
	var sawMutation bool
	for _, snap := range h.snapshots {
		if len(snap) == 0 {
			continue
		}
		top := snap[len(snap)-1]
		if _, ok := top.AST.(types.List); !ok {
			sawMutation = true
			break
		}
	}
	if !sawMutation {
		t.Fatal("expected TCO loop to mutate the top frame's AST to a non-list value")
	}
}
