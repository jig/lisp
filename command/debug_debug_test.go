//go:build lispdebug

package command

import (
	"context"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core"
	"github.com/jig/lisp/runtime"
)

func TestExecute_DebugFlagInstallsHook(t *testing.T) {
	prev := runtime.Hook
	t.Cleanup(func() { runtime.Hook = prev })
	runtime.Hook = nil

	ns := env.NewEnv()
	core.Load(ns)
	core.LoadInput(ns)
	ctx := context.Background()
	if _, err := lisp.REPL(ctx, ns, core.HeaderBasic(), nil); err != nil {
		t.Fatalf("preamble failed: %v", err)
	}
	if _, err := lisp.REPL(ctx, ns, core.HeaderLoadFile(), nil); err != nil {
		t.Fatalf("preamble failed: %v", err)
	}

	if err := Execute([]string{"lisp", "--debug", "-e", "(do 1 2)"}, ns); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := runtime.Hook.(runtime.PrintEvalHook); !ok {
		t.Fatalf("expected --debug to install PrintEvalHook, got %T", runtime.Hook)
	}
}
