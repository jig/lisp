//go:build !debugger

package command

import (
	"context"
	"strings"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core"
)

func TestExecute_DebugFlagRequiresDebugBuild(t *testing.T) {
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

	err := Execute([]string{"lisp", "--debug", "-e", "(do 1 2)"}, ns)
	if err == nil {
		t.Fatal("expected --debug to fail in release build")
	}
	if !strings.Contains(err.Error(), "debugger") {
		t.Fatalf("expected error to mention debugger build tag, got %v", err)
	}
}
