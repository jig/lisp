package docmeta_test

import (
	"context"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/docmeta"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/concurrent"
	"github.com/jig/lisp/lib/core"
	"github.com/jig/lisp/lib/coreextented"
	"github.com/jig/lisp/lib/coreextented/nscoreextended"
	"github.com/jig/lisp/types"
)

func fullEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	core.Load(ns)
	core.LoadInput(ns)
	concurrent.Load(ns)
	if err := nscoreextended.Load(ns); err != nil {
		t.Fatalf("nscoreextended.Load: %v", err)
	}
	ns.Set(types.Symbol{Val: "eval"}, types.Func{Fn: func(ctx context.Context, a []types.MalType) (types.MalType, error) {
		return lisp.EVAL(ctx, a[0], ns)
	}})
	ctx := context.Background()
	for _, h := range []string{core.HeaderBasic(), core.HeaderLoadFile(), concurrent.HeaderConcurrent(), coreextented.HeaderCoreExtended()} {
		if _, err := lisp.REPL(ctx, ns, h, types.NewCursorFile("preamble")); err != nil {
			t.Fatalf("header: %v", err)
		}
	}
	return ns
}

// TestBuiltinsAreHonest asserts every documented entry matches reality:
// Function entries resolve to a Go builtin (types.Func), SpecialForm
// entries are absent from the environment (handled by EVAL). This
// catches a renamed/removed builtin or a misclassified entry.
func TestBuiltinsAreHonest(t *testing.T) {
	ns := fullEnv(t)
	for name, e := range docmeta.Builtins {
		v, err := ns.Get(types.Symbol{Val: name})
		switch e.Kind {
		case docmeta.Function:
			if err != nil {
				t.Errorf("%q documented as function but not in env: %v", name, err)
				continue
			}
			if _, ok := v.(types.Func); !ok {
				t.Errorf("%q documented as Go function but env value is %T (lisp-defined? then remove it — it is covered from the env)", name, v)
			}
		case docmeta.SpecialForm:
			if err == nil {
				t.Errorf("%q documented as special form but resolves in the env as %T", name, v)
			}
		}
		if e.Params == "" && e.Kind == docmeta.Function {
			t.Errorf("%q has no arglist", name)
		}
		if e.Doc == "" {
			t.Errorf("%q has no doc", name)
		}
	}
}
