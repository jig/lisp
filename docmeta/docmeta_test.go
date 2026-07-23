package docmeta_test

import (
	"context"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/docmeta"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/concurrent"
	"github.com/jig/lisp/lib/core"
	"github.com/jig/lisp/lib/coreextended"
	"github.com/jig/lisp/lib/system"
	"github.com/jig/lisp/types"
)

func fullEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	core.Load(ns)
	core.LoadInput(ns)
	concurrent.Load(ns)
	ns.Set(types.Symbol{Val: "eval"}, types.Func{Fn: func(ctx context.Context, a []types.MalType) (types.MalType, error) {
		return lisp.EVAL(ctx, a[0], ns)
	}})
	ctx := context.Background()
	for _, h := range []string{core.HeaderBasic(), system.HeaderLoadFile(), concurrent.HeaderConcurrent(), coreextended.HeaderCoreExtended()} {
		if _, err := lisp.REPL(ctx, ns, h, types.NewCursorFile("preamble")); err != nil {
			t.Fatalf("header: %v", err)
		}
	}
	return ns
}

// TestSpecialFormsAreNotBound asserts every documented special form is
// truly a special form: absent from a fully loaded environment (handled
// by EVAL, not bound as a value). A relocated builtin would resolve and
// fail here.
func TestSpecialFormsAreNotBound(t *testing.T) {
	ns := fullEnv(t)
	for name, e := range docmeta.SpecialForms {
		if _, err := ns.Get(types.Symbol{Val: name}); err == nil {
			t.Errorf("%q documented as a special form but resolves in the env", name)
		}
		if e.Params == "" {
			t.Errorf("%q has no arglist", name)
		}
		if e.Doc == "" {
			t.Errorf("%q has no doc", name)
		}
	}
}
